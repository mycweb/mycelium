// package mvm1 contains an implementation of the Mycelium Virtual Machine (MVM)
package mvm1

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/hashicorp/golang-lru/v2/simplelru"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	mycelium "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

type Word = uint32

const (
	WordBits  = 32
	WordBytes = WordBits / 8
)

type VM struct {
	store cadata.Store

	prog  []I
	pc    uint32
	self  dynValue
	calls []Call
	stack []Word
	steps uint64

	err      error
	panicVal AnyValue
	ctx      context.Context

	ports     map[Port]PortBackend
	accels    map[Fingerprint]AccelFunc
	funcCache *simplelru.LRU[Fingerprint, []I]
}

func New(stackSize int, s cadata.Store, accels map[Fingerprint]AccelFunc) *VM {
	funcCache, err := simplelru.NewLRU[Fingerprint, []I](100, nil)
	if err != nil {
		panic(err)
	}
	return &VM{
		store: s,

		ports:     make(map[Port]PortBackend),
		accels:    accels,
		funcCache: funcCache,
	}
}

func (vm *VM) Reset() {
	vm.stack = vm.stack[:0]
	vm.calls = vm.calls[:0]
	vm.self = dynValue{}
	vm.err = nil
	vm.pc = 0
	vm.prog = nil
}

// Run executes the VM for a maximum of maxSteps.
// The number of steps taken is returned.
// If Run returns 0, then nothing happened and the machine has halted.
func (vm *VM) Run(ctx context.Context, maxSteps uint64) (steps uint64) {
	vm.ctx = ctx
	defer func() { vm.ctx = nil }()
	defer func() { vm.steps += steps }()

	for i := uint64(0); i < maxSteps; i++ {
		if !vm.isAlive() {
			return i
		}
		ix := vm.prog[vm.pc]
		vm.pc++
		// it is important to adjust the program counter before the instruction so
		// that the instruction can override it.
		vm.step(ix)
	}
	return maxSteps
}

func (vm *VM) step(ix I) {
	switch ix := ix.(type) {
	case pushI:
		vm.push(ix.x)
	case pushAnyTypeI:
		vm.pushAnyType(ix.x)
	case selfI:
		vm.pushSelf()
	case equalI:
		vm.equal(ix.words)
	case sizeOfI:
		vm.sizeOf()
	case muxI:
		vm.mux(ix.table, ix.elemSize)

	// control flow:
	case branchI:
		vm.branch(ix)
	case jumpI:
		vm.jump(ix)
	case evalI:
		vm.eval()
	case applyI:
		vm.apply(ix)
	case retI:
		vm.ret(ix)
	case panicI:
		vm.panic()
	case accelI:
		vm.applyAccel(ix)

	// compiler
	case lazyI:
		vm.lazy(ix.et, ix.layout)
	case lambdaI:
		vm.lambda(ix.et, ix.layout, ix.scratch)

	// stack
	case pickI:
		vm.pick(ix)
	case cutI:
		vm.cut(ix)
	case discardI:
		vm.discard(ix.words)
	case swapI:
		vm.swap(ix)

	// make / break
	case concatI:
		vm.concat(ix.leftBits, ix.rightBits)
	case sliceI:
		vm.slice(ix)
	case makeSumI:
		vm.makeSum(ix)

	// store
	case pushRefI:
		vm.pushRef(ix.x)
	case loadI:
		vm.load(ix.outputBits)
	case postI:
		vm.post(ix)

	// port
	case portInputI:
		vm.portInput(ix.produceWords)
	case portOutputI:
		vm.portOutput(ix.consumeWords)
	case portInteractI:
		vm.portInteract(ix.consumeWords, ix.produceWords)

	case listGetI:
		vm.listGet(ix)
	case anyTypeFromI:
		vm.anyTypeFrom(ix)
	case anyTypeToI:
		vm.anyTypeTo(ix)
	case anyValueFromI:
		vm.anyValueFrom(ix)
	case anyValueToI:
		vm.anyValueTo(ix)

	default:
		panic(ix)
	}
}

// GetFault returns the value the fault value passed to Panic
func (vm *VM) GetFault() mycelium.Value {
	if vm.panicVal.GetRef().CID().IsZero() {
		panic("VM has not faulted")
	}
	ctx := context.TODO()
	av, err := mycelium.LoadRoot(ctx, vm.store, vm.panicVal.AsBytes())
	if err != nil {
		panic(err)
	}
	return av.Unwrap()
}

func (vm *VM) Err() error {
	return vm.err
}

// SetEval tells the VM to evaluate the expression on the top stack when it is next run.
// SetEval erases the current call stack.
func (vm *VM) SetEval() {
	vm.calls = nil
	vm.self = dynValue{}
	vm.prog = []I{evalI{}}
}

func (vm *VM) SetProg(prog []I) {
	vm.prog = prog
}

func (vm *VM) ImportLazy(ctx context.Context, src cadata.Getter, laz *mycelium.Lazy) error {
	if err := laz.PullInto(ctx, vm.store, src); err != nil {
		return err
	}
	bb := bitbuf.New(LazyBits)
	laz.Encode(bb)
	var laz2 Lazy
	bytesToWords(bb.Bytes(), laz2[:])
	vm.PushLazy(laz2)
	return nil
}

// PushLazy pushes a reference to an expression
// the same representation is used for lambdas and lazy
func (vm *VM) PushLazy(laz Lazy) {
	vm.pushRef(Ref(laz[:8]))
	vm.push(laz[8])
}

func (vm *VM) PopLazy() (ret Lazy) {
	vm.popInto(ret[:])
	return ret
}

func (vm *VM) ImportAnyValue(ctx context.Context, src cadata.Getter, x AnyValue) error {
	if err := pullAnyValue(ctx, vm.store, src, x); err != nil {
		return err
	}
	vm.pushAnyValue(x)
	return nil
}

func (vm *VM) ExportAnyValue(ctx context.Context, dst cadata.PostExister) (AnyValue, error) {
	if len(vm.stack) < len(AnyValue{}) {
		return AnyValue{}, fmt.Errorf("stack too small to contain AnyValue. %d", len(vm.stack))
	}
	av := vm.popAnyValue()
	if err := pullAnyValue(ctx, dst, vm.store, av); err != nil {
		return AnyValue{}, err
	}
	return av, nil
}

func (vm *VM) DumpStack(out []Word) []Word {
	return append(out, vm.stack...)
}

func (vm *VM) isAlive() bool {
	return int(vm.pc) < len(vm.prog) && vm.err == nil
}

func (vm *VM) push(x Word) {
	vm.stack = append(vm.stack, x)
}

func (vm *VM) pop() Word {
	i := len(vm.stack) - 1
	ret := vm.stack[i]
	vm.stack = vm.stack[:i]
	return ret
}

func (vm *VM) popInto(ws []Word) {
	start := len(vm.stack) - len(ws)
	for i := range ws {
		ws[i] = vm.stack[start+i]
	}
	vm.stack = vm.stack[:start]
}

func (vm *VM) pushBytes(data []byte) {
	for i := 0; i < len(data); i += WordBytes {
		vm.push(binary.LittleEndian.Uint32(data[i : i+WordBytes]))
	}
}

func (vm *VM) pushRef(ref Ref) {
	for _, w := range ref {
		vm.push(w)
	}
}

func (vm *VM) popRef() (ret Ref) {
	vm.popInto(ret[:])
	if ret == (Ref{}) {
		panic(ret)
	}
	return ret
}

func (vm *VM) pushType2(x Type2) {
	vm.push(x[0])
}

func (vm *VM) popType2() Type2 {
	return Type2{vm.pop()}
}

func (vm *VM) pushAnyType(at AnyType) {
	vm.pushRef(at.GetRef())
	vm.pushType2(at.GetType())
}

func (vm *VM) popAnyType() (ret AnyType) {
	at2 := vm.popType2()
	ref := vm.popRef()
	ret.SetRef(ref)
	ret.SetType(at2)
	return ret
}

func (vm *VM) pushAnyValue(av AnyValue) {
	vm.pushRef(av.GetRef())
	vm.pushAnyType(av.GetType())
}

func (vm *VM) popAnyValue() (ret AnyValue) {
	at := vm.popAnyType()
	ref := vm.popRef()

	ret.SetRef(ref)
	ret.SetType(at)
	return ret
}

func (vm *VM) popExpr() (ret Expr) {
	l := vm.pop()
	ref := vm.popRef()

	ret.SetRef(ref)
	ret.SetProgType(newProgType(int(l)))
	return ret
}

func (vm *VM) pushLambda(lam Lambda) {
	vm.pushRef(lam.GetRef())
	vm.push(lam.GetProgType()[0])
}

func (vm *VM) popLambda() (ret Lambda) {
	et := vm.pop()
	ref := vm.popRef()
	copy(ret[:8], ref[:])
	ret[8] = et
	return ret
}

func (vm *VM) popList() (ret List) {
	size := vm.pop()
	ref := vm.popRef()
	ret.SetLen(size)
	ret.SetRef(ref)
	return ret
}

func (vm *VM) mux(table []Word, elemSize int) {
	idx := vm.pop()

	// TODO: don't copy the whole table to the stack, just pick from the table and copy that to the stack.
	for i := range table {
		vm.push(table[i])
	}
	vm.push(idx)

	arrayLen := len(table) * WordBits / elemSize
	vm.arrayGet(arrayGetI{elemSize: elemSize, len: arrayLen})
}

func (vm *VM) anyTypeFrom(ix anyTypeFromI) {
	var salt *Ref
	if ix.t2.NeedsSalt() {
		salt = new(Ref)
		*salt = ix.t2.Salt()
	}
	vm.post(postI{salt: salt, inputBits: ix.inputBits})
	ref := vm.popRef()
	var at AnyType
	at.SetRef(ref)
	at.SetType(ix.t2)
	vm.pushAnyType(at)
}

// anyTypeTo pops an AnyType and pushes the types data to the stack
func (vm *VM) anyTypeTo(ix anyTypeToI) {
	at := vm.popAnyType()
	if at.GetType() != ix.to {
		vm.fail(fmt.Errorf("anyTypeTo: wrong type. HAVE: %v WANT: %v", ix.to, at.GetType()))
		return
	}
	vm.pushRef(at.GetRef())
	vm.load(ix.outputBits)
}

func (vm *VM) anyValueFrom(ix anyValueFromI) {
	var salt *Ref
	if ix.needsSalt {
		salt = new(Ref)
		*salt = ix.tyRef
	}
	vm.post(postI{salt: salt, inputBits: ix.inputBits})
	valRef := vm.popRef()

	var at AnyType
	at.SetRef(ix.tyRef)
	at.SetType(ix.t2)
	var av AnyValue
	av.SetRef(valRef)
	av.SetType(at)
	vm.pushAnyValue(av)
}

// anyValueTo pops an AnyValue off of the stack
// checks that arg0.GetType == arg[1], and then writes the native bitpattern to the stack
func (vm *VM) anyValueTo(ix anyValueToI) {
	av := vm.popAnyValue()
	if av.GetType() != ix.to {
		vm.fail(fmt.Errorf("AnyValueTo wrong type"))
		return
	}
	tag := ix.to.GetRef().CID()
	cid := av.GetRef().CID()
	buf := make([]byte, ix.outputBits)
	n, err := vm.store.Get(vm.ctx, &cid, &tag, buf)
	if err != nil {
		vm.fail(err)
		return
	}
	vm.pushBytes(buf[:n])
}

// sizeOf (AnyType) => Size
func (vm *VM) sizeOf() {
	at := vm.popAnyType()
	size, err := SizeOf(vm.ctx, vm.store, at)
	if err != nil {
		vm.fail(err)
		return
	}
	vm.push(Word(size))
}

func (vm *VM) fail(err error) {
	vm.prog = nil
	vm.err = err
}

func (vm *VM) discard(words int) {
	for i := 0; i < words; i++ {
		vm.pop()
	}
}

// loadLazy loads the code for a Lazy into the cache
func (vm *VM) loadLazy(x Lazy) ([]I, error) {
	body, err := loadAnyProg(vm.ctx, vm.store, x.GetRef(), x.GetProgType())
	if err != nil {
		return nil, err
	}
	c := vm.getCompiler()
	// we give no information about the stack because the expression should not reference it
	prog, err := c.compile(vm.ctx, machCtx{}, body.Prog())
	if err != nil {
		return nil, err
	}
	return prog.I, nil
}

// loadLambda returns the lambda body from the store, compiling and validating if necessary
func (vm *VM) loadLambda(lt LambdaType, lam Lambda) ([]I, error) {
	fp := lam.Fingerprint(&lt)
	if prog, exists := vm.funcCache.Get(fp); exists {
		return prog, nil
	}
	e, err := loadAnyProg(vm.ctx, vm.store, lam.GetRef(), lam.GetProgType())
	if err != nil {
		return nil, err
	}
	c := vm.getCompiler()
	// mc is empty. The compiler should not have accesss to the machine here
	mc := machCtx{}
	lam2, prog, err := c.makeLambda(vm.ctx, mc, lt.GetIn(), lt.GetOut(), e.Prog())
	if err != nil {
		return nil, err
	}
	if fp2 := lam2.Fingerprint(&lt); fp2 != fp {
		panic(fmt.Sprintf("fingerprint mismatch %v vs %v", fp, fp2))
	}
	vm.funcCache.Add(fp, prog.I)
	return prog.I, nil
}

// slice consumes inputBits from the top of the stack and produces end - beg bits
func (vm *VM) slice(ix sliceI) {
	inputWords := AlignSize(ix.inputBits) / WordBits
	outSize := ix.end - ix.beg
	outWords := AlignSize(int(outSize)) / WordBits

	ws := vm.stack[len(vm.stack)-inputWords:]
	cutBits(ws, 0, int(ix.beg))
	zeroBits(ws, ix.end, len(ws)*WordBits)
	vm.stack = vm.stack[:len(vm.stack)-inputWords+outWords]
}

// Concat merges 2 values into 1 value.
// Alignment applies to the outer value. it is not equivalent to concatenation of the Words in all cases.
func (vm *VM) concat(leftBits, rightBits int) {
	leftWords := AlignSize(leftBits) / WordBits
	rightWords := AlignSize(rightBits) / WordBits

	out := make([]Word, leftWords+rightWords)
	vm.popInto(out)
	cutBits(out, leftBits, AlignSize(leftBits))
	outWords := AlignSize(leftBits+rightBits) / WordBits
	for _, w := range out[:outWords] {
		vm.push(w)
	}
}

func (vm *VM) makeSum(ix makeSumI) {
	// the content will be on the top of the stack, pad it with zeros
	inputWords := AlignSize(ix.inputSize) / WordBits
	maxContentWords := AlignSize(ix.tagOffset) / WordBits
	for i := inputWords; i < maxContentWords; i++ {
		vm.push(0)
	}
	if ix.tagSize > 0 {
		// push the tag as a second argument
		vm.push(ix.tag)
		// concat the content, with the shortened tag
		vm.concat(ix.tagOffset, ix.tagSize)
	}
}

// pick copies an item wdepth into the stack
// wdepth = 0, wlength = 1 would refer to an item on the top of the stack
func (vm *VM) pick(ix pickI) {
	beg := len(vm.stack) - ix.wdepth
	end := beg + ix.wlength
	if beg < 0 || end > len(vm.stack) {
		panic(fmt.Sprintf("pick beg=%d end=%d stack=%d", beg, end, len(vm.stack)))
	}
	for i := beg; i < end; i++ {
		vm.push(vm.stack[i])
	}
}

func (vm *VM) cut(ix cutI) {
	if ix.wlength == 0 {
		return
	}
	beg := len(vm.stack) - ix.wdepth - 1
	end := beg + ix.wlength
	if beg < 0 || end > len(vm.stack) {
		panic(fmt.Sprintf("cut beg=%d end=%d stack=%d", beg, end, len(vm.stack)))
	}
	vm.stack = slices.Delete(vm.stack, beg, end)
}

func (vm *VM) swap(ix swapI) {
	lo := make([]Word, ix.loWords)
	hi := make([]Word, ix.hiWords)
	vm.popInto(hi)
	vm.popInto(lo)
	for _, w := range hi {
		vm.push(w)
	}
	for _, w := range lo {
		vm.push(w)
	}
}

func (vm *VM) equal(wsize int) {
	l := len(vm.stack)
	a := vm.stack[l-2*wsize : l-wsize]
	b := vm.stack[l-wsize:]
	eq := slices.Equal(a, b)
	for i := 0; i < 2*wsize; i++ {
		vm.pop()
	}
	if eq {
		vm.push(1)
	} else {
		vm.push(0)
	}
}

func (vm *VM) load(outputBits int) {
	ref := vm.popRef()
	pos := len(vm.stack)
	for i := 0; i < AlignSize(outputBits)/WordBits; i++ {
		vm.stack = append(vm.stack, 0)
	}
	if err := loadWords(vm.ctx, vm.store, ref, vm.stack[pos:]); err != nil {
		vm.fail(err)
		return
	}
}

func (vm *VM) post(ix postI) {
	words := AlignSize(ix.inputBits) / WordBits
	data := vm.stack[len(vm.stack)-words:]
	ref, err := postWords(vm.ctx, vm.store, ix.salt, ix.inputBits, data)
	if err != nil {
		vm.fail(err)
		return
	}
	vm.discard(words)
	vm.pushRef(ref)
}

// branch pops a Bit from the stack, if it is 1 then it skips forward in the program.
func (vm *VM) branch(ix branchI) {
	cond := vm.pop()
	if cond&1 == 1 {
		vm.pc += ix.offset
	}
}

func (vm *VM) jump(ix jumpI) {
	vm.pc += ix.offset
}

func (vm *VM) call(target dynValue, fn []I) {
	// if we are currently executing a function
	if !vm.self.IsZero() {
		// save the current position in the current procedure
		call := Call{
			From: vm.self.Clone(),
			Pos:  vm.pc - 1,
		}
		vm.calls = append(vm.calls, call)
	}
	vm.self.Set(target)
	vm.prog = fn
	vm.pc = 0
}

// ret returns from a call
func (vm *VM) ret(ix retI) {
	call := vm.calls[len(vm.calls)-1]
	var prog []I
	if call.isLambda() {
		fn, err := vm.loadLambda(call.lambdaType(), call.lambda())
		if err != nil {
			vm.fail(err)
			return
		}
		prog = fn
	} else {
		fn, err := vm.loadLazy(call.lazy())
		if err != nil {
			vm.fail(err)
			return
		}
		prog = fn
	}
	vm.prog = prog
	vm.self.Set(call.From)
	vm.pc = call.Pos + 1
	vm.calls = vm.calls[:len(vm.calls)-1]

	vm.cut(cutI{
		wdepth:  ix.outputWords + ix.inputWords - 1,
		wlength: ix.inputWords,
	})
}

func (vm *VM) eval() {
	laz := vm.PopLazy()
	fn, err := vm.loadLazy(laz)
	if err != nil {
		vm.fail(err)
		return
	}
	target := dynValue{}
	target.SetType2(newType2(spec.TC_Lazy, 0))
	target.SetType(nil) // TODO
	target.SetValue(laz[:], len(laz)*WordBits)
	vm.call(target, fn)
}

func (vm *VM) apply(ix applyI) {
	lam := vm.popLambda()
	prog, err := vm.loadLambda(ix.lambdaType, lam)
	if err != nil {
		vm.fail(err)
		return
	}
	target := dynValue{}
	target.SetType2(newType2(spec.TC_Lambda, 0))
	target.SetType(ix.lambdaType[:])
	target.SetValue(lam[:], len(lam)*WordBits)
	vm.call(target, prog)
}

func (vm *VM) applyAccel(ix accelI) {
	for i := ix.inputWords; i < ix.outputWords; i++ {
		vm.push(0)
	}
	stack := vm.stack[len(vm.stack)-max(ix.inputWords, ix.outputWords):]
	if err := ix.fn(stack); err != nil {
		vm.fail(fmt.Errorf("during accelerator %w", err))
		return
	}
	vm.stack = vm.stack[:len(vm.stack)-ix.inputWords+ix.outputWords]
}

// arrayGet (Array[T], Size) => T
func (vm *VM) arrayGet(ix arrayGetI) {
	vm.checkBounds(0, ix.len)
	idx := int(vm.pop())
	beg := ix.elemSize * idx
	end := ix.elemSize + beg
	vm.slice(sliceI{
		inputBits: ix.len * ix.elemSize,
		beg:       beg,
		end:       end,
	})
}

func (vm *VM) listGet(ix listGetI) {
	idx := vm.pop()
	list := vm.popList()
	vm.pushRef(list.GetRef())
	arraySize := int(list.GetLen()) * ix.elemSize
	vm.load(arraySize)
	vm.push(idx)
	vm.arrayGet(arrayGetI{len: int(list.GetLen()), elemSize: ix.elemSize})
}

func (vm *VM) checkBounds(gteq, lt int) {
	x := vm.pop()
	if x < uint32(gteq) || x >= uint32(lt) {
		vm.fail(fmt.Errorf("out of bounds. min=%v max=%v idx=%v", gteq, lt, x))
		return
	}
	vm.push(x)
}

// lazy (Ref) => Lazy
func (vm *VM) lazy(et ProgType, layout []Type) {
	// pop
	expr := vm.popExpr()
	ref := expr.GetRef()
	e, err := loadAnyProg(vm.ctx, vm.store, ref, et)
	if err != nil {
		vm.fail(err)
		return
	}
	// compile
	c := vm.getCompiler()
	mc := machCtx{
		stack:  vm.stack,
		store:  vm.store,
		layout: slices.Clone(layout),
	}
	laz, _, err := c.makeLazy(vm.ctx, mc, e.Prog())
	if err != nil {
		vm.fail(err)
		return
	}
	// push
	vm.PushLazy(*laz)
}

// lambda (AnyType, AnyType, Ref) => Lambda
func (vm *VM) lambda(et ProgType, layout []Type, scratch []int) {
	expr := vm.popExpr()
	ref := expr.GetRef()
	out := vm.popAnyType()
	in := vm.popAnyType()

	e, err := loadAnyProg(vm.ctx, vm.store, ref, et)
	if err != nil {
		vm.fail(err)
		return
	}

	c := vm.getCompiler()
	mc := machCtx{
		stack:   vm.stack,
		store:   vm.store,
		layout:  slices.Clone(layout),
		scratch: slices.Clone(scratch),
	}
	lam, _, err := c.makeLambda(vm.ctx, mc, in, out, e.Prog())
	if err != nil {
		vm.fail(err)
		return
	}
	vm.pushLambda(*lam)
}

func (vm *VM) pushSelf() {
	if vm.self.IsZero() {
		vm.fail(fmt.Errorf("self is not set in this context"))
		return
	}
	vm.pushLambda(vm.self.Lambda())
}

// fault pops an AnyValue from the stack and faults the machine with that value as the reason
func (vm *VM) panic() {
	av := vm.popAnyValue()
	vm.err = fmt.Errorf("fault: %v", av)
	vm.panicVal = av
}

func (vm *VM) getCompiler() Compiler {
	return newCompiler(vm.store, vm.accels, func(ctx context.Context, prog []I) ([]Word, error) {
		vm2 := New(0, vm.store, vm.accels)
		vm2.Reset()
		vm2.SetProg(prog)
		vm2.Run(ctx, 1000)
		if err := vm2.Err(); err != nil {
			return nil, err
		}
		return vm2.stack, nil
	})
}

// cutBits removes from beg to end within ws.
// cutBits(_, 0, 1)
func cutBits(ws []uint32, beg, end int) {
	totalBits := len(ws) * 32
	if beg < 0 || end > totalBits {
		panic(fmt.Sprintf("beg=%d end=%d", beg, end))
	}
	bitCount := end - beg
	if bitCount == 0 {
		return
	}
	for i := beg; i < totalBits-bitCount; i++ {
		srcWord := (i + bitCount) / 32
		srcBit := (i + bitCount) % 32
		dstWord := i / 32
		dstBit := i % 32

		// Clear the destination bit
		ws[dstWord] &= ^(uint32(1) << dstBit)
		// Set the destination bit to the source bit
		ws[dstWord] |= ((ws[srcWord] >> srcBit) & 1) << dstBit
	}
	zeroBits(ws, totalBits-bitCount, totalBits)
}

func zeroBits(ws []Word, beg, end int) {
	// Zero out the remaining bits
	for i := beg; i < end; i++ {
		word := i / 32
		bit := i % 32
		ws[word] &= ^(uint32(1) << bit)
	}
}

func divCeil(x, d int) int {
	q := x / d
	if x%d != 0 {
		q++
	}
	return q
}

func loadAnyProg(ctx context.Context, s cadata.Getter, ref Ref, et ProgType) (*mycelium.AnyProg, error) {
	buf := make([]byte, spec.ExprBits/8)
	wordsToBytes(ref[:], buf[:32])
	wordsToBytes(et[:], buf[32:])

	bb := bitbuf.FromBytes(buf)
	var expr mycelium.AnyProg
	if err := expr.Decode(bb, func(ref mycelium.Ref) (mycelium.Value, error) {
		return mycelium.Load(ctx, s, ref)
	}); err != nil {
		return nil, err
	}
	return &expr, nil
}

// Call is a call site
// It is the position in a Lazy or Lambda body that the function was called from
// And the size of the current Lazy or Lambda
type Call struct {
	From dynValue
	Pos  uint32
}

func (c *Call) isLambda() bool {
	return c.From.t2.TypeCode() == spec.TC_Lambda
}

func (c *Call) lambda() (ret Lambda) {
	return c.From.Lambda()
}

func (c *Call) lambdaType() LambdaType {
	return LambdaType(c.From.typeData)
}

func (c *Call) lazy() (ret Lazy) {
	return c.From.Lazy()
}

// AlignSize takes a size in bits and returns the size padded to a multiple of WordBits
// The returned size is also in bits.
func AlignSize(sizeBits int) int {
	if sizeBits%WordBits > 0 {
		sizeBits += WordBits - sizeBits%WordBits
	}
	return sizeBits
}

func pullAnyValue(ctx context.Context, dst cadata.PostExister, src cadata.Getter, x AnyValue) error {
	val, err := mycelium.LoadRoot(ctx, src, x.AsBytes())
	if err != nil {
		return err
	}
	val = mycelium.NewAnyValue(val)
	return val.PullInto(ctx, dst, src)
}

func loadWords(ctx context.Context, s cadata.Getter, ref Ref, ws []Word) error {
	cid := ref.CID()
	buf := make([]byte, len(ws)*WordBits/8)
	_, err := s.Get(ctx, &cid, nil, buf)
	if err != nil {
		return err
	}
	for i := range ws {
		ws[i] = 0
	}
	bytesToWords(buf, ws)
	return nil
}

func postWords(ctx context.Context, s cadata.Poster, salt *Ref, size int, ws []Word) (Ref, error) {
	buf := make([]byte, len(ws)*WordBits/8)
	wordsToBytes(ws, buf)

	var saltCID *cadata.ID
	if salt != nil {
		saltCID = new(cadata.ID)
		*saltCID = salt.CID()
	}
	cid, err := s.Post(ctx, saltCID, buf[:divCeil(size, 8)])
	if err != nil {
		return Ref{}, err
	}
	return RefFromCID(cid), nil
}

package mycmem

import (
	"context"
	"encoding/binary"
	"fmt"
	"iter"
	"slices"
	"strings"

	"go.brendoncarroll.net/exp/slices2"
	"golang.org/x/exp/constraints"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type ProgType struct {
	sizeBytes uint32
}

func NewProgType(sizeBytes int) *ProgType {
	return &ProgType{uint32(sizeBytes)}
}

func (pt *ProgType) isValue() {}
func (pt *ProgType) isType()  {}

func (pt *ProgType) Type() Type {
	return ProgKind()
}

// SizeOf returns the size of a value of this type in bits
func (pt *ProgType) SizeOf() int {
	return int(pt.sizeBytes) * 8
}

func (pt *ProgType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return nil
}

func (pt *ProgType) Encode(bb BitBuf) {
	bb.Put32(0, pt.sizeBytes)
}

func (pt *ProgType) Decode(bb BitBuf, _ LoadFunc) error {
	pt.sizeBytes = bb.Get32(0)
	return nil
}

func (pt *ProgType) Zero() Value {
	return &Prog{
		{code: spec.ProductEmpty},
	}
}

func (pt *ProgType) Components() iter.Seq[Value] {
	return emptyIter
}

func (pt *ProgType) String() string {
	return fmt.Sprintf("ProgType[bytes=%d]", pt.sizeBytes)
}

// Prog is compiled VM byte-code
type Prog []Node

func (*Prog) isValue() {}

func (p *Prog) Root() Node {
	return (*p)[len(*p)-1]
}

func (p Prog) InputOffset(i int) uint32 {
	node := p.Root()
	if !node.IsOp() {
		panic(p)
	}
	off := node.args[i]
	if off == 0 {
		panic(i)
	}
	return off
}

func (p Prog) Input(i int) Prog {
	off := p.InputOffset(i)
	return p[:len(p)-int(off)]
}

func (p Prog) NumInputs() int {
	return p.Root().code.InDegree()
}

func (p Prog) Inputs() iter.Seq[Prog] {
	return func(yield func(Prog) bool) {
		for i := 0; i < p.NumInputs(); i++ {
			if !yield(p.Input(i)) {
				return
			}
		}
	}
}

// Code is equivalent for Root().Code()
func (p Prog) Code() spec.Op {
	return p.Root().code
}

// IsLiteral
func (p Prog) IsLiteral() bool {
	return p.Root().IsLiteral()
}

func (p Prog) Literal() Value {
	return p.Root().Literal()
}

func (p Prog) IsParam() bool {
	return p.Root().IsParam()
}

func (p Prog) Param() uint32 {
	return p.Root().Param()
}

func (p Prog) IsSelf() bool {
	return p.Root().IsSelf()
}

func (p *Prog) ProgType() *ProgType {
	return NewProgType(p.Size() / 8)
}

func (p *Prog) Type() Type {
	return p.ProgType()
}

// Size returns the size of the program in bits
func (p *Prog) Size() int {
	var sum int
	for _, node := range *p {
		sum += node.Size()
	}
	return sum
}

func (p *Prog) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	for _, node := range *p {
		switch node.code {
		case spec.LiteralAnyType:
			if err := node.litAnyType.PullInto(ctx, dst, src); err != nil {
				return err
			}
		case spec.LiteralAnyValue:
			if err := node.litAnyValue.PullInto(ctx, dst, src); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Prog) Encode(bb BitBuf) {
	if bb.Len() < p.Size() {
		panic(fmt.Sprintf("buffer too short %d < %d", bb.Len(), p.Size()))
	}
	out, err := encodeProg(bb.Bytes()[:0], *p)
	if err != nil {
		panic(err)
	}
	if len(out) > len(bb.Bytes()) {
		panic(fmt.Sprintf("len(buf)=%dB, len(prog)=%dB prog=%v", len(bb.Bytes()), len(out), *p))
	}
}

func (dst *Prog) Decode(bb BitBuf, load LoadFunc) error {
	prog, err := decodeProg(bb.Bytes(), load)
	if err != nil {
		return err
	}
	*dst = prog
	return nil
}

func (p *Prog) String() string {
	return fmt.Sprintf("Prog{len=%d}", len(*p))
}

func (p *Prog) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for _, node := range *p {
			node := node
			switch node.code {
			case spec.LiteralAnyType:
				if !yield(node.litAnyType) {
					return
				}
			case spec.LiteralAnyValue:
				if !yield(node.litAnyValue) {
					return
				}
			}
		}
	}
}

func encodeProg(out []byte, prog Prog) ([]byte, error) {
	for i, node := range prog {
		beginlen := len(out)
		out = append(out, byte(node.code))
		switch {
		case node.IsCode(spec.Self):
		case node.IsParam():
			if node.IsCode(spec.ParamN) {
				out = binary.AppendUvarint(out, uint64(node.param))
			}
		case node.code.IsSmallBitArray():
			// no additional bytes
		case node.IsCode(spec.LiteralB8):
			if len(node.litBits) < 1 {
				panic(fmt.Sprintf("encodeProg: LiteralB8 node has litBits length %d, expected at least 1", len(node.litBits)))
			}
			out = append(out, node.litBits[:1]...)
		case node.IsCode(spec.LiteralB16):
			if len(node.litBits) < 2 {
				panic(fmt.Sprintf("encodeProg: LiteralB16 node has litBits length %d, expected at least 2", len(node.litBits)))
			}
			out = append(out, node.litBits[:2]...)
		case node.IsCode(spec.LiteralB32):
			if len(node.litBits) < 4 {
				panic(fmt.Sprintf("encodeProg: LiteralB32 node has litBits length %d, expected at least 4. Node: %+v", len(node.litBits), node))
			}
			out = append(out, node.litBits[:4]...)
		case node.IsCode(spec.LiteralB64):
			if len(node.litBits) < 8 {
				panic(fmt.Sprintf("encodeProg: LiteralB64 node has litBits length %d, expected at least 8", len(node.litBits)))
			}
			out = append(out, node.litBits[:8]...)
		case node.IsCode(spec.LiteralB128):
			if len(node.litBits) < 16 {
				panic(fmt.Sprintf("encodeProg: LiteralB128 node has litBits length %d, expected at least 16", len(node.litBits)))
			}
			out = append(out, node.litBits[:16]...)
		case node.IsCode(spec.LiteralB256):
			if len(node.litBits) < 32 {
				panic(fmt.Sprintf("encodeProg: LiteralB256 node has litBits length %d, expected at least 32", len(node.litBits)))
			}
			out = append(out, node.litBits[:32]...)
		case node.IsCode(spec.LiteralKind):
			out = MarshalAppend(out, node.litKind)
		case node.IsCode(spec.LiteralAnyType):
			out = MarshalAppend(out, node.litAnyType)
		case node.IsCode(spec.LiteralAnyValue):
			out = MarshalAppend(out, node.litAnyValue)
		default:
			deps := node.args[:node.code.InDegree()]
			for _, arg := range deps {
				if arg > uint32(i) {
					return nil, fmt.Errorf("node %d referenced %d", i, arg)
				}
				out = binary.AppendUvarint(out, uint64(arg))
			}
		}
		if delta := len(out) - beginlen; delta != divCeil(node.Size(), 8) {
			panic(fmt.Sprintf("node %v has size=%d but added %d bytes", node, node.Size(), delta))
		}
	}
	return out, nil
}

func decodeProg(src []byte, load LoadFunc) (Prog, error) {
	var nodes []Node
	for len(src) > 0 {
		var dst Node
		if n, err := decodeNode(&dst, uint32(len(nodes)), src, load); err != nil {
			return nil, err
		} else {
			src = src[n:]
		}
		nodes = append(nodes, dst)
	}
	return nodes, nil
}

func decodeNode(dst *Node, idx uint32, src []byte, load LoadFunc) (int, error) {
	opc := spec.Op(src[0])
	n := 1
	src = src[1:]
	dst.code = opc

	if opc.IsParam() {
		if opc == spec.ParamN {
			x, n2 := binary.Uvarint(src)
			if n2 <= 0 {
				return 0, fmt.Errorf("could not read varint")
			}
			n += n2
			dst.param = uint32(x)
		} else {
			dst.param = uint32(opc - spec.Param0)
		}
		return n, nil
	}

	if opc.IsSmallBitArray() {
		dst.litBits = dst.litBits[:0]
		l, d := opc.SmallBitArray()
		if l > 0 {
			dst.litBits = append(dst.litBits[:0], d)
		}
		return n, nil
	}
	switch opc {
	case spec.Self:
	case spec.LiteralB8:
		dst.litBits = append(dst.litBits[:0], src[:1]...)
		n += 1
	case spec.LiteralB16:
		dst.litBits = append(dst.litBits[:0], src[:2]...)
		n += 2
	case spec.LiteralB32:
		dst.litBits = append(dst.litBits[:0], src[:4]...)
		n += 4
	case spec.LiteralB64:
		dst.litBits = append(dst.litBits[:0], src[:8]...)
		n += 8
	case spec.LiteralB128:
		dst.litBits = append(dst.litBits[:0], src[:16]...)
		n += 16
	case spec.LiteralB256:
		dst.litBits = append(dst.litBits[:0], src[:32]...)
		n += 32
	case spec.LiteralKind:
		var k Kind
		if err := k.Decode(bitbuf.FromBytes(src), nil); err != nil {
			return 0, err
		}
		dst.litKind = &k
		n += spec.KindBits / 8
	case spec.LiteralAnyType:
		var at AnyType
		if err := at.Decode(bitbuf.FromBytes(src), load); err != nil {
			return 0, err
		}
		dst.litAnyType = &at
		n += spec.AnyTypeBits / 8
	case spec.LiteralAnyValue:
		var av AnyValue
		if err := av.Decode(bitbuf.FromBytes(src), load); err != nil {
			return 0, err
		}
		dst.litAnyValue = &av
		n += spec.AnyValueBits / 8
	default:
		for i := 0; i < opc.InDegree(); i++ {
			offset, n2 := binary.Uvarint(src)
			if n2 <= 0 {
				return 0, fmt.Errorf("could not read varint")
			}
			n += n2
			src = src[n2:]
			if uint32(offset) > idx {
				return 0, fmt.Errorf("node %d references %d", idx, offset)
			}
			dst.args[i] = uint32(offset)
		}
	}
	return n, nil
}

func equalProgs(a, b Prog) bool {
	return slices.EqualFunc(a, b, func(a, b Node) bool {
		return a.code == b.code && a.args == b.args && a.param == b.param &&
			Equal(a.litAnyType, b.litAnyType) &&
			Equal(a.litAnyValue, b.litAnyValue)
	})
}

type Node struct {
	code spec.Op

	// param holds the index of the parameter
	param uint32
	// args holds the relative offset of the arguments
	args [3]uint32

	// below here are literals

	litBits     []byte
	litKind     *Kind
	litAnyType  *AnyType
	litAnyValue *AnyValue
}

func Literal(v Value) Node {
	switch x := v.(type) {
	case *Bit:
		if x.AsBool() {
			return Node{code: spec.ONE}
		} else {
			return Node{code: spec.ZERO}
		}
	case AsBitArray:
		return literalBits(x)
	case *Kind:
		return literalKind(x)
	case Type:
		if at, ok := x.(*AnyType); ok {
			return literalAnyValue(NewAnyValue(at))
		}
		return literalAnyType(NewAnyType(x))
	}
	return literalAnyValue(NewAnyValue(v))
}

func literalKind(k *Kind) Node {
	return Node{code: spec.LiteralKind, litKind: k}
}

func literalBits(x AsBitArray) Node {
	l := x.AsBitArray().Len()
	switch l {
	case 0:
		return Node{code: spec.LiteralB0}
	case 1:
		return Node{code: spec.LiteralB1(int(x.AsBitArray().AsUint32()))}
	case 2:
		return Node{code: spec.LiteralB2(int(x.AsBitArray().AsUint32()))}
	case 3:
		return Node{code: spec.LiteralB3(int(x.AsBitArray().AsUint32()))}
	case 4:
		return Node{code: spec.LiteralB4(int(x.AsBitArray().AsUint32()))}
	case 8, 16, 32, 64, 128, 256:
		bb := bitbuf.New(l)
		x.Encode(bb)
		return Node{
			code:    spec.LiteralBx8(l / 8),
			litBits: bb.Bytes(),
		}
	default:
		return literalAnyValue(NewAnyValue(x))
	}
}

func literalAnyType(x *AnyType) Node {
	return Node{code: spec.LiteralAnyType, litAnyType: x}
}

func literalAnyValue(x *AnyValue) Node {
	return Node{code: spec.LiteralAnyValue, litAnyValue: x}
}

func Self() Node {
	return Node{code: spec.Self}
}

func Param(x uint32) Node {
	const maxParam = spec.ParamN - spec.Param0
	if x < uint32(maxParam) {
		return Node{
			code:  spec.Op(x) + spec.Param0,
			param: x,
		}
	} else {
		return Node{
			code:  spec.ParamN,
			param: x,
		}
	}
}

func OpNode(code spec.Op, offsets ...uint32) Node {
	if len(offsets) != code.InDegree() {
		panic(code)
	}
	node := Node{code: code}
	copy(node.args[:], offsets)
	return node
}

func (n Node) Code() spec.Op {
	return n.code
}

func (n Node) IsSelf() bool {
	return n.IsCode(spec.Self)
}

func (n Node) IsLiteral() bool {
	return n.IsLiteralBits() || n.IsCode(spec.LiteralKind, spec.LiteralAnyType, spec.LiteralAnyValue)
}

func (n Node) IsLiteralBits() bool {
	return n.code >= spec.LiteralB8
}

func (n Node) IsParam() bool {
	return n.code >= spec.Param0 && n.code <= spec.ParamN
}

func (n Node) IsOp() bool {
	return n.code < 128
}

func (n Node) IsCode(xs ...spec.Op) bool {
	return forAny(xs, func(x spec.Op) bool { return x == n.code })
}

func (n Node) Param() uint32 {
	return n.param
}

func (node Node) Literal() Value {
	if node.code.IsSmallBitArray() {
		l, d := node.code.SmallBitArray()
		return decodeBitArray(bitbuf.FromBytes([]byte{d}).Slice(0, l))
	}
	switch node.code {
	case spec.LiteralB8, spec.LiteralB16, spec.LiteralB32, spec.LiteralB64, spec.LiteralB128, spec.LiteralB256:
		// Defensive programming: check if litBits has the expected size
		expectedBytes := node.code.DataBits() / 8
		if len(node.litBits) < expectedBytes {
			panic(fmt.Sprintf("Node.Literal(): opcode %v expects %d bytes but litBits has length %d. This indicates a bug in node creation.", node.code, expectedBytes, len(node.litBits)))
		}
		return decodeBitArray(bitbuf.FromBytes(node.litBits).Slice(0, node.code.DataBits()))
	case spec.LiteralKind:
		return node.litKind
	case spec.LiteralAnyType:
		return node.litAnyType.Unwrap()
	case spec.LiteralAnyValue:
		return node.litAnyValue.Unwrap()
	default:
		panic(fmt.Sprintf("not a literal code=%v", node.code))
	}
}

// Size returns the size of the node in bits
func (n Node) Size() int {
	switch {
	case n.IsSelf():
		return spec.OpBits
	case n.IsParam():
		if n.code == spec.ParamN {
			return spec.OpBits + varintLen(n.param)*8
		} else {
			return spec.OpBits
		}
	case n.code.IsSmallBitArray():
		return spec.OpBits
	case n.code == spec.LiteralB8:
		return spec.OpBits + 1*8
	case n.code == spec.LiteralB16:
		return spec.OpBits + 2*8
	case n.code == spec.LiteralB32:
		return spec.OpBits + 4*8
	case n.code == spec.LiteralB64:
		return spec.OpBits + 8*8
	case n.code == spec.LiteralB128:
		return spec.OpBits + 16*8
	case n.code == spec.LiteralB256:
		return spec.OpBits + 32*8
	case n.IsCode(spec.LiteralKind):
		return spec.OpBits + spec.KindBits
	case n.IsCode(spec.LiteralAnyType):
		return spec.OpBits + spec.AnyTypeBits
	case n.IsCode(spec.LiteralAnyValue):
		return spec.OpBits + spec.AnyValueBits
	default:
		ret := int(spec.OpBits)
		for _, idx := range n.args[:n.code.InDegree()] {
			ret += varintLen(idx) * 8
		}
		return ret
	}
}

func (n Node) String() string {
	sb := &strings.Builder{}
	sb.WriteString("{")
	switch {
	case n.IsSelf():
		sb.WriteString("SELF")
	case n.IsParam():
		fmt.Fprintf(sb, "%%%d", n.param)
	case n.IsLiteralBits():
		fmt.Fprintf(sb, "bits=%v", n.litBits)
	case n.IsCode(spec.LiteralKind):
		fmt.Fprintf(sb, "kind=%v", n.litKind)
	case n.IsCode(spec.LiteralAnyType):
		fmt.Fprintf(sb, "anyType=%v", n.litAnyType)
	case n.IsCode(spec.LiteralAnyValue):
		fmt.Fprintf(sb, "anyValue=%v", n.litAnyValue)
	default:
		fmt.Fprintf(sb, "%v %v", n.code, n.args[:n.code.InDegree()])
	}
	sb.WriteString("}")
	return sb.String()
}

type ProgCtx struct {
	Lookup func(i uint32) (Node, error)
	Self   func() (Node, error)
}

// Closure closes over x and appends the new Prog to out
// Any Params less than level will not be bound, and >= level will be replaced using pctx.
func Closure(out Prog, pctx ProgCtx, level uint32, bindSelf bool, x Prog) (Prog, error) {
	appendOp := func(out Prog, op spec.Op, idxs ...int) Prog {
		offsets := slices2.Map(idxs, func(absPos int) uint32 {
			return uint32(len(out) - absPos)
		})
		return append(out, OpNode(op, offsets...))
	}
	node := x.Root()
	switch {
	case node.IsLiteral():
		return append(out, node), nil
	case node.IsSelf():
		if !bindSelf {
			return append(out, node), nil
		}
		self, err := pctx.Self()
		if err != nil {
			return nil, err
		}
		return append(out, self), nil
	case node.IsParam():
		if node.Param() < level {
			return append(out, node), nil
		}
		// TODO: double-check the math for this index
		val, err := pctx.Lookup(node.Param() - level + 1)
		if err != nil {
			return nil, err
		}
		return append(out, val), nil
	case node.IsCode(spec.Let):
		out, err := Closure(out, pctx, level, bindSelf, x.Input(0))
		if err != nil {
			return nil, err
		}
		valueIdx := len(out) - 1
		out, err = Closure(out, pctx, level+1, bindSelf, x.Input(1))
		if err != nil {
			return nil, err
		}
		bodyIdx := len(out) - 1
		return appendOp(out, spec.Let, valueIdx, bodyIdx), nil
	case node.IsCode(spec.Lambda):
		out, err := Closure(out, pctx, level, bindSelf, x.Input(0))
		if err != nil {
			return nil, err
		}
		inTypeIdx := len(out) - 1
		out, err = Closure(out, pctx, level, bindSelf, x.Input(1))
		if err != nil {
			return nil, err
		}
		outTypeIdx := len(out) - 1
		out, err = Closure(out, pctx, level+1, false, x.Input(2))
		if err != nil {
			return nil, err
		}
		bodyIdx := len(out) - 1
		return appendOp(out, spec.Lambda, inTypeIdx, outTypeIdx, bodyIdx), nil
	case node.IsCode(spec.Lazy):
		out, err := Closure(out, pctx, level, bindSelf, x.Input(0))
		if err != nil {
			return nil, err
		}
		return appendOp(out, spec.Lazy, len(out)-1), nil
	case node.IsCode(spec.Fractal):
		out, err := Closure(out, pctx, level, false, x.Input(0))
		if err != nil {
			return nil, err
		}
		return appendOp(out, spec.Fractal, len(out)-1), nil
	default:
		var idxs []int
		for i := 0; i < x.NumInputs(); i++ {
			var err error
			out, err = Closure(out, pctx, level, bindSelf, x.Input(i))
			if err != nil {
				return nil, err
			}
			idxs = append(idxs, len(out)-1)
		}
		return appendOp(out, x.Code(), idxs...), nil
	}
}

func varintLen(x uint32) int {
	var buf [binary.MaxVarintLen64]byte
	return binary.PutUvarint(buf[:], uint64(x))
}

func divCeil[T constraints.Integer](a, b T) T {
	y := a / b
	if a%b > 0 {
		y++
	}
	return y
}

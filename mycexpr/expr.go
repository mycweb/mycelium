// package mycexpr has an Expression Builder for assembling computational values
// like Progs, Exprs, Lazys, Lambdas, and Fractals.
package mycexpr

import (
	"fmt"
	"math/bits"
	"strings"

	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

type Node = myc.Node

// Expr is a description of a computation, waiting to be replaced with a value
// Expr = Literal | Param | Op
type Expr struct {
	// op is always set
	op spec.Op
	// args is set for computations
	args [3]*Expr
	// literal is set for literals
	literal myc.Value
}

func FromMycelium(x *myc.Expr) *Expr {
	var e Expr
	cache := make(map[uint32]*Expr)
	e.decode(x.Prog(), cache)
	return &e
}

func (e *Expr) Build() *myc.Expr {
	cache := make(map[*Expr]uint32)
	prog := e.encode(nil, cache)
	return myc.NewExpr(prog)
}

func (e *Expr) encode(out myc.Prog, cache map[*Expr]uint32) myc.Prog {
	switch {
	case e.IsParam():
		return append(out, myc.Param(e.Param()))
	case e.IsSelf():
		return append(out, myc.Self())
	case e.IsLiteral():
		return append(out, myc.Literal(e.Value()))
	default:
		var idxs [3]uint32
		for i, arg := range e.args[:e.op.InDegree()] {
			idx, exists := cache[arg]
			if !exists {
				out = arg.encode(out, cache)
				idx = uint32(len(out) - 1)
				cache[arg] = idx
			}
			idxs[i] = idx
		}
		var offsets [3]uint32
		for i, idx := range idxs[:e.op.InDegree()] {
			offsets[i] = uint32(len(out)) - idx
		}
		return append(out, myc.OpNode(e.op, offsets[:e.op.InDegree()]...))
	}
}

func (e *Expr) decode(prog myc.Prog, cache map[uint32]*Expr) {
	if e == nil {
		panic("decode nil")
	}
	if len(prog) == 0 {
		panic("empty prog")
	}
	idx := uint32(len(prog) - 1)
	if e2, exists := cache[idx]; exists {
		*e = *e2
		return
	}
	node := prog[idx]
	e.op = node.Code()

	switch {
	case node.IsSelf():
		*e = *Self()
	case node.IsParam():
		*e = *Param(node.Param())
	case node.IsLiteral():
		*e = *literal(node.Literal())
	default:
		var i int
		for arg := range prog.Inputs() {
			var e2 Expr
			e2.decode(arg, cache)
			e.args[i] = &e2
			i++
		}
	}
}

func (e *Expr) IsLiteral() bool {
	return e.IsLiteralBits() ||
		e.IsCode(spec.LiteralKind, spec.LiteralAnyType, spec.LiteralAnyValue)
}

func (e *Expr) IsLiteralBits() bool {
	return e.op >= spec.LiteralB0 && e.op <= spec.LiteralB256
}

func (e *Expr) IsParam() bool {
	return e.op.IsParam()
}

func (e *Expr) IsSelf() bool {
	return e.op == spec.Self
}

// InDegree returns the number of arguments this expression takes.
// It will always be <= 4
func (e *Expr) InDegree() int {
	if e.IsLiteral() {
		return 0
	}
	if e.IsParam() {
		return 0
	}
	if e.IsSelf() {
		return 0
	}
	return e.op.InDegree()
}

func (e *Expr) OpCode() spec.Op {
	return e.op
}

func (e *Expr) ArgExprs() []*Expr {
	ret := e.args[:e.op.InDegree()]
	// check that all the sub expressions are non-nil
	for i := range ret {
		if ret[i] == nil {
			panic(fmt.Sprint(e.op, ret))
		}
	}
	return ret
}

func (e *Expr) Arg(i int) *Expr {
	return e.args[i]
}

// IsCode returns true if the expression's code is any of xs
func (e *Expr) IsCode(xs ...spec.Op) bool {
	if e == nil {
		return false
	}
	return forAny(xs, func(c spec.Op) bool { return e.op == c })
}

// HasCode returns true if the expression or any of it's children have a code in cs
func (e *Expr) HasCode(cs ...spec.Op) bool {
	if e == nil {
		return false
	}
	if e.IsCode(cs...) {
		return true
	}
	return forAny(e.args[:], func(e *Expr) bool {
		return e.HasCode(cs...)
	})
}

// IsKind returns true if the expression would evaluate to a Kind
func (e *Expr) IsKind() bool {
	if e.IsLiteral() {
		_, isKind := e.Value().(*myc.Kind)
		return isKind
	}
	return false
}

func (e *Expr) isKind(kc spec.TypeCode) bool {
	if e.IsLiteral() {
		k, ok := e.Value().(*myc.Kind)
		return ok && k.TypeCode() == kc
	}
	return false
}

func (e *Expr) IsKindKind() bool {
	return e.isKind(spec.TC_Kind)
}

func (e *Expr) IsBitKind() bool {
	return e.isKind(spec.TC_Bit)
}

func (e *Expr) IsArrayKind() bool {
	return e.isKind(spec.TC_Array)
}

func (e *Expr) IsListKind() bool {
	return e.isKind(spec.TC_List)
}

func (e *Expr) IsRefKind() bool {
	return e.isKind(spec.TC_Ref)
}

func (e *Expr) IsSumKind() bool {
	return e.isKind(spec.TC_Sum)
}

func (e *Expr) IsProductKind() bool {
	return e.isKind(spec.TC_Product)
}

func (e *Expr) IsExprKind() bool {
	return e.isKind(spec.TC_Prog)
}

func (e *Expr) IsLazyKind() bool {
	return e.isKind(spec.TC_Lazy)
}

func (e *Expr) IsLambdaKind() bool {
	return e.isKind(spec.TC_Lambda)
}

func (e *Expr) IsFractalKind() bool {
	return e.isKind(spec.TC_Fractal)
}

func (e *Expr) IsPortKind() bool {
	return e.isKind(spec.TC_Port)
}

func (e *Expr) IsDistinctKind() bool {
	return e.isKind(spec.TC_Distinct)
}

func (e *Expr) IsAnyTypeKind() bool {
	return e.isKind(spec.TC_AnyType)
}

func (e *Expr) IsAnyValueKind() bool {
	return e.isKind(spec.TC_AnyValue)
}

// IsType returns true if the expression would evaluate to a type
func (e *Expr) IsType() bool {
	if e.IsLiteral() {
		_, isType := e.Value().(myc.Type)
		return isType
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsKind()
}

func (e *Expr) IsArrayType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.ArrayType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsArrayKind()
}

func (e *Expr) IsListType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.ListType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsListKind()
}

func (e *Expr) IsRefType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.RefType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsRefKind()
}

func (e *Expr) IsSumType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(myc.SumType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsSumKind()
}

func (e *Expr) IsProductType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(myc.ProductType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsProductKind()
}

func (e *Expr) IsLazyType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.LazyType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsLazyKind()
}

func (e *Expr) IsLambdaType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.LambdaType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsLambdaKind()
}

func (e *Expr) IsFractalType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.FractalType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsFractalKind()
}

func (e *Expr) IsDistinctType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.DistinctType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsDistinctKind()
}

func (e *Expr) IsPortType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(*myc.PortType)
		return ok
	}
	return e.IsCode(spec.Craft) && e.Arg(0).IsPortKind()
}

func (e *Expr) IsAnyTypeType() bool {
	if e.IsLiteral() {
		_, ok := e.Value().(myc.AnyTypeType)
		return ok
	}
	return false
}

// HasND returns true if the expr or any sub-expr is non-determinisitic
func (e *Expr) HasND() bool {
	return e.HasCode(spec.Input, spec.Interact)
}

// HasEffect returns true if the expr or any sub-expr has an effect
func (e *Expr) HasEffect() bool {
	return e.HasCode(spec.Output, spec.Interact)
}

// IsKnown returns true if the expression could be evaluated statically
// IsKnown only returns false when the Expr or a child contains a parameter or a self reference.
func (e *Expr) IsKnown() bool {
	switch {
	case e == nil:
		return true
	case e.IsParam():
		return false
	case e.IsSelf():
		return false
	case e.IsLiteral():
		return true
	case e.IsCode(spec.Input, spec.Interact):
		return false
	default:
		return forAll(e.args[:], (*Expr).IsKnown)
	}
}

// equalExprs returns true if a and b would serialize to the same bits.
func equalExprs(a, b *Expr) bool {
	switch {
	case a == b:
		return true
	case a == nil || b == nil:
		return false
	case a.OpCode() != b.OpCode():
		return false
	case a.Param() != b.Param():
		return false
	case a.IsLiteral() && b.IsLiteral():
		return myc.Equal(a.Value(), b.Value())
	default:
		for i := range a.args {
			if !equalExprs(a.args[i], b.args[i]) {
				return false
			}
		}
		return true
	}
}

func (e *Expr) String() string {
	switch {
	case e.IsLiteral():
		return fmt.Sprintf("(%v)", e.Value())
	case e.IsParam():
		return `%` + fmt.Sprintf("%d", e.Param())
	default:
		sb := &strings.Builder{}
		fmt.Fprintf(sb, "(%v", e.OpCode())
		for _, a := range e.args {
			if a == nil {
				break
			}
			sb.WriteString(" ")
			fmt.Fprintf(sb, "%v", a)
		}
		sb.WriteString(")")
		return sb.String()
	}
}

// Value return the value this expression evaluates to if it is a literal
// If !e.IsLiteral(); Value() will panic
func (e *Expr) Value() myc.Value {
	if !e.IsLiteral() {
		panic("Expr.Value called on non-literal")
	}
	return e.literal
}

func (e *Expr) Param() uint32 {
	if !e.IsParam() {
		panic("Expr.Param called on non-param")
	}
	return uint32(e.op - spec.Param0)
}

func (e *Expr) DFS(fn func(*Expr) error) error {
	if e == nil {
		return nil
	}
	for _, a := range e.args {
		if err := a.DFS(fn); err != nil {
			return err
		}
	}
	return fn(e)
}

func (e *Expr) MapLeaves(fn func(*Expr) *Expr) *Expr {
	switch {
	case e == nil:
		return e
	case e.IsLiteral() || e.IsParam() || e.IsSelf():
		return fn(e)
	default:
		e2 := *e
		for i := range e2.args {
			e2.args[i] = e2.args[i].MapLeaves(fn)
		}
		return &e2
	}
}

func (e *Expr) MapErr(fn func(*Expr) (*Expr, error)) (*Expr, error) {
	if e == nil {
		return nil, nil
	}
	args := e.args
	for i := range args {
		if args[i] == nil {
			continue
		}
		a, err := fn(args[i])
		if err != nil {
			return nil, err
		}
		args[i] = a
	}
	if args == e.args {
		return e, nil
	}
	return &Expr{op: e.op, args: args}, nil
}

func newExpr(opc spec.Op, args ...*Expr) *Expr {
	if opc.InDegree() != len(args) {
		panic(fmt.Sprintf("%v has wrong indegree. HAVE: %d WANT: %d", opc, len(args), opc.InDegree()))
	}
	ret := &Expr{op: opc}
	copy(ret.args[:], args)
	return ret
}

// Param creates a new Parameter Expression
// Parameters are referenced using DeBruijn indicies.  https://en.wikipedia.org/wiki/De_Bruijn_index
// Each lambda function creates one parameter which is conventionally an argument tuple.
// Parameter 0 is the formal parameter to the innermost function.
func Param(x uint32) *Expr {
	if x > uint32(spec.ParamN-spec.Param0) {
		panic(x)
	}
	return &Expr{
		op: spec.Op(x + uint32(spec.Param0)),
	}
}

func Self() *Expr {
	return &Expr{op: spec.Self}
}

func Literal(x myc.Value) *Expr {
	return literal(x)
}

func literal(v myc.Value) *Expr {
	switch x := v.(type) {
	case *myc.Bit:
		if x.AsBool() {
			return newExpr(spec.ONE)
		} else {
			return newExpr(spec.ZERO)
		}
	case myc.AsBitArray:
		return literalBits(x)
	case *myc.Kind:
		return literalKind(x)
	case myc.Type:
		if at, ok := x.(*myc.AnyType); ok {
			return literalAnyValue(myc.NewAnyValue(at))
		}
		return literalAnyType(myc.NewAnyType(x))
	}
	return literalAnyValue(myc.NewAnyValue(v))
}

func literalKind(k *myc.Kind) *Expr {
	return &Expr{op: spec.LiteralKind, literal: k}
}

func literalBits(x myc.AsBitArray) *Expr {
	l := x.AsBitArray().Len()
	switch l {
	case 0:
		return &Expr{
			op:      spec.LiteralB0,
			literal: x,
		}
	case 2, 4, 8, 16, 32, 64, 128, 256:
		return &Expr{
			op:      spec.LiteralB2 + spec.Op(bits.TrailingZeros(uint(l))-1),
			literal: x,
		}
	default:
		return literalAnyValue(myc.NewAnyValue(x))
	}
}

func literalAnyType(x *myc.AnyType) *Expr {
	return &Expr{op: spec.LiteralAnyType, literal: x.Unwrap()}
}

func literalAnyValue(x *myc.AnyValue) *Expr {
	return &Expr{op: spec.LiteralAnyValue, literal: x.Unwrap()}
}

// lit is a shorter name for literal
func lit(x myc.Value) *Expr {
	return literal(x)
}

// Match checks if x matches pattern.
// - If pattern is equivalent to x, then it matches
// - If pattern is a parameter, then it matches everything.
// - If a parameter is used more than once all matches must be the same value
// Additionally, if pattern is a parameter Match assigns dst[pattern.Param()] = x
func Match(dst []*Expr, pattern *Expr, x *Expr, litMatch func(a, b myc.Value) bool) bool {
	for i := range dst {
		dst[i] = nil
	}
	if litMatch == nil {
		litMatch = func(a, b myc.Value) bool {
			return myc.Equal(a, b)
		}
	}
	var match func(*Expr, *Expr) bool
	match = func(pattern *Expr, x *Expr) bool {
		switch {
		case pattern == x:
			if pattern.IsParam() {
				dst[pattern.Param()] = x
			}
			return true
		case pattern == nil || x == nil:
			return false
		case pattern.IsLiteral() && x.IsLiteral():
			return litMatch(pattern.Value(), x.Value())
		case pattern.IsParam():
			i := pattern.Param()
			if dst[i] == nil {
				dst[i] = x
			}
			return equalExprs(dst[i], x)

		case pattern.OpCode() != x.OpCode():
			return false
		default:
			for i := range pattern.args {
				if !match(pattern.args[i], x.args[i]) {
					return false
				}
			}
			return true
		}
	}
	return match(pattern, x)
}

// forAll return true if fn(x) is true for all x
func forAll[E any, S ~[]E](xs S, fn func(E) bool) bool {
	for i := range xs {
		if !fn(xs[i]) {
			return false
		}
	}
	return true
}

// forAny returns true if fn(x) is true for any x
func forAny[E any, S ~[]E](xs S, fn func(E) bool) bool {
	for i := range xs {
		if fn(xs[i]) {
			return true
		}
	}
	return false
}

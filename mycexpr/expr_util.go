package mycexpr

import (
	"fmt"

	"go.brendoncarroll.net/exp/slices2"

	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

// NewExpr creates an expression from an operation and args
func NewExpr(opc spec.Op, args ...*Expr) (*Expr, error) {
	if ac := opc.InDegree(); ac != len(args) {
		return nil, fmt.Errorf("%v takes %d arguments HAVE: %v", opc.String(), ac, args)
	}
	return newExpr(opc, args...), nil
}

func BuildLazy(ty myc.Type, fn func(eb EB) *Expr) (*myc.Lazy, error) {
	var eb EB
	body := eb.Lazy(fn(eb)).Arg(0)
	return myc.NewLazy(ty, body.Build())
}

func BuildLambda(in, out myc.Type, fn func(eb EB) *Expr) (*myc.Lambda, error) {
	var eb EB
	body := eb.Lambda(in, out, fn).Arg(2)
	return myc.NewLambda(in, out, body.Build())
}

func BuildFractalType(fn func(eb EB) *Expr) (*myc.FractalType, error) {
	var eb EB
	body := eb.Fractal(fn).Arg(0)
	return myc.NewFractalType(body.Build())
}

// EB is an expression builder
type EB struct {
	ctxLen  uint32
	hasSelf bool
}

func (eb EB) Bit(x myc.Bit) *Expr {
	return eb.Lit(&x)
}

func (eb EB) Lit(x myc.Value) *Expr {
	return Literal(x)
}

func (eb EB) P(x uint32) *Expr {
	if x >= eb.ctxLen {
		panic(x)
	}
	return Param(x)
}

func (eb EB) Self() *Expr {
	if !eb.hasSelf {
		panic("self not defined in this context")
	}
	return Self()
}

// Let creates a new lambda with body, and calls it with vars as a tuple of arguments
func (eb EB) Let(bind *Expr, fn func(eb EB) *Expr) *Expr {
	eb2 := eb
	eb2.ctxLen++
	return newExpr(spec.Let, bind, fn(eb2))
}

// Lambda returns an expression for a new lambda
func (eb EB) Lambda(inputType, outputType myc.Type, fn func(eb EB) *Expr) *Expr {
	eb2 := eb
	eb2.ctxLen++
	eb2.hasSelf = true
	body := fn(eb2)
	if err := myc.ValidateBody(body.Build(), eb2.ctxLen, true); err != nil {
		panic(err)
	}
	return newExpr(spec.Lambda,
		lit(inputType), lit(outputType),
		body,
	)
}

func (eb EB) Fractal(fn func(eb EB) *Expr) *Expr {
	eb2 := eb
	eb2.hasSelf = true
	return newExpr(spec.Fractal, fn(eb2))
}

func (eb EB) Lazy(x *Expr) *Expr {
	if err := myc.ValidateBody(x.Build(), eb.ctxLen, false); err != nil {
		panic(err)
	}
	return newExpr(spec.Lazy, x)
}

func (eb EB) Mux(table *Expr, pick *Expr) *Expr {
	return newExpr(spec.Mux, table, pick)
}

func (eb EB) BitType() *Expr {
	return Literal(myc.BitType{})
}

func (eb EB) BitArray(bs ...myc.Bit) *Expr {
	return eb.Array(eb.BitType(), slices2.Map(bs, func(x myc.Bit) *Expr {
		return eb.Bit(x)
	})...)
}

func (eb EB) B8(x uint8) *Expr {
	return Literal(myc.NewB8(x))
}

func (eb EB) B16(x uint16) *Expr {
	return Literal(myc.NewB16(x))
}

func (eb EB) B32(x uint32) *Expr {
	return Literal(myc.NewB32(x))
}

func (eb EB) B64(x uint64) *Expr {
	return Literal(myc.NewB64(x))
}

func (eb EB) ArrayUnit(x *Expr) *Expr {
	return newExpr(spec.ArrayUnit, x)
}

func (eb EB) Array(ty *Expr, xs ...*Expr) *Expr {
	switch len(xs) {
	case 0:
		return eb.ArrayEmpty(ty)
	case 1:
		return newExpr(spec.ArrayUnit, xs[0])
	default:
		l := len(xs)
		return newExpr(spec.Concat,
			eb.Array(ty, xs[:l/2]...),
			eb.Array(ty, xs[l/2:]...),
		)
	}
}

func (eb EB) ArrayEmpty(ty *Expr) *Expr {
	return newExpr(spec.ArrayEmpty, ty)
}

func (eb EB) Concat(left, right *Expr) *Expr {
	return newExpr(spec.Concat, left, right)
}

func (eb EB) List(xs ...*Expr) *Expr {
	if len(xs) == 0 {
		return eb.ListFrom(eb.Post(eb.Array(eb.Bottom())))
	}
	return eb.ListFrom(eb.Post(eb.Array(eb.TypeOf(xs[0]), xs...)))
}

func (eb EB) Encode(x *Expr) *Expr {
	return newExpr(spec.Encode, x)
}

func (eb EB) Decode(ty *Expr, data *Expr) *Expr {
	return newExpr(spec.Decode, ty, data)
}

// ListFrom takes a Ref[Array[T, _]] and returns a List[T]
func (eb EB) ListFrom(x *Expr) *Expr {
	return newExpr(spec.ListFrom, x)
}

// ListTo takes a List[T] and returns a Ref[Array[T, n]]
func (eb EB) ListTo(x *Expr, n int) *Expr {
	return newExpr(spec.ListTo, x, eb.B32(uint32(n)))
}

func (eb EB) String(x string) *Expr {
	return eb.Lit(myc.NewString(x))
}

// Apply returns an expression which applies a to args
func (eb EB) Apply(la *Expr, arg *Expr) *Expr {
	return newExpr(spec.Apply, la, arg)
}

func (eb EB) Craft(ty *Expr, x *Expr) *Expr {
	return newExpr(spec.Craft, ty, x)
}

func (eb EB) Uncraft(ty *Expr) *Expr {
	return newExpr(spec.Uncraft, ty)
}

// TypeOfExpr returns an expression for the type of the evaluation of x
func (EB) TypeOf(x *Expr) *Expr {
	return newExpr(spec.TypeOf, x)
}

func (EB) If(test, then, els *Expr) *Expr {
	// branch is in terms of the zero path and the ones path, opposite of convention
	return newExpr(spec.Branch, test, els, then)
}

func (EB) Equal(a, b *Expr) *Expr {
	return newExpr(spec.Equal, a, b)
}

func (eb EB) LetVal(v myc.Value, bodyFn func(eb EB) *Expr) *Expr {
	return eb.Let(Literal(v), bodyFn)
}

func (eb EB) Fault(x *Expr) *Expr {
	return newExpr(spec.Panic, x)
}

func (eb EB) AnyValueFrom(x *Expr) *Expr {
	return newExpr(spec.AnyValueFrom, x)
}

// AnyValueTo resolves to e if e is of type ty, or faults the machine.
func (eb EB) AnyValueTo(x *Expr, ty myc.Type) *Expr {
	return newExpr(spec.AnyValueTo, x, Literal(ty))
}

func (eb EB) AnyTypeFrom(x *Expr) *Expr {
	return newExpr(spec.AnyTypeFrom, x)
}

func (eb EB) Bottom() *Expr {
	return eb.Lit(myc.Bottom())
}

func (eb EB) ArrayType(ty, l *Expr) *Expr {
	ty = eb.AnyTypeFrom(ty)
	return newExpr(spec.Craft, lit(myc.ArrayKind()), EB{}.Product(ty, l))
}

func (eb EB) ArrayTypeElem(ty *Expr) *Expr {
	return eb.Field(eb.Uncraft(ty), 0)
}

func (eb EB) ArrayTypeLen(ty *Expr) *Expr {
	return eb.Field(eb.Uncraft(ty), 1)
}

func (eb EB) ListType(ty *Expr) *Expr {
	ty = eb.AnyTypeFrom(ty)
	return newExpr(spec.Craft, lit(myc.ListKind()), eb.Product(ty))
}

func (eb EB) ListTypeElem(ty *Expr) *Expr {
	return eb.Field(eb.Uncraft(ty), 0)
}

func (eb EB) RefType(elem *Expr) *Expr {
	elem = eb.AnyTypeFrom(elem)
	return newExpr(spec.Craft, lit(myc.RefKind()), eb.Product(elem))
}

func (eb EB) RefTypeElem(x *Expr) *Expr {
	return eb.Field(eb.Uncraft(x), 0)
}

func (eb EB) LazyType(elem *Expr) *Expr {
	elem = eb.AnyTypeFrom(elem)
	return newExpr(spec.Craft, lit(myc.LazyKind()), eb.Product(elem))
}

func (eb EB) LambdaType(in, out *Expr) *Expr {
	in = eb.AnyTypeFrom(in)
	out = eb.AnyTypeFrom(out)
	return newExpr(spec.Craft, lit(myc.LambdaKind()), eb.Product(in, out))
}

func (eb EB) LambdaTypeIn(lt *Expr) *Expr {
	return eb.Field(newExpr(spec.Uncraft, lt), 0)
}

func (eb EB) LambdaTypeOut(lt *Expr) *Expr {
	return eb.Field(newExpr(spec.Uncraft, lt), 1)
}

func (eb EB) DistinctType(base, mark *Expr) *Expr {
	base = eb.AnyTypeFrom(base)
	mark = eb.AnyValueFrom(mark)
	return newExpr(spec.Craft, lit(myc.DistinctKind()), eb.Product(base, mark))
}

func (eb EB) DistinctTypeBase(x *Expr) *Expr {
	return eb.Field(eb.Uncraft(x), 0)
}

func (eb EB) DistinctTypeMarkExpr(x *Expr) *Expr {
	return eb.Field(eb.Uncraft(x), 1)
}

func (eb EB) PortType(tell, recv, req, resp *Expr) *Expr {
	tell = eb.AnyTypeFrom(tell)
	recv = eb.AnyTypeFrom(recv)
	req = eb.AnyTypeFrom(req)
	resp = eb.AnyTypeFrom(resp)
	return newExpr(spec.Craft, lit(myc.PortKind()), eb.Product(
		tell,
		recv,
		req,
		resp,
	))
}

func (eb EB) ProductType(xs ...*Expr) *Expr {
	xs = slices2.Map(xs, eb.AnyTypeFrom)
	return newExpr(spec.Craft, lit(myc.ProductKind(len(xs))),
		eb.Array(eb.Lit(myc.AnyTypeType{}), xs...),
	)
}

func (eb EB) SumType(xs ...*Expr) *Expr {
	xs = slices2.Map(xs, eb.AnyTypeFrom)
	return newExpr(spec.Craft, lit(myc.SumKind(len(xs))),
		eb.Array(eb.Lit(myc.AnyTypeType{}), xs...),
	)
}

// ProductExpr returns an Expr which evaluates to a tuple containing
// Values
func (eb EB) Product(xs ...*Expr) *Expr {
	switch len(xs) {
	case 0:
		return newExpr(spec.ProductEmpty)
	case 1:
		return newExpr(spec.ProductUnit, xs[0])
	default:
		l := len(xs)
		return newExpr(spec.Concat,
			eb.Product(xs[:l/2]...),
			eb.Product(xs[l/2:]...),
		)
	}
}

func (eb EB) ArrayLen(x *Expr) *Expr {
	return eb.Section(eb.Encode(eb.TypeOf(x)), spec.AnyTypeBits, spec.AnyTypeBits+spec.SizeBits)
}

func (eb EB) ListLen(x *Expr) *Expr {
	return eb.Section(eb.Encode(x), spec.RefBits, spec.RefBits+spec.SizeBits)
}

func (eb EB) Field(x *Expr, i int) *Expr {
	return newExpr(spec.Field, x, eb.B32(uint32(i)))
}

func (EB) Slot(x *Expr, idx *Expr) *Expr {
	return newExpr(spec.Slot, x, idx)
}

func (eb EB) Section(x *Expr, beg, end int) *Expr {
	return newExpr(spec.Section, x, eb.B32(uint32(beg)), eb.B32(uint32(end)))
}

func (eb EB) Slice(x *Expr, beg, end *Expr) *Expr {
	return newExpr(spec.Slice, x, beg, end)
}

// Arg refers to a Product field in the Product at %0
func (eb EB) Arg(level uint32, i uint32) *Expr {
	return eb.Field(eb.P(level), int(i))
}

func (EB) Output(port *Expr, val *Expr) *Expr {
	return newExpr(spec.Output, port, val)
}

func (EB) Input(port *Expr) *Expr {
	return newExpr(spec.Input, port)
}

func (EB) Interact(port *Expr, val *Expr) *Expr {
	return newExpr(spec.Interact, port, val)
}

func (EB) Post(x *Expr) *Expr {
	return newExpr(spec.Post, x)
}

func (EB) Load(x *Expr) *Expr {
	return newExpr(spec.Load, x)
}

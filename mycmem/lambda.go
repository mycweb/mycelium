package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type LambdaType struct {
	inAT, outAT *AnyType
}

func NewLambdaType(in, out Type) *LambdaType {
	in = unwrapAnyType(in)
	out = unwrapAnyType(out)
	return &LambdaType{
		inAT:  NewAnyType(in),
		outAT: NewAnyType(out),
	}
}

func (*LambdaType) isValue() {}
func (*LambdaType) isType()  {}

func (lpt *LambdaType) Type() Type {
	return LambdaKind()
}

func (lpt *LambdaType) SizeOf() int {
	return ProductType{NewRefType(ProductType{}), B32Type()}.SizeOf()
}

func (lpt *LambdaType) Unmake() Value {
	return Product{lpt.inAT, lpt.outAT}
}

// In returns the type this lambda requires as input
func (lpt *LambdaType) In() Type {
	return lpt.inAT.Unwrap()
}

// Out returns the type this lambda produces as output
func (lpt *LambdaType) Out() Type {
	return lpt.outAT.Unwrap()
}

func (lpt *LambdaType) String() string {
	return fmt.Sprintf("|%v -> %v|", lpt.In(), lpt.Out())
}

func (lpt *LambdaType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		_ = yield(lpt.inAT) && yield(lpt.outAT)
	}
}

func (lt *LambdaType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return pullIntoBatch(ctx, dst, src, lt.inAT, lt.outAT)
}

func (lt *LambdaType) Encode(bb BitBuf) {
	Product{lt.inAT, lt.outAT}.Encode(bb)
}

func (lt *LambdaType) Decode(bb BitBuf, load LoadFunc) error {
	ats := [2]*AnyType{lt.inAT, lt.outAT}
	for i := range ats {
		beg := spec.AnyTypeBits * i
		end := beg + spec.AnyTypeBits
		if err := ats[i].Decode(bb.Slice(beg, end), load); err != nil {
			return err
		}
	}
	return nil
}

func (lt *LambdaType) Zero() Value {
	body := Prog{Literal(lt.Out().Zero())}
	return &Lambda{ty: lt, body: *NewAnyProg(body)}
}

type Lambda struct {
	ty   *LambdaType
	body AnyProg
}

// NewLambda creates a new Lambda from a body
// - body must not contain free parameters
func NewLambda(in Type, out Type, body *AnyProg) (*Lambda, error) {
	in = unwrapAnyType(in)
	out = unwrapAnyType(out)
	if body == nil {
		return nil, fmt.Errorf("nil lambda body")
	}
	if err := ValidateBody(body, 1, true); err != nil {
		return nil, err
	}
	la := &Lambda{ty: NewLambdaType(in, out)}
	la.body = *body
	return la, nil
}

func (la *Lambda) isValue() {}

func (la *Lambda) LambdaType() *LambdaType {
	return la.ty
}

func (la *Lambda) Type() Type {
	return la.LambdaType()
}

func (la *Lambda) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return la.body.PullInto(ctx, dst, src)
}

func (la *Lambda) Encode(buf BitBuf) {
	la.body.Encode(buf)
}

func (la *Lambda) Decode(buf BitBuf, load LoadFunc) error {
	var body AnyProg
	if err := body.Decode(buf, load); err != nil {
		return err
	}
	lt := la.LambdaType()
	la2, err := NewLambda(lt.In(), lt.Out(), &body)
	if err != nil {
		return err
	}
	*la = *la2
	return nil
}

func (la *Lambda) String() string {
	return fmt.Sprintf("|%v => %v|", la.ty.In(), la.ty.Out())
}

func (la *Lambda) Body() *AnyProg {
	return &la.body
}

func (la *Lambda) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(&la.body)
	}
}

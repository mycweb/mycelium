package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

var _ Type = &LazyType{}

type LazyType struct {
	elemAT *AnyType
}

func NewLazyType(elem Type) *LazyType {
	elem = unwrapAnyType(elem)
	return &LazyType{NewAnyType(elem)}
}

func (*LazyType) isValue() {}
func (*LazyType) isType()  {}
func (*LazyType) Type() Type {
	return LazyKind()
}

func (lt *LazyType) Elem() Type {
	return lt.elemAT.Unwrap()
}

func (t *LazyType) SizeOf() int {
	return spec.RefBits + spec.SizeBits
}

func (t *LazyType) Unmake() Value {
	return Product{t.elemAT}
}

func (t *LazyType) String() string {
	return fmt.Sprintf("Lazy[%v]", t.elemAT.Unwrap())
}

func (t *LazyType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(NewAnyType(t.elemAT.Unwrap()))
	}
}

func (lt *LazyType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return lt.elemAT.PullInto(ctx, dst, src)
}

func (lt *LazyType) Encode(bb BitBuf) {
	lt.elemAT.Encode(bb)
}

func (lt *LazyType) Decode(bb BitBuf, load LoadFunc) error {
	var at AnyType
	if err := at.Decode(bb, load); err != nil {
		return err
	}
	lt.elemAT = &at
	return nil
}

func (lt *LazyType) Zero() Value {
	return &Lazy{
		body: *NewExpr(Prog{
			Literal(lt.Elem().Type()),
		}),
		ty: lt,
	}
}

type Lazy struct {
	ty   *LazyType
	body Expr
}

func NewLazy(ty Type, body *Expr) (*Lazy, error) {
	if err := ValidateBody(body, 0, false); err != nil {
		return nil, err
	}
	la := &Lazy{
		ty:   NewLazyType(ty),
		body: *body,
	}
	return la, nil
}

func (*Lazy) isValue() {}

func (l *Lazy) Type() Type {
	return l.ty
}

func (l *Lazy) Body() *Expr {
	return &l.body
}

func (l *Lazy) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(&l.body)
	}
}

func (la *Lazy) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return la.body.PullInto(ctx, dst, src)
}

func (la *Lazy) Encode(buf BitBuf) {
	la.body.Encode(buf)
}

func (la *Lazy) Decode(buf BitBuf, load LoadFunc) error {
	return la.body.Decode(buf, load)
}

package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type DistinctType struct {
	base Type
	mark Value
}

func NewDistinctType(base Type, mark Value) *DistinctType {
	base = unwrapAnyType(base)
	mark = unwrapAnyValue(mark)
	return &DistinctType{base: base, mark: mark}
}

func (*DistinctType) isValue() {}

func (*DistinctType) isType() {}

func (t *DistinctType) Type() Type {
	return DistinctKind()
}

func (t *DistinctType) SizeOf() int {
	return t.base.SizeOf()
}

func (t *DistinctType) Base() Type {
	return t.base
}

func (t *DistinctType) Mark() Value {
	return t.mark
}

func (t *DistinctType) Unmake() Value {
	return Product{NewAnyType(t.base), NewAnyValue(t.mark)}
}

func (dt *DistinctType) New(x Value) (*Distinct, error) {
	if !TypeContains(dt.base, x) {
		return nil, fmt.Errorf("NewDistinct: value not member of base type: %v !<: %v", x, dt.base)
	}
	return &Distinct{ty: dt, val: x}, nil
}

func (dt *DistinctType) MustNew(x Value) Value {
	ret, err := dt.New(x)
	if err != nil {
		panic(err)
	}
	return ret
}

// Make implements the Maker interface
func (dt *DistinctType) Make(x Value) Value {
	return dt.MustNew(x)
}

func (dt *DistinctType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return pullIntoBatch(ctx, dst, src, NewAnyType(dt.base), NewAnyValue(dt.mark))
}

func (dt *DistinctType) Encode(bb BitBuf) {
	Product{
		NewAnyType(dt.base),
		NewAnyValue(dt.mark),
	}.Encode(bb)
}

func (dt *DistinctType) Decode(bb BitBuf, load LoadFunc) error {
	var base AnyType
	if err := base.Decode(bb.Slice(0, spec.AnyTypeBits), load); err != nil {
		return err
	}
	var mark AnyValue
	if err := mark.Decode(bb.Slice(spec.AnyTypeBits, bb.Len()), load); err != nil {
		return err
	}
	dt.base = base.Unwrap()
	dt.mark = mark.Unwrap()
	return nil
}

func (dt *DistinctType) Zero() Value {
	return dt.Make(dt.base.Zero())
}

func (dt *DistinctType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		_ = yield(NewAnyType(dt.base)) && yield(NewAnyValue(dt.mark))
	}
}

func (dt *DistinctType) String() string {
	return fmt.Sprintf("Distinct[base=%v, mark=%v]", dt.base, dt.mark)
}

type Distinct struct {
	ty  *DistinctType
	val Value
}

func (*Distinct) isValue() {}

func (d *Distinct) Type() Type {
	return d.ty
}

func (d *Distinct) Unwrap() Value {
	return d.val
}

func (d *Distinct) Unmake() Value {
	return d.Unwrap()
}

func (d *Distinct) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return d.val.PullInto(ctx, dst, src)
}

func (x *Distinct) Encode(bb BitBuf) {
	x.val.Encode(bb)
}

func (x *Distinct) Decode(bb BitBuf, load LoadFunc) error {
	return x.val.Decode(bb, load)
}

func (dt *Distinct) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(dt.val)
	}
}

func (dt *Distinct) String() string {
	return fmt.Sprintf("D{%v}", dt.val)
}

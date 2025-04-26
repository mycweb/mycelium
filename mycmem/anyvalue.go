package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type AnyValueType struct{}

func (AnyValueType) isValue() {}
func (AnyValueType) isType()  {}

func (AnyValueType) String() string {
	return "AnyValue"
}

func (AnyValueType) Type() Type {
	return AnyValueKind()
}

func (AnyValueType) SizeOf() int {
	return spec.AnyValueBits
}

func (av AnyValueType) Unmake() Value {
	return Product{}
}

func (AnyValueType) Components() iter.Seq[Value] {
	return emptyIter
}

func (AnyValueType) PullInto(context.Context, cadata.PostExister, cadata.Getter) error { return nil }
func (AnyValueType) Encode(BitBuf)                                                     {}
func (AnyValueType) Decode(BitBuf, LoadFunc) error                                     { return nil }
func (AnyValueType) Zero() Value                                                       { return NewAnyValue(KindKind()) }

type AnyValue struct {
	x Value

	valRef *Ref
	at     *AnyType
}

func NewAnyValue(x Value) *AnyValue {
	valRef := mkRef(x)
	at := NewAnyType(x.Type())
	return &AnyValue{
		x: x,

		valRef: &valRef,
		at:     at,
	}
}

func (*AnyValue) isValue() {}

func (*AnyValue) Type() Type {
	return AnyValueType{}
}

func (av *AnyValue) GetType() *AnyType {
	return av.at
}

func (av *AnyValue) To(ty Type) (Value, error) {
	if !TypeContains(ty, av.x) {
		return nil, fmt.Errorf("FAULT: AnyValue.To HAVE: %v WANT: %v", ty, av.x.Type())
	}
	return av.x, nil
}

func (av *AnyValue) Unwrap() Value {
	return av.x
}

func (av *AnyValue) String() string {
	return fmt.Sprintf("AnyValue{%v}", av.x)
}

func (av *AnyValue) Encode(o bitbuf.Buf) {
	Product{av.valRef, av.at}.Encode(o)
}

func (av *AnyValue) Decode(bb bitbuf.Buf, load LoadFunc) error {
	codec := ProductType{
		NewRefType(ProductType{}),
		AnyTypeType{},
	}
	tup := codec.Zero().(Product)
	if err := tup.Decode(bb, load); err != nil {
		return err
	}
	valRef, at := tup[0].(*Ref), tup[1].(*AnyType)
	valRef = valRef.retype(at.Unwrap())
	val, err := load(*valRef)
	if err != nil {
		return err
	}
	av.x = val
	av.valRef = valRef
	av.at = at
	return nil
}

func (av *AnyValue) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	if err := av.at.PullInto(ctx, dst, src); err != nil {
		return err
	}
	if yes, err := dst.Exists(ctx, &av.valRef.cid); err != nil {
		return err
	} else if yes {
		return nil
	}
	if err := av.x.PullInto(ctx, dst, src); err != nil {
		return err
	}
	_, err := Post(ctx, dst, av.x)
	return err
}

func (av *AnyValue) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		_ = yield(av.at) && yield(av.valRef)
	}
}

func unwrapAnyValue(x Value) Value {
	if av, ok := x.(*AnyValue); ok {
		x = av.Unwrap()
	}
	return x
}

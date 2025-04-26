package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

// AnyType is a type that includes all possible types.
// NOTE: AnyType does not includes values of those types, just the types themselves.
type AnyTypeType struct{}

func (AnyTypeType) isValue() {}
func (AnyTypeType) isType()  {}

func (AnyTypeType) String() string {
	return "AnyType"
}

func (AnyTypeType) Type() Type {
	return AnyTypeKind()
}

func (at AnyTypeType) Unmake() Value {
	return Product{}
}

func (AnyTypeType) SizeOf() int {
	return spec.AnyTypeBits
}

func (AnyTypeType) PullInto(context.Context, cadata.PostExister, cadata.Getter) error { return nil }
func (AnyTypeType) Encode(BitBuf)                                                     {}
func (AnyTypeType) Decode(BitBuf, LoadFunc) error                                     { return nil }
func (AnyTypeType) Zero() Value                                                       { return NewAnyType(Bottom()) }

func (t AnyTypeType) Components() iter.Seq[Value] { return emptyIter }

type AnyType struct {
	x Type

	ref *Ref
	k   *Kind
}

func NewAnyType(x Type) *AnyType {
	x = unwrapAnyType(x)
	ref := mkRef(x)
	return &AnyType{
		x:   x,
		ref: &ref,
		k:   x.Type().(*Kind),
	}
}

func (*AnyType) isValue() {}
func (*AnyType) isType()  {}

func (*AnyType) Type() Type {
	return AnyTypeType{}
}

func (at *AnyType) Unwrap() Type {
	return at.x
}

func (at *AnyType) To(k *Kind) (Type, error) {
	if !TypeContains(k, at.x) {
		return nil, fmt.Errorf("AnyType.To wrong type. HAVE: %v. WANT: %v", k, at.x.Type())
	}
	return at.x, nil
}

func (at *AnyType) String() string {
	return fmt.Sprintf("AnyType{%v}", at.x)
}

func (at *AnyType) SizeOf() int {
	return at.x.SizeOf()
}

func (at *AnyType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	if yes, err := dst.Exists(ctx, &at.ref.cid); err != nil {
		return err
	} else if yes {
		return nil
	}
	if err := at.x.PullInto(ctx, dst, src); err != nil {
		return err
	}
	_, err := Post(ctx, dst, at.x)
	return err
}

func (at *AnyType) Encode(b bitbuf.Buf) {
	Product{at.ref, at.k}.Encode(b)
}

func (at *AnyType) Decode(b bitbuf.Buf, load LoadFunc) error {
	codec := ProductType{
		NewRefType(ProductType{}),
		KindKind(),
	}
	val := codec.Zero()
	if err := val.Decode(b, load); err != nil {
		return err
	}
	tup := val.(Product)
	ref, kind := tup[0].(*Ref), tup[1].(*Kind)
	ref = ref.retype(kind)
	ty, err := load(*ref)
	if err != nil {
		return err
	}
	at.x = ty.(Type)
	at.ref = ref
	at.k = kind
	return nil
}

func (at *AnyType) Zero() Value {
	return at.x.Zero()
}

func (at *AnyType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		_ = yield(at.k) && yield(at.ref)
	}
}

func unwrapAnyType(x Type) Type {
	if at, ok := x.(*AnyType); ok {
		x = at.Unwrap()
	}
	return x
}

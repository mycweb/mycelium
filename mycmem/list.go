package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"
)

// ListType is a type that includes Arrays of all lengths for a specific element type
type ListType struct {
	elemAT *AnyType
}

func (*ListType) isValue() {}
func (*ListType) isType()  {}

func ListOf(elem Type) *ListType {
	if elem == nil {
		panic("AnyLenArrayOf(nil)")
	}
	elem = unwrapAnyType(elem)
	return &ListType{elemAT: NewAnyType(elem)}
}

func (t *ListType) Elem() Type {
	return t.elemAT.Unwrap()
}

func (t *ListType) Type() Type {
	return ListKind()
}

func (t *ListType) Supersets(x Type) bool {
	switch x := x.(type) {
	case *ListType:
		return Supersets(t.Elem(), x.Elem())
	default:
		return false
	}
}

func (t *ListType) SizeOf() int {
	return ProductType{B32Type(), NewRefType(t.Elem())}.SizeOf()
}

func (lt *ListType) Unmake() Value {
	return Product{lt.elemAT}
}

func (t *ListType) String() string {
	elem := t.elemAT.Unwrap()
	if Equal(elem, ByteType()) {
		return "String"
	}
	return fmt.Sprintf("List[%v]", elem)
}

func (lt *ListType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return lt.elemAT.PullInto(ctx, dst, src)
}

func (lt *ListType) Encode(bb BitBuf) {
	lt.elemAT.Encode(bb)
}

func (lt *ListType) Decode(bb BitBuf, load LoadFunc) error {
	var at AnyType
	if err := at.Decode(bb, load); err != nil {
		return err
	}
	lt.elemAT = &at
	return nil
}

func (lt *ListType) Zero() Value {
	return NewList(lt.Elem())
}

func (t *ListType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(t.elemAT)
	}
}

var _ ArrayLike = &List{}

type List struct {
	a   ArrayLike
	ref *Ref
}

func (l *List) isValue() {}

func NewList(elem Type, vals ...Value) *List {
	elem = unwrapAnyType(elem)
	arr := NewArray(elem, vals...)
	return listFromArray(arr)
}

func listFromArray(arr ArrayLike) *List {
	ref := mkRef(arr)
	return &List{
		a:   arr,
		ref: &ref,
	}
}

func (l *List) Type() Type {
	return ListOf(l.a.Elem())
}

func (l *List) Elem() Type {
	return l.a.Elem()
}

func (l *List) Get(i int) Value {
	return l.a.Get(i)
}

func (l *List) Len() int {
	return l.a.Len()
}

func (l *List) String() string {
	return fmt.Sprint(l.a)
}

func (l *List) Array() ArrayLike {
	return l.a
}

func (l *List) Slice(beg, end int) ArrayLike {
	return listFromArray(l.a.Slice(beg, end))
}

func (l *List) Map(outType Type, fn func(Value) (Value, error)) (Mapable, error) {
	a2, err := l.a.Map(outType, fn)
	if err != nil {
		return nil, err
	}
	return listFromArray(a2.(ArrayLike)), nil
}

func (l *List) Reduce(fn func(Value, Value) (Value, error)) (Value, error) {
	return l.a.Reduce(fn)
}

func (l *List) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yieldAll(yield, l.ref, NewSize(l.a.Len()))
	}
}

func (list *List) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	if yes, err := dst.Exists(ctx, &list.ref.cid); err != nil {
		return err
	} else if yes {
		return nil
	}
	if err := list.a.PullInto(ctx, dst, src); err != nil {
		return err
	}
	_, err := Post(ctx, dst, list.a)
	return err
}

func (list *List) Encode(bb BitBuf) {
	Product{list.ref, NewSize(list.a.Len())}.Encode(bb)
}

func (list *List) Decode(o BitBuf, load LoadFunc) error {
	codec := ProductType{
		NewRefType(ProductType{}),
		SizeType(),
	}
	x := codec.Zero()
	if err := x.Decode(o, load); err != nil {
		return err
	}
	tup := x.(Product)
	ref, l := tup[0].(*Ref), tup[1].(*B32)
	ref = ref.retype(ArrayOf(list.a.Elem(), int(*l)))
	arr, err := load(*ref)
	if err != nil {
		return err
	}
	list.ref = ref
	list.a = arr.(ArrayLike)
	return nil
}

func StringType() *ListType {
	return ListOf(ByteType())
}

// NewString returns a List[Byte] from a string
func NewString(x string) *List {
	arr := ByteArray{d: []byte(x)}
	return listFromArray(arr)
}

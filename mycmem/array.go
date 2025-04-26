package mycmem

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"strings"

	"go.brendoncarroll.net/exp/slices2"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type ArrayType struct {
	elemAT *AnyType
	len    int
}

func (*ArrayType) isValue() {}
func (*ArrayType) isType()  {}

func ArrayOf(elem Type, l int) *ArrayType {
	if elem == nil {
		panic("ArrayOf(nil)")
	}
	elem = unwrapAnyType(elem)
	return &ArrayType{elemAT: NewAnyType(elem), len: l}
}

func (t *ArrayType) Elem() Type {
	return t.elemAT.Unwrap()
}

func (t *ArrayType) Len() int {
	return t.len
}

func (t *ArrayType) Type() Type {
	return ArrayKind()
}

func (t *ArrayType) Supersets(u Type) bool {
	if t2, ok := u.(*ArrayType); ok {
		return Supersets(t.Elem(), t2.Elem()) && t.Len() == t2.Len()
	}
	return false
}

func (t *ArrayType) SizeOf() int {
	if t.Len() == 0 {
		// This is necessary to prevent an infinite loop when encoding Kinds
		return 0
	}
	return t.Elem().SizeOf() * t.Len()
}

func (at *ArrayType) Zero() Value {
	elem := at.elemAT.Unwrap()
	var vals []Value
	for i := 0; i < at.Len(); i++ {
		vals = append(vals, elem.Zero())
	}
	return NewArray(elem, vals...)
}

func (at *ArrayType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return at.elemAT.PullInto(ctx, dst, src)
}

func (at *ArrayType) Encode(bb BitBuf) {
	l := Size(at.Len())
	Product{NewAnyType(at.Elem()), &l}.Encode(bb)
}

func (at *ArrayType) Decode(bb BitBuf, load LoadFunc) error {
	var elem AnyType
	if err := elem.Decode(bb.Slice(0, spec.AnyTypeBits), load); err != nil {
		return err
	}
	var l Size
	if err := l.Decode(bb.Slice(spec.AnyTypeBits, bb.Len()), load); err != nil {
		return err
	}
	at.elemAT = &elem
	at.len = int(l)
	return nil
}

func (t *ArrayType) Unmake() Value {
	return Product{t.elemAT, NewSize(t.len)}
}

func (t *ArrayType) String() string {
	return fmt.Sprintf("Array[%v, %v]", t.Elem(), t.Len())
}

func (t *ArrayType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yieldAll(yield, t.elemAT, NewSize(t.len))
	}
}

// Array is an ordered collection of values, all the same type, accessed by index.
type Array struct {
	ty Type
	vs []Value
}

// NewArray constructs a new array from an element type, and a list of values.
// NewArray may return optimized implementations like BitArray and ByteArray
// instead of the general Array.
func NewArray(ty Type, vs ...Value) ArrayLike {
	if ty == nil {
		panic("Array element type cannot be nil")
	}
	ty = unwrapAnyType(ty)
	for i := range vs {
		if !TypeContains(ty, vs[i]) {
			panic(fmt.Sprintf("%v does not contain %v : %v", ty, vs[i], vs[i].Type()))
		}
	}
	switch {
	case reflect.DeepEqual(ty, BitType{}):
		return NewBitArray(slices2.Map(vs, func(v Value) Bit {
			return *(v.(*Bit))
		})...)
	case reflect.DeepEqual(ty, ByteType()):
		return NewByteArray(slices2.Map(vs, func(v Value) byte {
			return byte(*v.(*B8))
		}))
	}
	return &Array{ty: ty, vs: vs}
}

func (a *Array) isValue() {}

func (a *Array) Elem() Type {
	return a.ty
}

func (a *Array) Get(i int) Value {
	return a.vs[i]
}

func (a *Array) Len() int {
	return len(a.vs)
}

func (a *Array) Type() Type {
	return ArrayOf(a.ty, len(a.vs))
}

func (a *Array) String() string {
	sb := strings.Builder{}
	sb.WriteString("[")
	for i := range a.vs {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%v", a.vs[i])
	}
	sb.WriteString("]")
	return sb.String()
}

func (a *Array) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return pullIntoBatch(ctx, dst, src, a.vs...)
}

func (a *Array) Encode(b BitBuf) {
	elemSize := int(a.Elem().SizeOf())
	for i, x := range a.vs {
		beg := i * elemSize
		end := beg + elemSize
		x.Encode(b.Slice(beg, end))
	}
}

func (a *Array) Decode(b BitBuf, load LoadFunc) error {
	size := a.ty.SizeOf()
	for i := 0; i < a.Len(); i++ {
		beg := i * size
		end := beg + size
		if err := a.vs[i].Decode(b.Slice(beg, end), load); err != nil {
			return err
		}
	}
	return nil
}

func (a *Array) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	panic("ArrayMap")
}

func (a *Array) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	panic("ArrayReduce")
}

func (a *Array) Slice(beg, end int) ArrayLike {
	return &Array{vs: a.vs[beg:end]}
}

func (a *Array) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yieldAll(yield, a.vs...)
	}
}

func ArrayZip(al1, al2 ArrayLike, elemTy Type, fn func(a, b Value) (Value, error)) (ArrayLike, error) {
	vals := make([]Value, max(al1.Len(), al2.Len()))
	for i := 0; i < al1.Len() && i < al2.Len(); i++ {
		ae := al1.Get(i)
		be := al2.Get(i)
		out, err := fn(ae, be)
		if err != nil {
			return nil, err
		}
		vals[i] = out
	}
	return NewArray(elemTy, vals...), nil
}

type ArrayLike interface {
	Composite
	Elem() Type
	Slice(beg, end int) ArrayLike

	Mapable
	Reducible
}

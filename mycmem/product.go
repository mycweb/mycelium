package mycmem

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"go.brendoncarroll.net/exp/slices2"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

// Product is a higher order type
// Product is the type of Product values
type ProductType []Type

func (t ProductType) isValue() {}
func (t ProductType) isType()  {}

func (t ProductType) Supersets(ty Type) bool {
	t2, ok := ty.(ProductType)
	if !ok {
		return false
	}
	if len(t) != len(t2) {
		return false
	}
	for i := range t {
		if !Supersets(t[i], t2[i]) {
			return false
		}
	}
	return true
}

func (t ProductType) Type() Type {
	return ProductKind(len(t))
}

func (t ProductType) Get(i int) Value {
	return t[i]
}

func (t ProductType) Len() int {
	return len(t)
}

func (t ProductType) SizeOf() (ret int) {
	for _, m := range t {
		ret += m.SizeOf()
	}
	return ret
}

func (t ProductType) Unmake() Value {
	return NewArray(AnyTypeType{}, slices2.Map(t, func(x Type) Value {
		return NewAnyType(x)
	})...)
}

func (t ProductType) String() string {
	sb := strings.Builder{}
	sb.WriteString("Product[")
	for i, t2 := range t {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprint(&sb, t2)
	}
	sb.WriteString("]")
	return sb.String()
}

func (pt ProductType) Zero() Value {
	ret := make(Product, len(pt))
	for i := range ret {
		ret[i] = pt[i].Zero()
	}
	return ret
}

func (pt ProductType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for i := range pt {
			if !yield(NewAnyType(pt[i])) {
				return
			}
		}
	}
}

func (pt ProductType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	for _, ty := range pt {
		at := NewAnyType(ty)
		if err := at.PullInto(ctx, dst, src); err != nil {
			return err
		}
	}
	return nil
}

func (pt ProductType) Encode(bb BitBuf) {
	for i := range pt {
		at := NewAnyType(pt[i])
		beg := i * spec.AnyTypeBits
		end := beg + spec.AnyTypeBits
		at.Encode(bb.Slice(beg, end))
	}
}

func (pt ProductType) Decode(bb BitBuf, load LoadFunc) error {
	for i := range pt {
		var at AnyType
		beg := i * spec.AnyTypeBits
		end := beg + spec.AnyTypeBits
		if err := at.Decode(bb.Slice(beg, end), load); err != nil {
			return err
		}
		pt[i] = at.Unwrap()
	}
	return nil
}

// Product is an ordered collection of values of arbitrary types.
// The type of Product types is a Product Type
type Product []Value

func (Product) isValue() {}

func (v Product) Type() Type {
	return ProductType(slices2.Map(v, func(x Value) Type { return x.Type() }))
}

func (p Product) Get(i int) Value {
	return p[i]
}

func (p Product) Len() int {
	return len(p)
}

func (v Product) String() string {
	sb := strings.Builder{}
	sb.WriteString("{")
	for i, v2 := range v {
		if i > 0 {
			sb.WriteString(" ")
		}
		fmt.Fprint(&sb, v2)
	}
	sb.WriteString("}")
	return sb.String()
}

func (p Product) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return pullIntoBatch(ctx, dst, src, p...)
}

func (p Product) Encode(bb BitBuf) {
	var beg int
	for i := range p {
		end := beg + int(p[i].Type().SizeOf())
		p[i].Encode(bb.Slice(beg, end))
		beg = end
	}
}

func (p Product) Decode(bb BitBuf, load LoadFunc) error {
	var beg int
	for i := range p {
		end := beg + int(p[i].Type().SizeOf())
		if err := p[i].Decode(bb.Slice(beg, end), load); err != nil {
			return err
		}
		beg = end
	}
	return nil
}

func (p Product) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yieldAll(yield, p...)
	}
}

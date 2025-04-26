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

type SumType []Type

func (t SumType) isValue() {}
func (t SumType) isType()  {}

func (t SumType) Type() Type {
	return SumKind(len(t))
}

func (st SumType) Get(i int) Value {
	return st[i]
}

func (st SumType) Len() int {
	return len(st)
}

func (t SumType) SizeOf() int {
	return t.TagSize() + t.ContentSize()
}

func (st SumType) Zero() Value {
	if len(st) == 0 {
		return &Sum{ty: st}
	}
	return &Sum{ty: st, tag: 0, val: st[0].Zero()}
}

func (st SumType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	for _, ty := range st {
		at := NewAnyType(ty)
		if err := at.PullInto(ctx, dst, src); err != nil {
			return err
		}
	}
	return nil
}

func (st SumType) Encode(bb BitBuf) {
	for i := range st {
		at := NewAnyType(st[i])
		beg := i * spec.AnyTypeBits
		end := beg + spec.AnyTypeBits
		at.Encode(bb.Slice(beg, end))
	}
}

func (st SumType) Decode(bb BitBuf, load LoadFunc) error {
	for i := range st {
		var at AnyType
		beg := i * spec.AnyTypeBits
		end := beg + spec.AnyTypeBits
		if err := at.Decode(bb.Slice(beg, end), load); err != nil {
			return err
		}
		st[i] = at.Unwrap()
	}
	return nil
}

// TagBits is the number of bits needed for the discriminant
func (t SumType) TagSize() int {
	return spec.SumTagBits(len(t))
}

func (t SumType) ContentSize() (ret int) {
	for _, m := range t {
		ret = max(ret, m.SizeOf())
	}
	return ret
}

func (t SumType) Unmake() Value {
	return NewArray(AnyTypeType{}, slices2.Map(t, func(x Type) Value {
		return NewAnyType(x)
	})...)
}

func (st SumType) String() string {
	if len(st) == 0 {
		return "Bottom"
	}
	sb := strings.Builder{}
	sb.WriteString("Sum[")
	for i := range st {
		if i > 0 {
			sb.WriteString(" ")
		}
		fmt.Fprintf(&sb, "%v", st[i])
	}
	sb.WriteString("]")
	return sb.String()
}

func (st SumType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for i := range st {
			if !yield(NewAnyType(st[i])) {
				return
			}
		}
	}
}

type Sum struct {
	ty  SumType
	tag int
	val Value
}

func (st SumType) New(tag int, v Value) (*Sum, error) {
	if tag >= int(len(st)) {
		return nil, fmt.Errorf("SumType only has %d variants, have tag=%d", len(st), tag)
	}
	if !TypeContains(st[tag], v) {
		return nil, fmt.Errorf("Sum field %d: %v cannot hold value %v", tag, st[tag], v)
	}
	return &Sum{ty: st, tag: tag, val: v}, nil
}

func (sum Sum) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return sum.val.PullInto(ctx, dst, src)
}

func (sum *Sum) Encode(b BitBuf) {
	b.Zero(0, b.Len())
	contentBuf := b.Slice(0, int(sum.ty.ContentSize()))
	tagBuf := b.Slice(int(sum.ty.ContentSize()), int(sum.ty.ContentSize()+sum.ty.TagSize()))

	// encode tag
	tag64 := B32(sum.tag).AsBitArray()
	var bs []Bit
	for i := 0; i < int(sum.ty.TagSize()); i++ {
		bs = append(bs, tag64.At(i))
	}
	NewBitArray(bs...).Encode(tagBuf)
	// encode content
	sum.val.Encode(contentBuf)
}

func (sum *Sum) Decode(b BitBuf, load LoadFunc) error {
	ty := sum.ty
	contentBuf := b.Slice(0, int(ty.ContentSize()))
	tagBuf := b.Slice(int(ty.ContentSize()), int(ty.ContentSize()+ty.TagSize()))
	// read tag
	bitArr := ArrayOf(BitType{}, int(ty.TagSize())).Zero().(AsUint32)
	if err := bitArr.Decode(tagBuf, load); err != nil {
		return err
	}
	tag := int(bitArr.AsUint32())
	if tag >= len(ty) {
		return fmt.Errorf("Sum.Decode: invalid tag=%v, type=%v", tag, ty)
	}
	// read content
	content := ty[tag].Zero()
	if err := content.Decode(contentBuf, load); err != nil {
		return err
	}

	if err := b.CheckZero(int(content.Type().SizeOf()), int(ty.ContentSize())); err != nil {
		return err
	}
	sum.tag = tag
	sum.val = content
	return nil
}

func (s *Sum) Tag() int {
	return s.tag
}

func (s *Sum) Get(i int) Value {
	if i != s.tag {
		panic(i)
	}
	return s.val
}

func (s *Sum) Len() int {
	return len(s.ty)
}

func (s *Sum) Unwrap() Value {
	return s.val
}

func (s *Sum) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(s.val)
	}
}

// MustSum calls NewSum and panics if there is an error.
func MustSum(ty SumType, tag int, v Value) *Sum {
	ret, err := ty.New(tag, v)
	if err != nil {
		panic(err)
	}
	return ret
}

func (v *Sum) isValue() {}

func (v *Sum) Type() Type {
	return v.ty
}

func (s *Sum) String() string {
	return fmt.Sprintf("Sum{%d: %v}", s.tag, s.val)
}

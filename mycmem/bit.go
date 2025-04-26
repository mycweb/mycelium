package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"

	"golang.org/x/exp/constraints"
)

type BitType struct{}

func (BitType) isValue() {}
func (BitType) isType()  {}

func (BitType) Type() Type {
	return BitKind()
}

func (BitType) String() string {
	return "Bit"
}

func (BitType) SizeOf() int { return 1 }

func (BitType) PullInto(context.Context, cadata.PostExister, cadata.Getter) error { return nil }
func (BitType) Encode(BitBuf)                                                     {}
func (BitType) Decode(BitBuf, LoadFunc) error                                     { return nil }
func (BitType) Zero() Value {
	return new(Bit)
}

func (b BitType) Unmake() Value {
	return Product{}
}

func (b BitType) Components() iter.Seq[Value] { return emptyIter }

type Bit uint8

func NewBit[T constraints.Integer](x T) *Bit {
	x = x & 1
	y := Bit(x)
	return &y
}

func (Bit) isValue() {}

func (v *Bit) Type() Type {
	return BitType{}
}

func (b *Bit) AsUint32() uint32 {
	return uint32(*b)
}

func (b *Bit) AsBool() bool {
	if *b&1 == 1 {
		return true
	} else {
		return false
	}
}

func (b *Bit) Components() iter.Seq[Value] { return emptyIter }

func (Bit) PullInto(context.Context, cadata.PostExister, cadata.Getter) error { return nil }

func (x *Bit) Encode(bb BitBuf) {
	bb.Put(0, uint8(*x))
}

func (x *Bit) Decode(bb BitBuf, _ LoadFunc) error {
	*x = Bit(bb.Get(0))
	return nil
}

func (x *Bit) String() string {
	return fmt.Sprint(*x)
}

func BitFromBool(x bool) *Bit {
	if x {
		return NewBit(1)
	} else {
		return NewBit(0)
	}
}

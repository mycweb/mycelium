package mycmem

import (
	"context"
	"fmt"
	"iter"
	"slices"

	"myceliumweb.org/mycelium/internal/cadata"
)

// ByteArray is an optimized implementation of Array[Array[Bit, 8], n]
type ByteArray struct {
	d []byte
}

// NewByteArray returns a new ByteArray from a []byte
func NewByteArray(xs []byte) ByteArray {
	xs = slices.Clone(xs)
	return ByteArray{d: xs}
}

func (ByteArray) isValue() {}

func (ba ByteArray) Type() Type {
	return ArrayOf(ByteType(), len(ba.d))
}

func (ba ByteArray) Len() int {
	return len(ba.d)
}

func (ba ByteArray) Elem() Type {
	return ByteType()
}

func (ba ByteArray) At(i int) byte {
	return ba.d[i]
}

func (ba ByteArray) Get(i int) Value {
	return ptrTo(B8(ba.d[i]))
}

func (ba ByteArray) AsString() string {
	return string(ba.d)
}

func (ba ByteArray) AsBytes() []byte {
	return slices.Clone(ba.d)
}

func (ba ByteArray) String() string {
	return fmt.Sprintf("%q", ba.d)
}

func (ba ByteArray) Slice(beg, end int) ArrayLike {
	return NewByteArray(ba.d[beg:end])
}

func (ba ByteArray) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	ys := make([]Value, len(ba.d))
	for i := range ba.d {
		y, err := fn((*B8)(&ba.d[i]))
		if err != nil {
			return nil, err
		}
		ys[i] = y
	}
	return NewArray(ty, ys...), nil
}

func (ba ByteArray) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	if len(ba.d) < 1 {
		return nil, fmt.Errorf("reduce on ByteArray with len=0")
	}
	acc := B8(ba.d[0])
	for i := 1; i < len(ba.d); i++ {
		acc2, err := fn(&acc, (*B8)(&ba.d[i]))
		if err != nil {
			return nil, err
		}
		acc = *acc2.(*B8)
	}
	return &acc, nil
}

func (ba ByteArray) Components() iter.Seq[Value] {
	return emptyIter
}

func (ByteArray) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (ba ByteArray) Encode(bb BitBuf) {
	for i := range ba.d {
		bb.Put8(i*8, ba.d[i])
	}
}

func (ba ByteArray) Decode(bb BitBuf, load LoadFunc) error {
	for i := range ba.d {
		ba.d[i] = bb.Get8(i * 8)
	}
	return nil
}

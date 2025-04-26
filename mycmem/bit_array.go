package mycmem

import (
	"context"
	"fmt"
	"iter"
	"math/bits"

	"myceliumweb.org/mycelium/internal/cadata"

	"golang.org/x/exp/constraints"
)

func ByteType() Type {
	return ArrayOf(BitType{}, 8)
}

func BitArrayType(n int) Type {
	return ArrayOf(BitType{}, n)
}

type BitArray struct {
	ws   []uint64
	bits int
}

func NewBitArray(bs ...Bit) AsBitArray {
	switch len(bs) {
	case 0:
		return BitArray{}
	case 8:
		var ret uint8
		for i := range bs {
			ret |= uint8(bs[i]) << i
		}
		return ptrTo(B8(ret))
	case 16:
		var ret uint16
		for i := range bs {
			ret |= uint16(bs[i]) << i
		}
		return ptrTo(B16(ret))
	case 32:
		var ret uint32
		for i := range bs {
			ret |= uint32(bs[i]) << i
		}
		return ptrTo(B32(ret))
	case 64:
		var ret uint64
		for i := range bs {
			ret |= uint64(bs[i]) << i
		}
		return ptrTo(B64(ret))
	}
	var ws []uint64
	var w uint64
	for i, b := range bs {
		if i > 0 && i%64 == 0 {
			ws = append(ws, w)
			w = 0
		}
		w |= uint64(b) << (i % 64)
	}
	ws = append(ws, w)
	return BitArray{ws: ws, bits: len(bs)}
}

func (BitArray) isValue() {}

func (z BitArray) Type() Type {
	return ArrayOf(z.Elem(), z.Len())
}

func (z BitArray) Elem() Type {
	return BitType{}
}

func (z BitArray) At(i int) Bit {
	return Bit(z.ws[i/64]>>(i%64)) & 1
}

func (z BitArray) Get(i int) Value {
	return ptrTo(z.At(i))
}

func (z BitArray) Len() int {
	return int(z.bits)
}

func (z BitArray) AsUint32() uint32 {
	if len(z.ws) == 0 {
		return 0
	}
	return uint32(z.ws[0])
}

func (z BitArray) AsBitArray() BitArray {
	return z
}

func (z BitArray) String() string {
	if z.bits <= 64 {
		return fmt.Sprintf("BitArray{0x%x, len=%v}", z.ws[0], z.bits)
	}
	return fmt.Sprintf("BitArray{len=%v}", z.bits)
}

func (z BitArray) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	panic("BitArray.Map")
}

func (z BitArray) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	panic("BitArray.Map")
}

func (z BitArray) Slice(beg, end int) ArrayLike {
	panic("BitArray")
}

func (z BitArray) Components() iter.Seq[Value] {
	return emptyIter
}

func (z BitArray) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (z BitArray) Encode(bb BitBuf) {
	for i := 0; i < z.Len(); i++ {
		bb.Put(i, uint8(z.At(i)))
	}
}

func (z BitArray) Decode(bb BitBuf, load LoadFunc) error {
	for i := 0; i < z.Len(); i++ {
		x := uint64(bb.Get(i)) << (i % 64)
		z.ws[i/64] &= ^x
		z.ws[i/64] |= x
	}
	return nil
}

// ModAdd performs modular addition
func ModAdd(a, b BitArray) BitArray {
	if a.Len() != b.Len() {
		panic("ZAdd not defined on different length arrays")
	}
	out := make([]uint64, len(a.ws))
	carryIn := uint64(0)
	for i := range a.ws {
		out[i], carryIn = bits.Add64(a.ws[i], b.ws[i], carryIn)
	}
	return BitArray{
		ws:   out,
		bits: a.bits,
	}
}

func ModSub(a, b BitArray) BitArray {
	if a.Len() != b.Len() {
		panic("ModSub not defined on different length arrays")
	}
	out := make([]uint64, len(a.ws))
	borrowIn := uint64(0)
	for i := range a.ws {
		out[i], borrowIn = bits.Sub64(a.ws[i], b.ws[i], borrowIn)
	}
	return BitArray{
		ws:   out,
		bits: b.bits,
	}
}

func ModMul(a, b BitArray) BitArray {
	if a.Len() != b.Len() {
		panic("ModMul not defined on different length arrays")
	}
	out := make([]uint64, len(a.ws)*2)
	for i := range a.ws {
		var carry uint64
		for j := range b.ws {
			high, low := bits.Mul64(a.ws[i], b.ws[j])
			sum, carryOut := bits.Add64(out[i+j], low, carry)
			out[i+j] = sum
			carry = high + carryOut
		}
	}
	return BitArray{
		ws:   out[:len(a.ws)],
		bits: b.bits,
	}
}

// AsBitArray is implemented by types that can be represented as a BitArray
type AsBitArray interface {
	ArrayLike
	AsBitArray() BitArray
}

var (
	_ AsBitArray = ptrTo(B8(0))
	_ AsBitArray = ptrTo(B16(0))
	_ AsBitArray = ptrTo(B32(0))
	_ AsBitArray = ptrTo(B64(0))
)

type AsUint32 interface {
	Value
	AsUint32() uint32
}

func B8Type() Type {
	return ArrayOf(BitType{}, 8)
}

type B8 uint8

func NewB8[T constraints.Integer](x T) *B8 {
	y := B8(x)
	return &y
}

func (*B8) isValue() {}
func (*B8) Type() Type {
	return ByteType()
}

func (n *B8) AsBitArray() BitArray {
	return BitArray{
		ws:   []uint64{uint64(*n)},
		bits: 8,
	}
}

func (n *B8) AsUint32() uint32 {
	return uint32(*n)
}

func (n *B8) Elem() Type {
	return BitType{}
}

func (n *B8) Get(i int) Value {
	return NewBit((*n >> i) & 1)
}

func (*B8) Len() int {
	return 8
}

func (*B8) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	panic("B8.Map")
}

func (*B8) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	panic("B8.Reduce")
}

func (n *B8) Slice(beg, end int) ArrayLike {
	return n
}

func (n *B8) Components() iter.Seq[Value] {
	return emptyIter
}

func (*B8) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (n *B8) Encode(bb BitBuf) {
	bb.Put8(0, uint8(*n))
}

func (n *B8) Decode(bb BitBuf, load LoadFunc) error {
	*n = B8(bb.Get8(0))
	return nil
}

func B16Type() Type {
	return BitArrayType(16)
}

type B16 uint16

func NewB16[T constraints.Integer](x T) *B16 {
	y := B16(x)
	return &y
}

func (*B16) isValue() {}

func (*B16) Type() Type {
	return B16Type()
}

func (*B16) Elem() Type {
	return BitType{}
}

func (n *B16) AsBitArray() BitArray {
	return BitArray{
		ws:   []uint64{uint64(*n)},
		bits: 16,
	}
}

func (n *B16) Get(i int) Value {
	return NewBit((*n >> i) & 1)
}

func (*B16) Len() int {
	return 16
}

func (n *B16) Slice(beg, end int) ArrayLike {
	return n
}

func (B16) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	panic("B16.Map")
}

func (B16) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	panic("B16.Reduce")
}

func (n B16) Components() iter.Seq[Value] {
	return emptyIter
}

func (*B16) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (n *B16) Encode(bb BitBuf) {
	bb.Put16(0, uint16(*n))
}

func (n *B16) Decode(bb BitBuf, load LoadFunc) error {
	*n = B16(bb.Get16(0))
	return nil
}

func (n *B16) String() string {
	return fmt.Sprint(*n)
}

func B32Type() Type {
	return ArrayOf(BitType{}, 32)
}

type B32 uint32

func NewB32[T constraints.Integer](x T) *B32 {
	y := B32(x)
	return &y
}

func (n B32) ToUint32() uint32 {
	return uint32(n)
}

func (n B32) isValue() {}

func (n B32) Type() Type {
	return B32Type()
}

func (B32) Elem() Type {
	return BitType{}
}

func (n *B32) Get(i int) Value {
	return NewBit((*n >> i) & 1)
}

func (n B32) AsBitArray() BitArray {
	return BitArray{
		ws:   []uint64{uint64(n)},
		bits: 32,
	}
}

func (n B32) AsUint32() uint32 {
	return uint32(n)
}

func (B32) Len() int {
	return 32
}

func (B32) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	panic("B32.Map")
}

func (B32) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	panic("B32.Reduce")
}

func (n *B32) Slice(beg, end int) ArrayLike {
	return n
}

func (n B32) Components() iter.Seq[Value] {
	return emptyIter
}

func (*B32) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (n *B32) Encode(bb BitBuf) {
	bb.Put32(0, uint32(*n))
}

func (n *B32) Decode(bb BitBuf, load LoadFunc) error {
	*n = B32(bb.Get32(0))
	return nil
}

func (n *B32) String() string {
	return fmt.Sprint(*n)
}

func B64Type() Type {
	return ArrayOf(BitType{}, 64)
}

type B64 uint64

func NewB64[T constraints.Integer](x T) *B64 {
	y := B64(x)
	return &y
}

func (n B64) isValue() {}

func (n B64) Type() Type {
	return B64Type()
}

func (B64) Elem() Type {
	return BitType{}
}

func (n B64) Get(i int) Value {
	return NewBit((n >> i) & 1)
}

func (n *B64) AsBitArray() BitArray {
	return BitArray{
		ws:   []uint64{uint64(*n)},
		bits: 64,
	}
}

func (n *B64) Len() int {
	return 64
}

func (n *B64) Slice(beg, end int) ArrayLike {
	return n
}

func (B64) Map(ty Type, fn func(Value) (Value, error)) (Mapable, error) {
	panic("B64.Map")
}

func (B64) Reduce(fn func(left, right Value) (Value, error)) (Value, error) {
	panic("B64.Reduce")
}

func (B64) Components() iter.Seq[Value] {
	return emptyIter
}

func (*B64) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (n *B64) Encode(bb BitBuf) {
	bb.Put64(0, uint64(*n))
}

func (n *B64) Decode(bb BitBuf, load LoadFunc) error {
	*n = B64(bb.Get64(0))
	return nil
}

func decodeBitArray(bb BitBuf) AsBitArray {
	// optimized integer sizes
	switch bb.Len() {
	case 8:
		return ptrTo(B8(bb.Get8(0)))
	case 16:
		return ptrTo(B16(bb.Get16(0)))
	case 32:
		return ptrTo(B32(bb.Get32(0)))
	case 64:
		return ptrTo(B64(bb.Get64(0)))
	default:
		var bs []Bit
		for i := 0; i < bb.Len(); i++ {
			bs = append(bs, Bit(bb.Get(int(i))))
		}
		return NewBitArray(bs...)
	}
}

func NewSize[T constraints.Integer](x T) *Size {
	return ptrTo(Size(x))
}

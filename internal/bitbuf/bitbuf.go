package bitbuf

import (
	"encoding/binary"
	"fmt"
)

type Bit = uint8

type Word = uint8

const WordBits = 8

type Buf struct {
	// offset is the offset in bits from the start of d
	offset int
	// l is the length of the buffer.  The end is offset + l
	l int
	d []Word
}

func New(l int) Buf {
	return Buf{
		l: l,
		d: make([]Word, divCeil(l, WordBits)),
	}
}

func FromBytes(d []byte) Buf {
	l := len(d) * 8
	if l > len(d)*WordBits {
		panic(fmt.Sprintf("bitbuf len=%d from %d bytes", l, len(d)))
	}
	return Buf{d: d, l: l}
}

func (b Buf) Len() int {
	return b.l
}

func (b Buf) Bytes() []byte {
	if b.offset != 0 {
		panic("Bytes can only be called on the original buffer")
	}
	return b.d
}

func (b Buf) Slice(beg, end int) Buf {
	if end-beg > b.Len() {
		panic(fmt.Sprintf("bitbuf: out of bounds slice. beg=%v end=%v end-beg=%v. len=%d", beg, end, end-beg, b.Len()))
	}
	return Buf{
		offset: b.offset + beg,
		l:      end - beg,
		d:      b.d,
	}
}

func (b Buf) Put(i int, x Bit) {
	i += b.offset
	putBit(b.d, i, x)
}

func (b Buf) Put8(i int, x uint8) {
	if i2, ok := b.isAligned(i); ok {
		b.d[i2] = x
	} else {
		// TODO: make this faster
		for j := int(0); j < 8; j++ {
			b.Put(i+j, Bit(x))
			x = x >> 1
		}
	}
}

func (b Buf) Put16(i int, x uint16) {
	if i2, ok := b.isAligned(i); ok {
		binary.LittleEndian.PutUint16(b.d[i2:], x)
	}
	// TODO: make this faster
	for j := int(0); j < 16; j++ {
		b.Put(i+j, Bit(x))
		x = x >> 1
	}
}

func (b Buf) Put32(i int, x uint32) {
	if i2, ok := b.isAligned(i); ok {
		binary.LittleEndian.PutUint32(b.d[i2:], x)
		return
	}
	// TODO: make this faster
	for j := int(0); j < 32; j++ {
		b.Put(i+j, Bit(x))
		x = x >> 1
	}
}

func (b Buf) Put64(i int, x uint64) {
	if i2, ok := b.isAligned(i); ok {
		binary.LittleEndian.PutUint64(b.d[i2:], x)
		return
	}
	// TODO: make this faster
	for j := int(0); j < 64; j++ {
		b.Put(i+j, Bit(x))
		x = x >> 1
	}
}

func (b Buf) PutBytes(i int, x []byte) {
	for j := range x {
		b.Put8(i+8*int(j), x[j])
	}
}

func (b Buf) Zero(beg, end int) {
	for ; end-beg >= 64; beg += 64 {
		b.Put64(beg, 0)
	}
	for ; end-beg >= 32; beg += 32 {
		b.Put32(beg, 0)
	}
	for ; end-beg >= 8; beg += 8 {
		b.Put8(beg, 0)
	}
	for ; beg < end; beg += 1 {
		b.Put(beg, 0)
	}
}

func (b Buf) Ones(beg, end int) {
	for ; end-beg >= 64; beg += 64 {
		b.Put64(beg, ^uint64(0))
	}
	for ; end-beg >= 32; beg += 32 {
		b.Put32(beg, ^uint32(0))
	}
	for ; end-beg >= 8; beg += 8 {
		b.Put8(beg, ^uint8(0))
	}
	for ; beg < end; beg += 1 {
		b.Put(beg, 1)
	}
}

func (b Buf) CheckZero(beg, end int) error {
	var acc uint64
	for ; end-beg >= 64; beg += 64 {
		acc |= b.Get64(beg)
	}
	for ; end-beg >= 32; beg += 32 {
		acc |= uint64(b.Get32(beg))
	}
	for ; end-beg >= 8; beg += 8 {
		acc |= uint64(b.Get8(beg))
	}
	for ; beg < end; beg += 1 {
		acc |= uint64(b.Get(beg))
	}
	if acc != 0 {
		return fmt.Errorf("buffer is not zeros")
	}
	return nil
}

func (b Buf) Get(i int) Bit {
	i += b.offset
	return getBit(b.d, i)
}

func (b Buf) Get8(i int) uint8 {
	var dst [1]byte
	b.GetBytes(i, i+8, dst[:])
	return dst[0]
}

func (b Buf) Get16(i int) uint16 {
	var dst [2]byte
	b.GetBytes(i, i+16, dst[:])
	return binary.LittleEndian.Uint16(dst[:])
}

func (b Buf) Get32(i int) uint32 {
	var dst [4]byte
	b.GetBytes(i, i+32, dst[:])
	return binary.LittleEndian.Uint32(dst[:])
}

func (b Buf) Get64(i int) uint64 {
	var dst [8]byte
	b.GetBytes(i, i+64, dst[:])
	return binary.LittleEndian.Uint64(dst[:])
}

func (b Buf) GetBytes(beg, end int, dst []byte) {
	beg += b.offset
	end += b.offset
	var j int
	for i := beg; i < end; i++ {
		putBit(dst, j, getBit(b.d, i))
		j++
	}
}

func (b Buf) isAligned(i int) (int, bool) {
	i += b.offset
	return i / WordBits, i%WordBits == 0
}

func putBit(d []Word, i int, x Bit) {
	x &= 1 // ensure only the low bit is set.
	byteIndex := i / WordBits
	bitPos := i % WordBits

	d[byteIndex] = (d[byteIndex] &^ (1 << bitPos)) | (Word(x) << bitPos)
}

func getBit(d []Word, i int) Bit {
	byteIndex := i / WordBits
	bitPos := i % WordBits
	return Bit(d[byteIndex]>>bitPos) & 1
}

func divCeil(a, b int) int {
	ret := a / b
	if a%b > 0 {
		ret++
	}
	return ret
}

package mvm1

import (
	"context"
	"encoding/binary"
	"fmt"
	"slices"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

const (
	Type2Bits = spec.KindBits
	BitBits   = spec.BitBits
	SizeBits  = spec.SizeBits
	RefBits   = spec.RefBits

	ListBits        = spec.ListBits
	LazyBits        = spec.LazyBits
	LambdaBits      = spec.LambdaBits
	FractalTypeBits = spec.FractalTypeBits
	PortBits        = spec.PortBits
	AnyTypeBits     = spec.AnyTypeBits
	AnyValueBits    = spec.AnyValueBits
)

// AnyType is capable of representing the Type of any Value
type AnyType [RefBits/WordBits + Type2Bits/WordBits]Word

func (at *AnyType) FromBytes(data []byte) {
	bytesToWords(data, at[:])
}

func (at *AnyType) AsBytes(dst []byte) {
	wordsToBytes(at[:], dst)
}

func (at AnyType) Size() int {
	return AnyTypeBits
}

func (at *AnyType) GetRef() Ref {
	return Ref(at[:8])
}

func (at *AnyType) SetRef(ref Ref) {
	copy(at[:8], ref[:])
}

func (at *AnyType) GetType() Type2 {
	return Type2{at[8]}
}

func (at *AnyType) SetType(t2 Type2) {
	at[8] = t2[0]
}

func (at *AnyType) SizeAtRef() int {
	return at.GetType().SizeOf()
}

func (at AnyType) String() string {
	return fmt.Sprintf("AnyType{%v %v}", at.GetType().TypeCode(), at.GetRef())
}

// AnyValue is capable of representing any Value
type AnyValue [2*RefBits/WordBits + Type2Bits/WordBits]Word

func (av *AnyValue) FromBytes(data []byte) {
	bytesToWords(data, av[:])
}

func (av *AnyValue) AsBytes() []byte {
	out := make([]byte, len(av)*4)
	wordsToBytes(av[:], out)
	return out
}

func (at *AnyValue) GetRef() Ref {
	return Ref(at[:8])
}

func (av *AnyValue) SetRef(ref Ref) {
	copy(av[:8], ref[:])
}

func (at *AnyValue) GetType() AnyType {
	return AnyType(at[8:])
}

func (av *AnyValue) SetType(at AnyType) {
	copy(av[8:], at[:])
}

// Ref refers to a Value in the store
type Ref [RefBits / WordBits]Word

func RefFromCID(cid cadata.ID) (r Ref) {
	r.FromBytes(cid[:])
	return r
}

func (r *Ref) FromBytes(bs []byte) {
	for i := range r {
		r[i] = binary.LittleEndian.Uint32(bs[i*4:])
	}
}

func (r Ref) CID() (ret cadata.ID) {
	for i, w := range r {
		binary.LittleEndian.PutUint32(ret[i*4:], w)
	}
	return ret
}

type List [RefBits/WordBits + SizeBits/WordBits]Word

func (l *List) GetRef() Ref {
	return Ref(l[:RefBits/WordBits])
}

func (l *List) SetRef(x Ref) {
	copy(l[:RefBits/WordBits], x[:])
}

func (l *List) GetLen() Word {
	return l[8]
}

func (l *List) SetLen(x uint32) {
	l[8] = x
}

type Expr [spec.ExprBits / WordBits]Word

func (e *Expr) GetRef() Ref {
	return Ref(e[0:8])
}

func (e *Expr) SetRef(x Ref) {
	copy(e[0:8], x[:])
}

func (e *Expr) GetProgType() ProgType {
	return ProgType{e[8]}
}

func (e *Expr) SetProgType(x ProgType) {
	e[8] = x[0]
}

type Lazy Expr

func (l *Lazy) GetRef() Ref {
	return (*Expr)(l).GetRef()
}

func (l *Lazy) SetRef(x Ref) {
	(*Expr)(l).SetRef(x)
}

func (l *Lazy) GetProgType() ProgType {
	return (*Expr)(l).GetProgType()
}

func (l *Lazy) SetProgType(x ProgType) {
	(*Expr)(l).SetProgType(x)
}

func (l *Lazy) Fingerprint(ly *LazyType) Fingerprint {
	t2 := newType2(spec.TC_Lazy, 0)
	return fingerprint(t2, ly[:], l[:], LazyBits)
}

type Lambda Expr

func (l *Lambda) GetRef() Ref {
	return (*Expr)(l).GetRef()
}

func (l *Lambda) SetRef(x Ref) {
	(*Expr)(l).SetRef(x)
}

func (l *Lambda) GetProgType() ProgType {
	return (*Expr)(l).GetProgType()
}

func (l *Lambda) SetProgType(x ProgType) {
	(*Expr)(l).SetProgType(x)
}

func (la *Lambda) Fingerprint(lty *LambdaType) Fingerprint {
	t2 := newType2(spec.TC_Lambda, 0)
	return fingerprint(t2, lty[:], la[:], LambdaBits)
}

type Fingerprint [256 / WordBits]Word

func (fp Fingerprint) String() string {
	var cid cadata.ID
	wordsToBytes(fp[:], cid[:])
	return cid.String()
}

func Hash(salt *Fingerprint, ws []Word, nbits int) Fingerprint {
	var salt2 *cadata.ID
	if salt != nil {
		salt2 = new(cadata.ID)
		wordsToBytes(salt[:], salt2[:])
	}
	data := make([]byte, len(ws)*WordBytes)
	wordsToBytes(ws, data)
	data = data[:divCeil(nbits, 8)]
	cid := mycelium.Hash(salt2, data)
	return Fingerprint(RefFromCID(cid))
}

func bytesToWords(bs []byte, ws []Word) {
	for len(bs)%WordBytes != 0 {
		bs = append(bs, 0)
	}
	for i := range ws {
		ws[i] = binary.LittleEndian.Uint32(bs[i*4:])
	}
}

func wordsToBytes(ws []Word, bs []byte) {
	for i, w := range ws {
		binary.LittleEndian.PutUint32(bs[i*4:], w)
	}
}

// fingerprint
// - all types are multiples of the Word size, so the size of types is not needed
// - valBits is the size of the value in bits, and valData may include up to 31 bits of zeros
func fingerprint(t2 Type2, tyData []Word, valData []Word, valBits int) Fingerprint {
	if t2.TypeCode() == spec.TC_Kind && tyData[0] == 0 && valData[0] == 0 {
		return Hash(nil, valData, valBits)
	}
	salt := fingerprint(newType2(spec.TC_Kind, 0), t2[:], tyData, len(tyData)*WordBits)
	return Hash(&salt, valData, valBits)
}

type dynValue struct {
	t2       Type2
	typeData []Word
	valData  []Word
	valBits  int
}

func (dv *dynValue) IsZero() bool {
	return dv.t2 == Type2{} &&
		len(dv.typeData) == 0 &&
		len(dv.valData) == 0 &&
		dv.valBits == 0
}

func (dv *dynValue) SetType2(x Type2) {
	dv.t2 = x
}

func (dv *dynValue) SetType(x []Word) {
	dv.typeData = append(dv.typeData[:0], x...)
}

func (dv *dynValue) SetValue(x []Word, nbits int) {
	dv.valData = append(dv.valData[:0], x...)
	dv.valBits = nbits
}

func (dv *dynValue) Set(x dynValue) {
	dv.SetType2(x.t2)
	dv.SetType(x.typeData)
	dv.SetValue(x.valData, x.valBits)
}

func (dv *dynValue) Clone() dynValue {
	return dynValue{
		t2:       dv.t2,
		typeData: slices.Clone(dv.typeData),
		valData:  slices.Clone(dv.valData),
		valBits:  dv.valBits,
	}
}

func (dv *dynValue) Lambda() Lambda {
	if dv.t2.TypeCode() != spec.TC_Lambda {
		panic("not a lambda")
	}
	return Lambda(dv.valData)
}

func (dv *dynValue) Lazy() Lazy {
	if dv.t2.TypeCode() != spec.TC_Lazy {
		panic("not a lambda")
	}
	return Lazy(dv.valData)
}

func (dv *dynValue) AsMycelium(ctx context.Context, s cadata.Getter) mycmem.Value {
	load := func(ref mycmem.Ref) (mycmem.Value, error) {
		return mycmem.Load(ctx, s, ref)
	}
	t2 := dv.t2.AsMycelium()
	tyBytes := makeBytes(dv.typeData)
	ty := t2.Zero().(mycmem.Type)
	if err := ty.Decode(bitbuf.FromBytes(tyBytes), load); err != nil {
		panic(err)
	}
	valBytes := makeBytes(dv.valData)
	val := ty.Zero()
	if err := val.Decode(bitbuf.FromBytes(valBytes), load); err != nil {
		panic(err)
	}
	return val
}

func makeBytes(ws []Word) []byte {
	ret := make([]byte, len(ws)*4)
	wordsToBytes(ws, ret)
	return ret
}

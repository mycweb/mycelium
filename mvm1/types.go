package mvm1

import (
	"context"
	"encoding/binary"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

// Type2 is capable of representing the Type of any Type
type Type2 [1]Word

func newType2(kc spec.TypeCode, data uint32) Type2 {
	// only sums and products should be using the data field
	switch kc {
	case spec.TC_Sum, spec.TC_Product:
	default:
		data = 0
	}
	return Type2{(data << 8) | uint32(kc)}
}

func (Type2) Size() int {
	return spec.KindBits
}

func (at2 *Type2) FromBytes(data []byte) {
	*at2 = Type2{binary.LittleEndian.Uint32(data)}
}

func (at2 Type2) AsBytes(dst []byte) {
	binary.LittleEndian.PutUint32(dst, at2[0])
}

func (at2 Type2) TypeCode() spec.TypeCode {
	return spec.TypeCode(at2[0])
}

func (at2 Type2) Data() uint32 {
	return uint32(at2[0] >> 8)
}

func (t2 Type2) PostAnyType(ctx context.Context, s cadata.Poster) (ret AnyType, _ error) {
	// typeOf(Type2) is always Type2 so we use the zero salt
	ref, err := postWords(ctx, s, nil, t2.Size(), t2[:])
	if err != nil {
		return AnyType{}, err
	}
	ret.SetRef(ref)
	ret[8] = uint32(spec.TC_Kind)
	return ret, nil
}

// SizeOf returns the SizeOf a value of this type
func (at2 Type2) SizeOf() int {
	switch at2.TypeCode() {
	case spec.TC_Kind:
		return spec.KindBits
	case spec.TC_Bit:
		return spec.BitTypeBits
	case spec.TC_Array:
		return spec.ArrayTypeBits
	case spec.TC_List:
		return spec.ListTypeBits
	case spec.TC_Ref:
		return spec.RefTypeBits
	case spec.TC_Sum:
		return spec.SumTypeBits(int(at2.Data()))
	case spec.TC_Product:
		return spec.ProductTypeBits(int(at2.Data()))
	case spec.TC_Prog:
		return spec.ProgTypeBits
	case spec.TC_Lazy:
		return spec.LazyTypeBits
	case spec.TC_Lambda:
		return spec.LambdaTypeBits
	case spec.TC_Fractal:
		return spec.FractalTypeBits
	case spec.TC_Port:
		return spec.PortTypeBits
	case spec.TC_Distinct:
		return spec.DistinctTypeBits
	case spec.TC_AnyType:
		return spec.AnyTypeTypeBits
	case spec.TC_AnyValue:
		return spec.AnyValueTypeBits
	default:
		panic(at2.TypeCode())
	}
}

func (t2 Type2) Salt() Ref {
	var buf [4]byte
	t2.AsBytes(buf[:])
	return RefFromCID(mycelium.Hash(nil, buf[:]))
}

// NeedsSalt returns true if a value of this type needs to be posted with a salt
func (t2 Type2) NeedsSalt() bool {
	switch t2.TypeCode() {
	case spec.TC_Kind, spec.TC_Bit, spec.TC_AnyType, spec.TC_AnyValue, spec.TC_Prog:
		return false
	case spec.TC_Sum, spec.TC_Product:
		return t2.Data() > 0
	default:
		return true
	}
}

// Fingerprint returns the fingerprint of the t2
func (t2 Type2) Fingerprint() Fingerprint {
	return fingerprint(newType2(spec.TC_Kind, 0), []Word{0}, t2[:], spec.KindBits)
}

func (t2 Type2) AsMycelium() mycmem.Kind {
	var buf [4]byte
	t2.AsBytes(buf[:])
	var ret mycmem.Kind
	if err := ret.Decode(bitbuf.FromBytes(buf[:]).Slice(0, 32), nil); err != nil {
		panic(err)
	}
	return ret
}

type ArrayType [AnyTypeBits/WordBits + SizeBits/WordBits]Word

func (at *ArrayType) FromBytes(data []byte) {
	bytesToWords(data, at[:])
}

func (at *ArrayType) AsBytes(bs []byte) {
	wordsToBytes(at[:], bs)
}

func (at *ArrayType) Size() int {
	return spec.ArrayTypeBits
}

func (at *ArrayType) Elem() AnyType {
	return AnyType(at[:AnyTypeBits/WordBits])
}

func (at *ArrayType) Len() int {
	return int(at[AnyTypeBits/WordBits])
}

type ProgType [1]Word

func newProgType(nbits int) ProgType {
	return ProgType{Word(divCeil(nbits, 8))}
}

func (et ProgType) Size() int {
	return spec.ProgTypeBits
}

func (et ProgType) SizeOf() int {
	return int(et[0] * 8)
}

func (et *ProgType) FromBytes(data []byte) {
	*et = ProgType{binary.LittleEndian.Uint32(data)}
}

func (et ProgType) AsBytes(dst []byte) {
	binary.LittleEndian.PutUint32(dst, et[0])
}

type RefType [AnyTypeBits / WordBits]Word

func (rt RefType) Elem() AnyType {
	return AnyType(rt)
}

type SumType []Word

func makeSumType(n int) ProductType {
	return make(ProductType, n*AnyTypeBits/WordBits)
}

func (t *SumType) FromBytes(data []byte) {
	bytesToWords(data, (*t)[:])
}

func (t *SumType) Size() int {
	return t.Len() * AnyTypeBits
}

func (t *SumType) At(i int) AnyType {
	beg := i * (AnyTypeBits / WordBits)
	end := beg + (AnyTypeBits / WordBits)
	return AnyType((*t)[beg:end])
}

func (st *SumType) Len() int {
	return len(*st) * WordBits / AnyTypeBits
}

type ProductType []Word

func makeProductType(n int) ProductType {
	return make(ProductType, n*AnyTypeBits/WordBits)
}

func (t *ProductType) Size() int {
	return t.Len() * AnyTypeBits
}

func (t *ProductType) FromBytes(data []byte) {
	bytesToWords(data, (*t)[:])
}

func (t *ProductType) AsBytes(data []byte) {
	wordsToBytes((*t)[:], data)
}

func (st ProductType) At(i int) AnyType {
	beg := i * (AnyTypeBits / WordBits)
	end := beg + (AnyTypeBits / WordBits)
	return AnyType(st[beg:end])
}

func (st ProductType) Len() int {
	return len(st) * WordBits / AnyTypeBits
}

type ListType [AnyTypeBits / WordBits]Word

func (lt *ListType) Elem() AnyType {
	return AnyType(*lt)
}

type LazyType [AnyTypeBits / WordBits]Word

func (lt *LazyType) Elem() AnyType {
	return AnyType(*lt)
}

type LambdaType [2 * AnyTypeBits / WordBits]Word

func (lt *LambdaType) GetIn() AnyType {
	return AnyType(lt[:AnyTypeBits/WordBits])
}

func (lt *LambdaType) GetOut() AnyType {
	return AnyType(lt[AnyTypeBits/WordBits:])
}

type FractalType Expr

func (ft *FractalType) GetRef() Ref {
	return (*Expr)(ft).GetRef()
}

func (ft *FractalType) SetRef(x Ref) {
	(*Expr)(ft).SetRef(x)
}

func (ft *FractalType) GetExprType() ProgType {
	return (*Expr)(ft).GetProgType()
}

func (ft *FractalType) SetExprType(x ProgType) {
	(*Expr)(ft).SetProgType(x)
}

type PortType [4 * AnyTypeBits / WordBits]Word

type DistinctType [AnyValueBits/WordBits + AnyTypeBits/WordBits]Word

func (dt *DistinctType) AsBytes(dst []byte) {
	wordsToBytes(dt[:], dst)
}

func (dt *DistinctType) FromBytes(x []byte) {
	bytesToWords(x, dt[:])
}

func (dt *DistinctType) Size() int {
	return len(dt) * WordBits
}

func (dt *DistinctType) Base() (ret AnyType) {
	return AnyType(dt[:len(ret)])
}

func (dt *DistinctType) Mark() AnyValue {
	return AnyValue(dt[AnyTypeBits/WordBits:])
}

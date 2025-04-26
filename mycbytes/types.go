package mycbytes

import (
	"encoding/binary"

	"myceliumweb.org/mycelium/spec"
)

type Kind [4]byte

type BitType struct{}

type ArrayType [spec.ArrayTypeBits / 8]byte

func (at *ArrayType) Elem() AnyType {
	return AnyType(at[0 : spec.AnyTypeBits/8])
}

type ProgType [spec.ProgTypeBits / 8]byte

func (pt *ProgType) Len() int {
	return int(binary.LittleEndian.Uint32(pt[:]))
}

type RefType [spec.RefTypeBits / 8]byte

func (rt *RefType) Elem() AnyType {
	return AnyType(rt[0 : spec.AnyTypeBits/8])
}

type SumType []byte

func (st SumType) Field(i int) AnyType {
	return AnyType(st[i*spec.AnyTypeBits/8:])
}

func (st SumType) Len() int {
	return len(st) * 8 / spec.AnyTypeBits
}

type ProductType []byte

func (pt ProductType) Field(i int) AnyType {
	return AnyType(pt[i*spec.AnyTypeBits/8:])
}

func (pt ProductType) Len() int {
	return len(pt) * 8 / spec.AnyTypeBits
}

type ListType [spec.ListTypeBits / 8]byte

func (lt *ListType) Elem() AnyType {
	return AnyType(lt[0 : spec.AnyTypeBits/8])
}

type LazyType [spec.LazyTypeBits / 8]byte

func (lt *LazyType) Elem() AnyType {
	return AnyType(lt[0 : spec.AnyTypeBits/8])
}

type LambdaType [spec.LambdaTypeBits / 8]byte

func (lt *LambdaType) In() AnyType {
	return AnyType(lt[0 : spec.AnyTypeBits/8])
}

func (lt *LambdaType) Out() AnyType {
	return AnyType(lt[spec.AnyTypeBits/8:])
}

type FractalType [spec.FractalTypeBits / 8]byte

func (ft FractalType) Expr() Expr {
	return Expr(ft)
}

type ExprType [spec.ExprTypeBits / 8]byte

type AnyTypeType [spec.AnyTypeTypeBits / 8]byte

type AnyValueType [spec.AnyValueTypeBits / 8]byte

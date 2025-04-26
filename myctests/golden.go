package myctests

import (
	"encoding/hex"
	"strings"

	"lukechampine.com/blake3"
)

// BP is a bit pattern
type BP struct {
	Data []byte
	Len  int
}

var (
	BP_KindKind      = fromHex("00_00_00_00", 32)
	BP_BitKind       = fromHex("01_00_00_00", 32)
	BP_ArrayKind     = fromHex("02_00_00_00", 32)
	BP_ProgKind      = fromHex("03_00_00_00", 32)
	BP_RefKind       = fromHex("04_00_00_00", 32)
	BP_SumKind_0     = fromHex("05_00_00_00", 32)
	BP_ProductKind_0 = fromHex("06_00_00_00", 32)
	BP_ListKind      = fromHex("07_00_00_00", 32)
	BP_LazyKind      = fromHex("08_00_00_00", 32)
	BP_LambdaKind    = fromHex("09_00_00_00", 32)
	BP_FractalKind   = fromHex("0a_00_00_00", 32)
	BP_PortKind      = fromHex("0b_00_00_00", 32)
	BP_DistinctKind  = fromHex("0c_00_00_00", 32)
	BP_ExprKind      = fromHex("0d_00_00_00", 32)
	BP_AnyTypeKind   = fromHex("0e_00_00_00", 32)
	BP_AnyValueKind  = fromHex("0f_00_00_00", 32)
)

var (
	BP_Bit_0 = BP{Len: 1, Data: []byte{0}}
	BP_Bit_1 = BP{Len: 1, Data: []byte{1}}
)

var (
	BP_Ref_KindKind      = hash(noSalt, BP_KindKind)
	BP_Ref_BitKind       = hash(noSalt, BP_BitKind)
	BP_Ref_ArrayKind     = hash(noSalt, BP_ArrayKind)
	BP_Ref_ProgKind      = hash(noSalt, BP_ProgKind)
	BP_Ref_RefKind       = hash(noSalt, BP_RefKind)
	BP_Ref_SumKind_0     = hash(noSalt, BP_SumKind_0)
	BP_Ref_ProductKind_0 = hash(noSalt, BP_ProductKind_0)
	BP_Ref_ListKind      = hash(noSalt, BP_ListKind)
	BP_Ref_LazyKind      = hash(noSalt, BP_LazyKind)
	BP_Ref_LambdaKind    = hash(noSalt, BP_LambdaKind)
	BP_Ref_FractalKind   = hash(noSalt, BP_FractalKind)
	BP_Ref_PortKind      = hash(noSalt, BP_PortKind)
	BP_Ref_DistinctKind  = hash(noSalt, BP_DistinctKind)
	BP_Ref_AnyTypeKind   = hash(noSalt, BP_AnyTypeKind)
	BP_Ref_AnyValueKind  = hash(noSalt, BP_AnyValueKind)
)

var (
	BP_BitType       = zero()
	BP_ProgType_0    = bytes(0, 0, 0, 0)
	BP_SumType_0     = zero()
	BP_ProductType_0 = zero()
	BP_AnyTypeType   = zero()
	BP_AnyValueType  = zero()
)

var (
	BP_Ref_BitType       = hash(noSalt, BP_BitType)
	BP_Ref_ProgType_0    = hash(noSalt, BP_ProgType_0)
	BP_Ref_SumType_0     = hash(noSalt, BP_SumType_0)
	BP_Ref_ProductType_0 = hash(noSalt, BP_ProductType_0)
	BP_Ref_AnyTypeType   = hash(noSalt, BP_AnyTypeType)
	BP_Ref_AnyValueType  = hash(noSalt, BP_AnyValueType)
)

var (
	BP_AnyType_KindKind      = concat(BP_Ref_KindKind, BP_KindKind)
	BP_AnyType_BitType       = concat(BP_Ref_BitType, BP_BitKind)
	BP_AnyType_ProgType_0    = concat(BP_Ref_ProgType_0, BP_ProgKind)
	BP_AnyType_SumType_0     = concat(BP_Ref_SumType_0, BP_SumKind_0)
	BP_AnyType_ProductType_0 = concat(BP_Ref_ProductType_0, BP_ProductKind_0)
	BP_AnyType_AnyTypeType   = concat(BP_Ref_AnyTypeType, BP_AnyTypeKind)
	BP_AnyType_AnyValueType  = concat(BP_Ref_AnyTypeType, BP_AnyValueKind)
)

var (
	BP_ArrayType_Bit_0          = concat(BP_AnyType_BitType, bytes(0, 0, 0, 0))
	BP_ArrayType_Bit_1          = concat(BP_AnyType_BitType, bytes(1, 0, 0, 0))
	BP_RefType_Bit              = BP_AnyType_BitType
	BP_SumType_Bit              = BP_AnyType_BitType
	BP_ProductType_Bit          = BP_AnyType_BitType
	BP_ListType_Bit             = BP_AnyType_BitType
	BP_LazyType_Bit             = BP_AnyType_BitType
	BP_LambdaType_Bit_Bit       = concat(BP_AnyType_BitType, BP_AnyType_BitType)
	BP_PortType_Bit_Bit_Bit_Bit = concat(BP_AnyType_BitType, BP_AnyType_BitType, BP_AnyType_BitType, BP_AnyType_BitType)
)

var (
	BP_Ref_Bit_0 = hash(noSalt, BP_Bit_0)
	BP_Ref_Bit_1 = hash(noSalt, BP_Bit_1)

	refBitSalt       = hash(BP_Ref_RefKind, BP_RefType_Bit)
	BP_Ref_Ref_Bit_0 = hash(refBitSalt, BP_Ref_Bit_0)
	BP_Ref_Ref_Bit_1 = hash(refBitSalt, BP_Ref_Bit_1)
)

var (
	BP_AnyValue_Bit_0 = concat(BP_Ref_Bit_0, BP_AnyType_BitType)
	BP_AnyValue_Bit_1 = concat(BP_Ref_Bit_1, BP_AnyType_BitType)
)

func zero() BP {
	return BP{Data: []byte{}}
}

func fromHex(x string, l int) BP {
	x = strings.Replace(x, "_", "", -1)
	data, err := hex.DecodeString(x)
	if err != nil {
		panic(err)
	}
	return BP{Data: data, Len: l}
}

func bytes(x ...byte) BP {
	return BP{Data: x, Len: len(x) * 8}
}

func hash(salt BP, data BP) BP {
	if salt.Data != nil && len(salt.Data) != 32 {
		panic(salt.Data)
	}
	h := blake3.New(32, salt.Data)
	h.Write(data.Data)
	return BP{
		Data: h.Sum(nil),
		Len:  256,
	}
}

func concat(xs ...BP) BP {
	var data []byte
	var n int
	for _, x := range xs {
		if x.Len%8 != 0 {
			panic(x) // TODO
		}
		data = append(data, x.Data...)
		n += x.Len
	}
	return BP{Data: data, Len: n}
}

var noSalt = BP{}

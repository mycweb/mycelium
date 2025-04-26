package spec

import "math/bits"

const (
	// KindBits is the size of a Kind in bits.
	KindBits = 32
	// BitBits is the size of a Bit in bits (it's 1).
	BitBits = 1
	// SizeBits is the size of a Size in bits
	SizeBits = 32
	// RefBits is the size of a ref in bits
	RefBits = 256

	// ListBits is the size of a List in bits
	ListBits = RefBits + SizeBits
	// Lazy is the size of a Lazy in bits
	LazyBits = RefBits + SizeBits
	// Lambda is the size of a Lambda in bits.
	LambdaBits = RefBits + SizeBits
	// PortBits is the size of a Port in bits
	PortBits = 256
	// ExprBits is the size of an Expr in bits
	ExprBits = RefBits + SizeBits
	// AnyTypeBits is the size of an AnyType in bits
	AnyTypeBits = RefBits + KindBits
	// AnyValue is the size of an AnyValue in bits
	AnyValueBits = RefBits + AnyTypeBits
)

const (
	// BitTypeBits is the size of a BitType in bits.
	BitTypeBits = 0

	// ArrayTypeBits is the size of an ArrayType in bits.
	// ArrayTypes encode the element type and the length of the array
	ArrayTypeBits = AnyTypeBits + SizeBits

	// ProgTypeBit is the size of an ProgType in bits
	// ProgTypes encode the number of bytes in the Program
	ProgTypeBits = SizeBits

	// RefTypeBits is the size of a RefType
	// RefTypes encode the element type.
	RefTypeBits = AnyTypeBits

	// ListTypeBits is the size of a ListType.
	// ListTypes encode the element type.
	ListTypeBits = AnyTypeBits

	// LazyTypeBits is the size of a LazyType in bits.
	LazyTypeBits = AnyTypeBits

	// LambdaTypeBits is the size of a LambdaType in bits.
	LambdaTypeBits = AnyTypeBits * 2

	// FractalTypeBits is size of a FractalType in bits.
	FractalTypeBits = RefBits + SizeBits

	// PortTypeBits is the size of a PortType in bits.
	PortTypeBits = AnyTypeBits * 4

	// DistinctTypeBits is the size of a DistinctType in bits.
	DistinctTypeBits = AnyTypeBits + AnyValueBits

	// ExprTypeBits is the size of an ExprType in bits.
	ExprTypeBits = 0

	// AnyTypeTypeBits is the size of an AnyTypeType in bits
	AnyTypeTypeBits = 0

	// AnyValueType is the size of an AnyValueType in bits
	AnyValueTypeBits = 0
)

// SumTypeBits returns the size of a SumType with arity n in bits
func SumTypeBits(n int) int {
	return AnyTypeBits * n
}

// ProductTypeBits returns the size of a ProductType with arity n in bits
func ProductTypeBits(n int) int {
	return AnyTypeBits * n
}

// SumTagBits returns the number of bits needed for the tag of a SumType with n fields
func SumTagBits(n int) int {
	if n <= 1 {
		return 0
	}
	return bits.Len(uint(n))
}

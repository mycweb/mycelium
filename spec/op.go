// package spec contains primitive operations
package spec

// OpBits is the number of bits needed to encode a Op
const OpBits = 8

// Op is a primitive operation
//
//go:generate go run golang.org/x/tools/cmd/stringer -type=Op
type Op uint8

const (
	Unknown Op = iota

	// Pass (val: T) => val
	Pass
	// Equal: (a: T, b: T) -> Bit
	Equal

	// Craft creates new values of a specified type
	// Craft(T: Type, x Product[...]) T
	Craft
	// Uncraft returns a Product representation of a Value.
	Uncraft
	// TypeOf: (x: 'Any) RealType
	TypeOf
	// SizeOf: (Type) -> Size
	SizeOf
	// MaxSize: () -> Size
	MaxSize
	// Fingerprint: (x) -> Array[Bit, 256]
	Fingerprint
	// Root: () -> Type2
	Root
	// Encode: (x: T) -> Array[Bit, SizeOf(T)]
	Encode
	// Decode: (T: Type, data: Array[Bit, SizeOf(T)]) -> T
	Decode
)

const (
	// ArrayEmpty :: (Type) -> Array
	ArrayEmpty Op = 1*section + iota
	// ArrayUnit :: (elemType, elem) -> Array[elemType]
	ArrayUnit
	// ProductEmpty
	ProductEmpty
	// ProductUnit: (x AnyValue) => (x)
	ProductUnit
	// Concat merges two composite values
	// Concat: (left Array[T, ll], right Array[T, lr]) -> Array[T, ll + lr]
	// Concat: (left Product[L0, L1... LN], right Product[R0, R1, ... RN]) -> Product[L0, L1, ... LN, R0, R1, ... RN]
	Concat
	// MakeSum (x SumType, tag BitArray, v AnyValue) =>
	MakeSum

	// Which returns the Sum field which can be read from.
	// Only one field of a Sum is valid to read at a time.
	Which
	// Field returns a field of a Sum or Product
	Field
	// Slot returns a value in an Array or List
	Slot
	// Section returns a contiguous range of an Array.
	// Section (x: Array[T, l], beg: Size, end: Size) -> Array[T, end - beg]
	// beg and end must be known at compile time
	Section

	// Gather: (x Array[List[T], _]) -> List[T]
	Gather
	// Slice (x List[T], beg: Size, end: Size) -> List[T]
	// Slice returns a List containing the elements in x from beg (inclusive) to end (exclusive)
	// If beg or end are out of bounds, Slice will panic.
	// If end is < beg, slice will panic.
	Slice
)

const (
	// Let: (var Expr body Expr)
	Let Op = 2*section + iota
	// Lazy: (body: 'T) -> Lazy[T]
	Lazy
	// Lambda creates a new lambda
	// Lambda (in: Type, out: Type, body: 'out) -> Lambda[in, out]
	Lambda
	// Fractal: (x: 'Type) -> FractalKind
	Fractal
	// Eval evaluates a Lazy value
	Eval
	// Apply applies a Lambda to an input
	// Apply: (la, input) => output
	Apply

	// Mux: (table: Array[T, SizeOf(I)], pick: I) -> T
	Mux
	// Branch: (test: Bit, branch0: 'Expr, branch1: 'Expr) ->
	Branch
	// Try: (expr 'Expr) -> Sum[Fault, TypeOf(expr)]
	Try
	// Panic (val: AnyValue) -> Bottom
	Panic
	// Self returns the current Lambda or Lazy as a value
	Self
)

const (
	// Post: (a Value) => Ref[ty]
	Post Op = 3*section + iota
	// Load: (x Ref[T]) => T
	Load
)

const (
	// Input: (p: Port[_, T, _, _]) -> T
	Input Op = 4*section + iota
	// Output: (p: Port[T, _, _, _], x: T) -> ()
	Output
	// Interact: (p: Port[_, _, Req, Resp], req: Req) -> Resp
	Interact
)

const (
	// ListFrom
	// DEPRECATED
	ListFrom Op = 5*section + iota
	// ListTo
	// DEPRECATED
	ListTo

	// AnyTypeFrom: (x: Type) -> AnyType
	// DEPRECATED
	AnyTypeFrom
	// AnyTypeTo: (x: Kind) -> x
	// DEPRECATED
	AnyTypeTo
	// AnyTypeElemType: (x: AnyType) -> Kind
	// DEPRECATED
	AnyTypeElemType

	// AnyValueFrom: (x: _) -> AnyValue
	// DEPRECATED
	AnyValueFrom
	// AnyValueTo: (x: AnyValue, ty: Type) -> Type
	// DEPRECATED
	AnyValueTo
	// AnyValueElemType: (x: AnyValue) -> AnyType
	// DEPRECATED
	AnyValueElemType
)

const (
	Param0 Op = 7 * section
	ParamN Op = 127
)

func (op Op) IsParam() bool {
	return Param0 <= op && op <= ParamN
}

const section = 1 << 4

const (
	// ZERO evalutes to the Bit 0
	ZERO Op = 128 + iota
	// ONE evalutes to the Bit 1
	ONE

	// LiteralKind encodes a kind inline in 32 bits.
	LiteralKind
	// LiteralArrayType encodes an ArrayType inline.
	LiteralArrayType
	// LiteralRefType encodes a RefType inline.
	LiteralRefType
	// LiteralListType encodes a ListType inline.
	LiteralListType
	// LiteralLazyType encodes a LazyType inline.
	LiteralLazyType
	// LiteralLambdaType encodes a LambdaType inline.
	LiteralLambdaType
	// LiteralPortType encodes a PortType inline.
	LiteralPortType
	// LiteralAnyType encodes an AnyType inline.
	LiteralAnyType
	// LiteralAnyValue encodes an AnyValue inline.
	LiteralAnyValue
)

const (
	LiteralB8   Op = 192 + 1 - 1
	LiteralB16  Op = 192 + 2 - 1
	LiteralB32  Op = 192 + 4 - 1
	LiteralB64  Op = 192 + 8 - 1
	LiteralB128 Op = 192 + 16 - 1
	LiteralB256 Op = 192 + 32 - 1
)

const (
	LiteralB0 Op = 224 + iota
	_

	LiteralB1_0
	LiteralB1_1

	LiteralB2_00
	LiteralB2_01
	LiteralB2_10
	LiteralB2_11

	LiteralB3_000
	LiteralB3_001
	LiteralB3_010
	LiteralB3_011
	LiteralB3_100
	LiteralB3_101
	LiteralB3_110
	LiteralB3_111

	LiteralB4_0000
	LiteralB4_0001
	LiteralB4_0010
	LiteralB4_0011
	LiteralB4_0100
	LiteralB4_0101
	LiteralB4_0110
	LiteralB4_0111
	LiteralB4_1000
	LiteralB4_1001
	LiteralB4_1010
	LiteralB4_1011
	LiteralB4_1100
	LiteralB4_1101
	LiteralB4_1110
	LiteralB4_1111
)

func LiteralB1(x int) Op {
	return LiteralB1_0 + Op(x&1)
}

func LiteralB2(x int) Op {
	return LiteralB2_00 + Op(x&3)
}

func LiteralB3(x int) Op {
	return LiteralB3_000 + Op(x&7)
}

func LiteralB4(x int) Op {
	return LiteralB4_0000 + Op(x&15)
}

// LiteralBytes returns the opcode for encoding n bytes of data as an Array of Bits
func LiteralBytes(n int) Op {
	if n > 32 || n < 1 {
		panic(n)
	}
	return LiteralB8 + Op(n) - 1
}

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

	// Pass (val: AnyVaue) => val
	Pass
	// Equal: (a: Any, b: Any) -> Bit
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
	// Zero => 0
	ZERO Op = 1*section + iota
	// NOR: (Bit Bit) -> Bit
	_
	// NOTPANDQ: (Bit Bit) -> Bit
	_
	// NOT: (Bit) -> Bit
	_
	// PANDNOTQ: (Bit Bit) -> Bit
	_
	// NOTQ
	_
	// XOR: (Bit Bit) -> Bit
	_
	// NAND: (Bit Bit) -> Bit
	_
	// AND: (Bit Bit) -> Bit
	_
	// XNOR: (Bit Bit) -> Bit
	_
	// Q
	_
	// If/Then
	_
	// P
	_
	// Then/If
	_
	// OR: (Bit Bit) -> Bit
	_
	// One => 1
	ONE
)

const (
	// ArrayEmpty :: (Type) -> Array
	ArrayEmpty Op = 2*section + iota
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

	// Len returns the length of a value.
	// All calls to Get must have an index < the value returned by Len.
	Len
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
)

const (
	// Let: (var Expr body Expr)
	Let Op = 3*section + iota
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
)

const (
	// Map: (elemIn: Type, x Array[ty, _], fn (elemIn) -> elemOut) Array[elemOut, _]
	Map Op = 4*section + iota
	// Reduce: (elem: Type, x Array[elem, _], fn (left, right elem) -> elem ) elem
	Reduce
)

const (
	// Post: (a Value) => Ref[ty]
	Post Op = 5*section + iota
	// Load: (x Ref[T]) => T
	Load
)

const (
	// Input: (p: Port[_, T, _, _]) -> T
	Input Op = 6*section + iota
	// Output: (p: Port[T, _, _, _], x: T) -> ()
	Output
	// Interact: (p: Port[_, _, Req, Resp], req: Req) -> Resp
	Interact
)

const (
	// Gather: (x Array[List[T], _]) -> List[T]
	Gather Op = 7*section + iota
	// Slice (x List[T], beg: Size, end: Size) -> List[T]
	// Slice returns a List containing the elements in x from beg (inclusive) to end (exclusive)
	// If beg or end are out of bounds, Slice will panic.
	// If end is < beg, slice will panic.
	Slice

	// ListFrom
	// DEPRECATED
	ListFrom
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

func (c Op) IsLang() bool {
	return c >= Self
}

const (
	Self Op = 128 + iota

	LiteralKind
	LiteralAnyType
	LiteralAnyValue
)

const (
	LiteralB0 Op = 128 + 8 + iota
	LiteralB2
	LiteralB4
	LiteralB8
	LiteralB16
	LiteralB32
	LiteralB64
	LiteralB128
	LiteralB256
)

const (
	Param0 Op = 0xc0
	ParamN Op = 0xff
)

func (c Op) IsParam() bool {
	return Param0 <= c && c <= ParamN
}

const section = 1 << 4

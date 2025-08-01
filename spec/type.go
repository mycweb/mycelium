package spec

// KindCode is used in the encoding of Kinds
//
//go:generate go run golang.org/x/tools/cmd/stringer -type=TypeCode
type TypeCode uint8

const (
	TC_Kind = TypeCode(iota)
	TC_Bit
	TC_Array
	TC_Prog
	TC_Ref
	TC_Sum
	TC_Product
	TC_List
	TC_Lazy
	TC_Lambda
	TC_Fractal
	TC_Port
	TC_Distinct
	TC_AnyProg
	TC_AnyType
	TC_AnyValue
)

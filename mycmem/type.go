package mycmem

import (
	"reflect"
)

// Type is the common interface implemented by all types.
// A Type describes a set of values
type Type interface {
	Value
	// SizeOf returns the size of Values of this Type
	SizeOf() int
	Zero() Value
	isType()
}

type Supersetter interface {
	Supersets(Type) bool
}

// Supersets returns true iff the set of values included in a is a superset
// of those included in b.
func Supersets(a, b Type) bool {
	if reflect.DeepEqual(b, Bottom()) {
		return true // All types superset bottom
	}
	if ss, ok := a.(Supersetter); ok {
		return ss.Supersets(b)
	}
	// Types Superset themselves
	// TODO: use Equal
	return reflect.DeepEqual(a, b)
}

// TypeContains returns true if ty contains v
func TypeContains(ty Type, v Value) bool {
	return Supersets(ty, v.Type())
}

// IsType returns true iff x is a Type
func IsType(x Value) bool {
	_, ok := x.(Type)
	return ok
}

// Bottom is the empty type
func Bottom() Type {
	return SumType{}
}

// forAll return true if fn(x) is true for all x
func forAll[E any, S ~[]E](xs S, fn func(E) bool) bool {
	for i := range xs {
		if !fn(xs[i]) {
			return false
		}
	}
	return true
}

// forAny returns true if fn(x) is true for any x
func forAny[E any, S ~[]E](xs S, fn func(E) bool) bool {
	for i := range xs {
		if fn(xs[i]) {
			return true
		}
	}
	return false
}

func TypeFromGo(x reflect.Type) Type {
	switch x.Kind() {
	case reflect.Bool:
		return BitType{}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return ArrayOf(BitType{}, int(x.Size()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return ArrayOf(BitType{}, int(x.Size()))
	case reflect.Array:
		return ArrayOf(TypeFromGo(x.Elem()), x.Len())
	case reflect.Slice:
		return ListOf(TypeFromGo(x.Elem()))
	case reflect.Pointer:
		return NewRefType(TypeFromGo(x.Elem()))

	case reflect.Struct:
		var fields []Type
		for i := 0; i < x.NumField(); i++ {
			f := x.Field(i)
			ty := TypeFromGo(f.Type)
			fields = append(fields, ty)
		}
		return ProductType(fields)
	default:
		panic(x)
	}
}

// pakage mycmem implements Mycelium Values entirely in memory.
// It is useful for quickly defining Mycelium Values, or manipulating Mycelium Values
// when the Values can be trusted to be small.
package mycmem

import (
	"fmt"
	"strings"
)

type Size = B32

func SizeType() Type {
	return B32Type()
}

func Pretty(x Value) string {
	sb := new(strings.Builder)
	pretty(sb, 0, x)
	return sb.String()
}

func pretty(sb *strings.Builder, level uint, x Value) {
	switch x := x.(type) {
	default:
		fmt.Fprintf(sb, "%v\n", x)
	}
}

// Composite provides Get and Len
type Composite interface {
	// Get returns a component Value at an index within the Compound
	Get(idx int) Value
	// Len returns the number of components this Compound
	Len() int
	Value
}

type Mapable interface {
	Map(outTy Type, fn func(Value) (Value, error)) (Mapable, error)
	Value
}

type Reducible interface {
	Reduce(fn func(left, right Value) (Value, error)) (Value, error)
	Value
}

type Maker interface {
	Make(Value) Value
	Type
}

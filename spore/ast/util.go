package ast

import (
	"reflect"
)

func MapNode(x Node, fn func(Node) (Node, error)) (Node, error) {
	switch x := x.(type) {
	case SExpr:
		y := make(SExpr, len(x))
		for i := range x {
			yi, err := MapNode(x[i], fn)
			if err != nil {
				return nil, err
			}
			y[i] = yi
		}
		return y, nil
	default:
		return fn(x)
	}
}

func Equal(a, b Node) bool {
	return reflect.DeepEqual(a, b)
}

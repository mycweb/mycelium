// package myctype implements type checking for Mycelium programs
package myctype

import (
	"myceliumweb.org/mycelium/spec"
)

type Params[Term any] struct {
	BitType      Term
	AnyValueType Term
	// TypeOf should return the a type Expr for a given value
	TypeOf func(val Term) Term
	Split  func(Term) (spec.Op, [3]Term)
	SizeOf func(ty Term) int

	MakeType2 func(code spec.TypeCode) Term
	RefOf     func(Term) Term
	// SumOf returns a sum type for a list of types
	SumOf     func([]Term) Term
	ProductOf func([]Term) Term
	ArrayOf   func(Term, int) Term
	LazyOf    func(Term) Term
	LambdaOf  func(in, out Term) Term
	ListOf    func(Term) Term
	TypeParam func(x Term, i int) Term
}

type Ctx[Expr any] struct {
	Params[Expr]

	Stack []Expr
}

// CheckInput returns 0 if the argument expressions are correct.
func (c Ctx[Term]) CheckInput(op spec.Op, args [3]Term, out Term) int {
	switch op.InDegree() {
	}

	switch op {
	case spec.Pass:
		return 0
	}
	return 0
}

// OutputType returns the type of an an expression.
func (c Ctx[Term]) Output(op spec.Op, args [3]Term) Term {
	switch op {
	case spec.Unknown:
		return c.SumOf(nil)
	case spec.Pass:
		return c.TypeOf(args[0])
	case spec.Equal:
		return c.BitType
	case spec.Craft:
		return args[0]
	case spec.Uncraft:
		panic("TODO")
	case spec.TypeOf:
		return c.TypeOf(args[0])
	case spec.SizeOf:
		return c.ArrayOf(c.BitType, 32)
	case spec.MaxSize:
		return c.SizeType()
	case spec.Fingerprint:
		return c.FingerprintType()
	case spec.Root:
		return c.TypeOf(c.TypeOf(c.BitType))
	case spec.Encode:
		return c.ArrayOf(c.BitType, c.SizeOf(c.TypeOf(args[0])))
	case spec.Decode:
		return args[0]

	case spec.ZERO, spec.ONE:
		return c.BitType

	case spec.ArrayEmpty:
		return c.ArrayOf(c.TypeOf(args[0]), 0)
	case spec.ProductEmpty:
		return c.ProductOf(nil)
	case spec.ArrayUnit:
		return c.ArrayOf(c.TypeOf(args[0]), 1)
	case spec.ProductUnit:
		return c.ProductOf(nil)
	case spec.MakeSum:
		return args[0]
	case spec.Concat:
		panic("todo")

	case spec.Let:
		c2 := c
		c2.Stack = append(c2.Stack, args[0])
		op2, args2 := c.Split(args[1])
		return c2.Output(op2, args2)
	case spec.Lazy:
		return c.LazyOf(c.TypeOf(args[0]))
	case spec.Lambda:
		return c.LambdaOf(args[0], args[1])
	case spec.Fractal:
		return c.MakeType2(spec.TC_Fractal)
	case spec.Eval:
		// get LazyType element
		return c.TypeParam(c.TypeOf(args[0]), 0)
	case spec.Apply:
		// get LambdaType output
		return c.TypeParam(c.TypeOf(args[0]), 1)
	case spec.Branch:
		// args[0] and args[1] must be the same type
		return c.TypeOf(args[0])
	case spec.Try:
		return c.SumOf([]Term{c.TypeOf(args[0]), c.AnyValueType})
	case spec.Panic:
		return c.SumOf(nil)

	// Store ops
	case spec.Post:
		return c.RefOf(c.TypeOf(args[0]))
	case spec.Load:
		return c.TypeParam(c.TypeOf(args[0]), 0)

	// Port ops
	case spec.Input:
		return c.TypeParam(c.TypeOf(args[0]), 0)
	case spec.Output:
		return c.TypeParam(c.TypeOf(args[0]), 1)
	case spec.Interact:
		return c.TypeParam(c.TypeOf(args[0]), 3)

	case spec.Gather:
		// get T out of Array[List[T], _]
		return c.ListOf(c.TypeParam(c.TypeParam(c.TypeOf(args[0]), 0), 0))

	default:
		panic(op)
	}
}

func (c Ctx[Term]) SizeType() Term {
	return c.ArrayOf(c.BitType, 32)
}

func (c Ctx[Term]) FingerprintType() Term {
	return c.ArrayOf(c.BitType, 256)
}

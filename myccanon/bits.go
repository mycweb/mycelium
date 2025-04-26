package myccanon

import (
	"fmt"

	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
)

type (
	EB   = mycexpr.EB
	Expr = mycexpr.Expr
)

var eb mycexpr.EB

func lambda(in, out Type, bodyFn func(eb mycexpr.EB) *Expr) *myc.Lambda {
	la, err := mycexpr.BuildLambda(in, out, bodyFn)
	if err != nil {
		panic(err)
	}
	return la
}

var (
	NOT = lambda(myc.BitType{}, myc.BitType{}, func(eb mycexpr.EB) *Expr {
		return eb.Mux(eb.BitArray(1, 0), eb.P(0))
	})
	AND = lambda(myc.ProductType{myc.BitType{}, myc.BitType{}}, myc.BitType{}, func(eb mycexpr.EB) *Expr {
		return eb.Mux(eb.BitArray(0, 0, 0, 1), eb.P(0))
	})
	OR = lambda(myc.ProductType{myc.BitType{}, myc.BitType{}}, myc.BitType{}, func(eb mycexpr.EB) *Expr {
		return eb.Mux(eb.BitArray(0, 1, 1, 1), eb.P(0))
	})
	XOR = lambda(myc.ProductType{myc.BitType{}, myc.BitType{}}, myc.BitType{}, func(eb mycexpr.EB) *Expr {
		return eb.Mux(eb.BitArray(0, 1, 1, 0), eb.P(0))
	})
)

var (
	B32 = myc.B32Type()

	B32_NOT = lambda(B32, B32, func(eb mycexpr.EB) *Expr { return eb.Map(eb.P(0), eb.Lit(NOT)) })
	B32_AND = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return elementWise(eb.Arg(0, 0), eb.Arg(0, 1), eb.Lit(AND)) })
	B32_OR  = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return elementWise(eb.Arg(0, 0), eb.Arg(0, 1), eb.Lit(OR)) })
	B32_XOR = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return elementWise(eb.Arg(0, 0), eb.Arg(0, 1), eb.Lit(XOR)) })

	B32_Neg = lambda(B32, B32, func(eb mycexpr.EB) *Expr { return negNBit(32, eb.P(0)) })

	B32_Add = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return addNBit(32, eb.Arg(0, 0), eb.Arg(0, 1)) })
	B32_Sub = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return subNBit(32, eb.Arg(0, 0), eb.Arg(0, 1)) })
	B32_Mul = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return mulNBit(32, eb.Arg(0, 0), eb.Arg(0, 1)) })
	B32_Div = lambda(myc.ProductType{B32, B32}, B32, func(eb mycexpr.EB) *Expr { return divNBit(32, eb.Arg(0, 0), eb.Arg(0, 1)) })

	B32_POPCOUNT = lambda(B32, B32, func(eb mycexpr.EB) *Expr {
		return eb.Reduce(
			eb.Map(eb.P(0), eb.Lambda(myc.BitType{}, B32, func(eb mycexpr.EB) *Expr {
				return eb.Concat(eb.ArrayUnit(eb.P(0)), mkZeros(31))
			})),
			eb.Lit(B32_Add),
		)
	})
)

var (
	B64 = myc.B64Type()

	B64_NOT = lambda(B64, B64, func(eb mycexpr.EB) *Expr { return eb.Map(eb.P(0), eb.Lit(NOT)) })
	B64_AND = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return elementWise(eb.Arg(0, 0), eb.Arg(0, 1), eb.Lit(AND)) })
	B64_OR  = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return elementWise(eb.Arg(0, 0), eb.Arg(0, 1), eb.Lit(OR)) })
	B64_XOR = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return elementWise(eb.Arg(0, 0), eb.Arg(0, 1), eb.Lit(XOR)) })

	B64_Neg = lambda(B64, B64, func(eb mycexpr.EB) *Expr { return negNBit(64, eb.P(0)) })

	B64_Add = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return addNBit(64, eb.Arg(0, 0), eb.Arg(0, 1)) })
	B64_Sub = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return subNBit(64, eb.Arg(0, 0), eb.Arg(0, 1)) })
	B64_Mul = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return mulNBit(64, eb.Arg(0, 0), eb.Arg(0, 1)) })
	B64_Div = lambda(myc.ProductType{B64, B64}, B64, func(eb mycexpr.EB) *Expr { return divNBit(64, eb.Arg(0, 0), eb.Arg(0, 1)) })

	B64_POPCOUNT = lambda(B64, B64, func(eb mycexpr.EB) *Expr {
		return eb.Reduce(
			eb.Map(eb.P(0), eb.Lambda(myc.BitType{}, B64, func(eb mycexpr.EB) *Expr {
				return eb.Concat(eb.ArrayUnit(eb.P(0)), mkZeros(63))
			})),
			eb.Lit(B64_Add),
		)
	})
)

func elementWise(a, b, fn *Expr) *Expr {
	return eb.Map(eb.Zip(a, b), fn)
}

func negNBit(n int, x *Expr) *Expr {
	// ~x + 1
	return addNBit(n,
		eb.Map(x, eb.Lit(NOT)),
		eb.Concat(eb.Bit(1), mkZeros(n-1)),
	)
}

func subNBit(n int, a, b *Expr) *Expr {
	return eb.Fault(eb.String(fmt.Sprintf("must accelerate b%dSub", n)))
}

func mulNBit(n int, a, b *Expr) *Expr {
	return eb.Fault(eb.String(fmt.Sprintf("must accelerate b%dMul", n)))
}

func divNBit(n int, a, b *Expr) *Expr {
	return eb.Fault(eb.String(fmt.Sprintf("must accelerate b%dDiv", n)))
}

func addNBit(n int, a, b *Expr) *Expr {
	out := make([]*Expr, n)
	carry := eb.Bit(0)
	for i := 0; i < n; i++ {
		ai := eb.Slot(a, eb.B32(uint32(i)))
		bi := eb.Slot(b, eb.B32(uint32(i)))
		out[i], carry = fullAdder(ai, bi, carry)
	}
	return eb.Array(lit(myc.BitType{}), out...)
}

func halfAdder(a, b *Expr) (sum, carryOut *Expr) {
	sum = eb.Apply(eb.Lit(XOR), eb.Product(a, b))
	carryOut = eb.Apply(eb.Lit(AND), eb.Product(a, b))
	return sum, carryOut
}

func fullAdder(a, b, carryIn *Expr) (sum, carryOut *Expr) {
	sum0, carry0 := halfAdder(a, b)
	sum, carry1 := halfAdder(sum0, carryIn)
	carryOut = eb.Apply(eb.Lit(OR), eb.Product(carry0, carry1))
	return sum, carryOut
}

func mkZeros(n int) *Expr {
	bs := make([]myc.Bit, n)
	for i := range bs {
		bs[i] = 0
	}
	a := myc.NewBitArray(bs...)
	return mycexpr.Literal(a)
}

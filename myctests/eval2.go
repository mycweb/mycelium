package myctests

import (
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

// EvalVecs2 has more expensive to compute EvalVecs
// some of them are only worth running with accelerators.
func EvalVecs2(s cadata.PostExister) (out []EvalVec) {
	for _, addVecs := range []func([]EvalVec, cadata.PostExister) []EvalVec{
		arithEval,
		nsEval,
	} {
		out = addVecs(out, s)
	}
	return out
}

func arithEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			Name: "2+3=5",
			I:    modAdd(eb, lit(b32(2)), lit(b32(3))),
			O:    b32(5),
		},
		{
			Name: "8-5=3",
			I:    modSub(eb, lit(b32(8)), lit(b32(5))),
			O:    b32(3),
		},
		{
			Name: "Countdown",
			I: eb.Apply(
				eb.Lambda(
					myc.B32Type(),
					myc.ProductType{},
					func(eb EB) *Expr {
						return eb.If(
							mkExpr(spec.Equal, lit(b32(0)), eb.P(0)),
							lit(Product{}),
							eb.Apply(
								eb.Self(),
								modSub(
									eb,
									eb.P(0),
									lit(b32(1)),
								),
							),
						)
					},
				),
				lit(b32(200)), // The max recursive depth for the tests is 100.
			),
			O: Product{},
		},
		{
			I: eb.Lambda(
				ProductType{B32Type, B32Type},
				B32Type,
				func(eb EB) *Expr { return modAdd(eb, eb.Arg(0, 0), eb.Arg(0, 1)) },
			),
			O: mkLambda(
				ProductType{B32Type, B32Type},
				B32Type,
				func(eb EB) *Expr { return modAdd(eb, eb.Arg(0, 0), eb.Arg(0, 1)) },
			),
		},
		{
			Name: "\\(x) => x + x",
			I: eb.Apply(
				eb.Lambda(
					// Args
					ProductType{B32Type},
					// Return
					B32Type,
					// Body
					func(eb EB) *Expr { return modAdd(eb, eb.Arg(0, 0), eb.Arg(0, 0)) },
				),
				lit(Product{b32(2)}),
			),
			O: b32(4),
		},
		{
			Name: "\\(x, y) => x + y",
			I: eb.Apply(
				eb.Lambda(
					// Args
					ProductType{B32Type, B32Type},
					// Return
					B32Type,
					// Body
					func(eb EB) *Expr { return modAdd(eb, eb.Arg(0, 0), eb.Arg(0, 1)) },
				),
				lit(Product{b32(2), b32(3)}),
			),
			O: b32(5),
		},
	}...)
}

func nsEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			I: eb.Apply(
				eb.Lit(myccanon.NS_Find),
				eb.Product(
					eb.Lit(myccanon.Namespace{
						"a": myc.NewB32(1),
						"b": myc.NewB32(2),
						"k": myc.NewString("this one"),
						"z": myc.NewB32(3),
					}.ToMycelium()),
					eb.String("k"),
				),
			),
			O: myc.NewB32(2),
		},
	}...)
}

func mapReduce(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			Name: "ArrayMap",
			I: mkExpr(spec.Map,
				lit(myc.NewArray(B8Type, b8(1), b8(2), b8(3), b8(4))),
				eb.Lambda(
					B8Type,
					B8Type,
					func(eb EB) *Expr { return modAdd(eb, eb.P(0), eb.P(0)) },
				),
			),
			O: myc.NewArray(B8Type, b8(2), b8(4), b8(6), b8(8)),
		},
		{
			Name: "ArrayReduce",
			I: mkExpr(spec.Reduce,
				lit(myc.NewArray(B8Type, b8(1), b8(2), b8(3), b8(4))),
				eb.Lambda(
					ProductType{B8Type, B8Type},
					myc.B8Type(),
					func(eb EB) *Expr { return modAdd(eb, eb.Arg(0, 0), eb.Arg(0, 1)) },
				),
			),
			O: b8(10),
		},
	}...)
}

func modAdd(eb EB, a, b *Expr) *Expr {
	return eb.Apply(eb.Lit(myccanon.B32_Add), eb.Product(a, b))
}

func modSub(eb EB, a, b *Expr) *Expr {
	return eb.Apply(eb.Lit(myccanon.B32_Sub), eb.Product(a, b))
}

var (
	B8Type  = myc.B8Type()
	B16Type = myc.B16Type()
	B32Type = myc.B32Type()
)

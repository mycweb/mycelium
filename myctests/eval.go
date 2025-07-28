package myctests

import (
	"fmt"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/mycmem"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

type (
	Expr = mycexpr.Expr
	EB   = mycexpr.EB
)

// Eval is a test vector for evaluation
type EvalVec struct {
	Name string
	I    *mycexpr.Expr
	O    myc.Value
	Err  error
}

// EvalVecs returns test vectors for expression evaluation
func EvalVecs(s cadata.PostExister) (out []EvalVec) {
	for _, addVecs := range []func([]EvalVec, cadata.PostExister) []EvalVec{
		literalEval,
		bitEval,
		letEval,
		lambdaEval,
		uncraftEval,
		sizeOfEval,
		typeOfEval,
		lenEval,
		listEval,
		codecEval,
		miscEval,
	} {
		out = addVecs(out, s)
	}

	return out
}

func literalEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	for _, val := range InterestingValues(s) {
		out = append(out, EvalVec{
			I: lit(val),
			O: val,
		})
	}
	return out
}

func letEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	vals := InterestingValues(s)
	// make sure everything can be used in a let
	for _, val := range vals {
		out = append(out, EvalVec{
			I: eb.Let(eb.Lit(val), func(eb EB) *Expr { return eb.P(0) }),
			O: val,
		})
	}
	// make sure everything can be used in a double let
	other := myc.NewB64(12345)
	for _, val := range vals {
		out = append(out, EvalVec{
			I: eb.Let(eb.Lit(val), func(eb EB) *Expr {
				return eb.Let(eb.Lit(other), func(eb EB) *Expr {
					return eb.P(1)
				})
			}),
			O: val,
		})
	}
	return append(out, []EvalVec{
		{
			Name: "Let3 B32",
			I: eb.Let(eb.B32(100), func(eb EB) *Expr {
				return eb.Let(eb.B32(200), func(eb EB) *Expr {
					return eb.Let(eb.B32(300), func(eb EB) *Expr {
						return eb.Product(eb.P(0), eb.P(1), eb.P(2))
					})
				})
			}),
			O: myc.Product{myc.NewSize(300), myc.NewSize(200), myc.NewSize(100)},
		},
		{
			Name: "Let Array[B32, 2]",
			I: eb.Let(eb.Array(eb.Lit(myc.B32Type()), eb.B32(1), eb.B32(2)),
				func(eb EB) *Expr {
					return eb.P(0)
				}),
			O: myc.NewArray(myc.B32Type(), b32(1), b32(2)),
		},
		{
			I: eb.Let(eb.ListFrom(eb.Post(eb.Array(eb.Lit(myc.B32Type())))),
				func(eb EB) *Expr {
					return eb.P(0)
				}),
			O: myc.NewList(myc.B32Type()),
		},
		{
			I: eb.Let(eb.List(eb.String("a1"), eb.String("b2"), eb.String("c3")), func(eb EB) *Expr {
				return eb.Slot(eb.P(0), eb.B32(1))
			}),
			O: myc.NewString("b2"),
		},
		{
			Name: "Let",
			I: eb.LetVal(Product{b32(0), b32(2), b32(3)}, func(eb EB) *Expr {
				return eb.Arg(0, 1)
			}),
			O: b32(2),
		},
	}...)
}

func lambdaEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	vals := InterestingValues(s)

	// everything can be returned from a lambda
	for i, val := range vals {
		out = append(out, EvalVec{
			Name: fmt.Sprintf("LambdaReturn/%d", i),
			I: eb.Apply(
				eb.Lambda(myc.ProductType{}, val.Type(), func(eb EB) *Expr {
					return eb.Lit(val)
				}),
				eb.Product(),
			),
			O: val,
		})
	}
	// // everything can be passed through a lambda
	for i, val := range vals {
		out = append(out, EvalVec{
			Name: fmt.Sprintf("LambdaPass/%d", i),
			I: eb.Apply(
				eb.Lambda(val.Type(), val.Type(), func(eb EB) *Expr {
					return eb.P(0)
				}),
				eb.Lit(val),
			),
			O: val,
		})
	}

	// make sure everything can be returned from a lambda
	return append(out, []EvalVec{
		{
			I: eb.Apply(
				eb.Lambda(myc.ProductType{BitType{}, BitType{}}, myc.BitType{}, func(eb EB) *Expr {
					return mkExpr(spec.Equal,
						eb.Field(mycexpr.Param(0), 0),
						eb.Field(mycexpr.Param(0), 1),
					)
				}),
				lit(myc.Product{bit(0), bit(1)}),
			),
			O: bit(0),
		},
		{
			Name: "closure",
			I: eb.Apply(
				eb.Let(
					eb.B32(123),
					func(eb EB) *Expr {
						return eb.Lambda(myc.ProductType{}, myc.B32Type(), func(eb EB) *Expr {
							return eb.P(1)
						})
					}),
				eb.Product(),
			),
			O: myc.NewB32(123),
		},
	}...)
}

func uncraftEval(out []EvalVec, _ cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			I: mkExpr(spec.Uncraft, eb.RefType(lit(myc.B32Type()))),
			O: Product{myc.NewAnyType(myc.B32Type())},
		},
		{
			I: mkExpr(spec.Uncraft, eb.ArrayType(
				lit(myc.B32Type()),
				eb.B32(0),
			)),
			O: Product{myc.NewAnyType(myc.B32Type()), b32(0)},
		},
		{
			I: mkExpr(spec.Uncraft, eb.ListType(lit(myc.B32Type()))),
			O: Product{myc.NewAnyType(myc.B32Type())},
		},
		{
			I: mkExpr(spec.Uncraft, eb.LazyType(lit(myc.B32Type()))),
			O: Product{myc.NewAnyType(myc.B32Type())},
		},
		{
			I: mkExpr(spec.Uncraft, lit(myc.NewPortType(
				myc.B8Type(),
				myc.B16Type(),
				myc.B32Type(),
				myc.B64Type())),
			),
			O: Product{
				myc.NewAnyType(myc.B8Type()),
				myc.NewAnyType(myc.B16Type()),
				myc.NewAnyType(myc.B32Type()),
				myc.NewAnyType(myc.B64Type()),
			},
		},
	}...)
}

func bitEval(out []EvalVec, _ cadata.PostExister) []EvalVec {
	out = append(out, []EvalVec{
		{I: mkExpr(spec.ZERO), O: myc.NewBit(0)},
		{I: mkExpr(spec.ONE), O: myc.NewBit(1)},
	}...)
	return out
}

func typeOfEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	type vec struct {
		Name  string
		Value myc.Value
		Type  myc.Type
	}
	vecs := []vec{
		{Value: myc.NewAnyType(myc.BitType{}), Type: myc.AnyTypeType{}},
		{Value: myc.NewAnyValue(myc.BitType{}), Type: myc.AnyValueType{}},
	}
	for _, vec := range vecs {
		out = append(out, EvalVec{
			Name: vec.Name,
			I:    eb.TypeOf(lit(vec.Value)),
			O:    vec.Type,
		})
	}

	type vec2 struct {
		Name string
		I    *Expr
		O    Type
	}
	vecs2 := []vec2{
		{
			Name: "Let",
			I: eb.Let(
				lit(b32(2)),
				func(eb EB) *Expr {
					return eb.P(0)
				},
			),
			O: B32Type,
		},
		{
			Name: "Let2",
			I: eb.Let(
				lit(b8(2)),
				func(eb EB) *Expr {
					return eb.Let(
						lit(b8(3)),
						func(eb EB) *Expr {
							return eb.P(1)
						},
					)
				}),
			O: B8Type,
		},
	}
	for _, vec := range vecs2 {
		out = append(out, EvalVec{
			Name: vec.Name,
			I:    eb.TypeOf(vec.I),
			O:    vec.O,
		})
	}
	return out
}

func sizeOfEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	type vec struct {
		Size  int
		Value myc.Value
	}
	vecs := []vec{
		{1, bit(0)},
		{1, bit(1)},
		{spec.AnyValueBits, myc.NewAnyValue(b32(100))},
		{spec.AnyTypeBits, myc.NewAnyType(myc.BitType{})},
		{spec.RefBits, mkRef(s, b16(123))},
		{spec.ListBits, myc.NewString("abcdefghijklmnopqrstuvwxyz")},
		{spec.LambdaBits, mkLambda(myc.ProductType{}, myc.ProductType{}, func(eb EB) *Expr { return eb.Product() })},
	}
	for _, vec := range vecs {
		out = append(out, EvalVec{
			I: mkExpr(spec.SizeOf, mkExpr(spec.AnyTypeFrom, mkExpr(spec.TypeOf, lit(vec.Value)))),
			O: b32(vec.Size),
		})
	}
	return out
}

func lenEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	type vec struct {
		Len   int
		Value myc.Value
	}
	vecs := []vec{
		{0, myc.NewArray(myc.BitType{})},
		{1, myc.NewArray(myc.BitType{}, bit(0))},
		{2, myc.NewArray(myc.BitType{}, bit(0), bit(1))},
	}
	for _, vec := range vecs {
		out = append(out, EvalVec{
			I: EB{}.ArrayLen(lit(vec.Value)),
			O: b32(vec.Len),
		})
	}

	// lists
	vecs = []vec{
		{0, myc.NewString("")},
		{1, myc.NewString("a")},
		{2, myc.NewString("ab")},
		{5, myc.NewString("abcde")},
	}
	for _, vec := range vecs {
		out = append(out, EvalVec{
			I: EB{}.ListLen(lit(vec.Value)),
			O: b32(vec.Len),
		})
	}
	return out
}

func listEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			I: eb.List(),
			O: myc.NewList(myc.Bottom()),
		},
		{
			I: eb.List(eb.B32(1), eb.B32(2), eb.B32(3)),
			O: myc.NewList(myc.B32Type(), b32(1), b32(2), b32(3)),
		},
		{
			// List of Lists
			I: eb.List(eb.String("a"), eb.String("b"), eb.String("c")),
			O: myc.NewList(myc.StringType(), myc.NewString("a"), myc.NewString("b"), myc.NewString("c")),
		},
		{
			I: eb.Slot(
				eb.List(eb.String("first"), eb.String("second"), eb.String("third")),
				eb.B32(1),
			),
			O: myc.NewString("second"),
		},
		{
			I: eb.Load(eb.ListTo(
				eb.List(eb.String("a1"), eb.String("b2"), eb.String("c3")),
				3,
			)),
			O: myc.NewArray(myc.StringType(), myc.NewString("a1"), myc.NewString("b2"), myc.NewString("c3")),
		},
	}...)
}

func miscEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			Name: "quote",
			I:    lit(Product{b32(1), b32(2), b32(3)}),
			O:    Product{b32(1), b32(2), b32(3)},
		},
		{
			Name: "ProductExpr",
			I:    eb.Product(eb.B8(1), eb.B8(2), eb.B8(3), eb.B8(4), eb.B8(5)),
			O:    Product{b8(1), b8(2), b8(3), b8(4), b8(5)},
		},
		{
			Name: "If(true, 111, 222)",
			I:    eb.If(eb.Bit(1), eb.B32(111), eb.B32(222)),
			O:    b32(111),
		},
		{
			Name: "If(false, 111, 222)",
			I:    eb.If(eb.Bit(0), eb.B32(111), eb.B32(222)),
			O:    b32(222),
		},
		{
			Name: "Let Nested Order",
			I: eb.LetVal(Product{b32(2)}, func(eb EB) *Expr {
				return eb.LetVal(Product{b32(3)}, func(eb EB) *Expr {
					return eb.Product(eb.Arg(1, 0), eb.Arg(0, 0))
				})
			}),
			O: Product{b32(2), b32(3)},
		},
		{
			I: eb.Field(eb.Lit(Product{b32(0), b32(1), b32(2), b32(3)}), 2),
			O: b32(2),
		},
		{
			I: eb.Field(
				eb.Product(eb.B8(0), eb.B8(1), eb.B8(2), eb.B8(3), eb.B8(4), eb.B8(5)),
				1,
			),
			O: b8(1),
		},
		{
			Name: "ArrayTypeElem",
			I: eb.ArrayTypeElem(
				eb.ArrayType(eb.Lit(myc.B8Type()), lit(b32(100))),
			),
			O: myc.NewAnyType(myc.B8Type()),
		},
		{
			Name: "ArrayTypeLen",
			I: eb.ArrayTypeLen(
				eb.ArrayType(eb.Lit(myc.B8Type()), eb.B32(100)),
			),
			O: b32(100),
		},
	}...)
}

func codecEval(out []EvalVec, s cadata.PostExister) []EvalVec {
	eb := EB{}
	return append(out, []EvalVec{
		{
			Name: "Encode4Bit",
			I:    eb.Encode(eb.Lit(mycmem.Product{bit(0), bit(1), bit(0), bit(1)})),
			O:    mycmem.NewBitArray(0, 1, 0, 1),
		},
		{
			Name: "Decode3Bit",
			I:    eb.Decode(eb.ProductType(eb.BitType(), eb.BitType(), eb.BitType()), eb.Lit(mycmem.NewBitArray(1, 0, 1))),
			O:    mycmem.Product{bit(1), bit(0), bit(1)},
		},
	}...)
}

func lit(x Value) *Expr {
	return mycexpr.Literal(x)
}

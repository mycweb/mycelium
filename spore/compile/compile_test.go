package compile

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	myc "myceliumweb.org/mycelium/mycmem"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/parser"

	"github.com/stretchr/testify/require"
)

type (
	ProductType = myc.ProductType
	BitType     = myc.BitType
)

func TestCompileAST(t *testing.T) {
	type testCase struct {
		I string
		O myc.Value
	}
	tcs := []testCase{
		{"()", myc.ProductType{}},

		{"0", myc.NewBit(0)},
		{"1", myc.NewBit(1)},
		{"(!comptime [0])", myc.NewBitArray(myc.Bit(0))},
		{"(!comptime [1])", myc.NewBitArray(myc.Bit(1))},
		{"4", myc.NewBitArray(myc.Bit(0), myc.Bit(0), myc.Bit(1))},

		{`""`, myc.NewString("")},
		{`"abcd"`, myc.NewString("abcd")},

		{
			`(!comptime
				(!arrayEmpty Bit)
			)`,
			myc.NewArray(myc.BitType{}),
		},
		{
			`(!comptime
				(!arrayUnit 1)
			)`,
			myc.NewArray(myc.BitType{}, myc.NewBit(1)),
		},
		{
			`(!comptime
				(!concat
					(!arrayUnit 0)
					(!arrayUnit 1)
				)
			)`,
			myc.NewArray(myc.BitType{}, myc.NewBit(0), myc.NewBit(1)),
		},
		{
			fmt.Sprintf(`(!comptime (Product
				(!anyTypeFrom (List (Array Bit 8)))
				(!anyTypeFrom (!craft (!kind %d) {}))
			))`, spec.TC_AnyValue),
			myc.ProductType{myc.ListOf(myc.ByteType()), myc.AnyValueType{}},
		},
		{
			`(!comptime
				(Product Bit Bit Bit)
			)`,
			myc.ProductType{myc.BitType{}, myc.BitType{}, myc.BitType{}},
		},
		{
			`(!comptime (Product))`,
			myc.ProductType{},
		},
		{
			`(!comptime (let {
					x: (b8 3)
					y: (b8 5)
					z: x
				}
				{x y z}
			))`,
			myc.Product{myc.NewB8(3), myc.NewB8(5), myc.NewB8(3)},
		},
		{
			`(!comptime
				(!scope (def x 0)
					(!scope (def y 1)
						(!scope (def z 0)
							{x y z}
						)
					)
				)
			)`,
			myc.Product{myc.NewBit(0), myc.NewBit(1), myc.NewBit(0)},
		},
		{
			`(!comptime
				(!scope (def x (b8 0))
					(!scope (def x (b8 1))
						(!scope (def x (b8 2))
							x
						)
					)
				)
			)`,
			myc.NewB8(2),
		},
		{
			`(!comptime
				(let {
						x: (b8 2)
						y: (b8 3)
					}
					{x y}
				)
			)`,
			myc.Product{myc.NewB8(2), myc.NewB8(3)},
		},
		{
			I: `'123`,
			O: ASTToMycelium(ast.NewInt(123)),
		},
		{
			I: `'"abcd"`,
			O: ASTToMycelium(ast.String("abcd")),
		},
		{
			I: `'%5`,
			O: ASTToMycelium(ast.Param(5)),
		},
		{
			I: `'symbol`,
			O: ASTToMycelium(ast.Symbol("symbol")),
		},
		{
			I: `'(1 2 3)`,
			O: ASTToMycelium(ast.SExpr{ast.NewInt(1), ast.NewInt(2), ast.NewInt(3)}),
		},
		{
			I: `'[1 2 3]`,
			O: ASTToMycelium(ast.Array{ast.NewInt(1), ast.NewInt(2), ast.NewInt(3)}),
		},
		{
			I: `'{1 2 3}`,
			O: ASTToMycelium(ast.Tuple{ast.NewInt(1), ast.NewInt(2), ast.NewInt(3)}),
		},
		{
			I: `'{x: 123}`,
			O: ASTToMycelium(ast.Table{ast.Row{Key: ast.Symbol("x"), Value: ast.NewInt(123)}}),
		},
		{
			I: `(!macro [a b c]
				a
			)`,
			O: mkLambda(myc.ListOf(AST_Node), AST_Node, func(eb EB) *Expr {
				return eb.Slot(eb.P(0), eb.B32(0))
			}),
		},
		{
			I: `(!macro a
				(!slot a 0)
			)`,
			O: mkLambda(myc.ListOf(AST_Node), AST_Node, func(eb EB) *Expr {
				return eb.Slot(eb.P(0), eb.B32(0))
			}),
		},
		{
			I: `
			(!comptime
				((!macro [a] a) 128)
			)
			`,
			O: myc.NewB8(128),
		},
	}

	s := testutil.NewStore(t)
	sc := New(s, nil)

	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			ctx := testutil.Context(t)
			par := parser.NewParser(strings.NewReader(tc.I))
			_, astNode, err := par.ParseAST()
			require.NoError(t, err)
			t.Log(astNode)

			expr, err := sc.CompileAST(ctx, astNode)
			require.NoError(t, err)
			require.NotNil(t, expr)

			var actual myc.Value
			if !expr.IsLiteral() {
				vm := newVM(s)
				actual, err = eval[myc.Value](ctx, s, vm, expr)
				require.NoError(t, err)
			} else {
				actual = expr.Value()
			}
			if !myc.Equal(tc.O, actual) {
				t.Errorf("HAVE: %v\nWANT: %v", actual, tc.O)
				require.Equal(t, tc.O.Type(), actual.Type())
			}
		})
	}
}

func mkExpr(code spec.Op, args ...*Expr) *Expr {
	e, err := mycexpr.NewExpr(code, args...)
	if err != nil {
		panic(err)
	}
	return e
}

func mkLambda(in, out myc.Type, body func(eb EB) *Expr) *myc.Lambda {
	la, err := mycexpr.BuildLambda(in, out, body)
	if err != nil {
		panic(err)
	}
	return la
}

func newVM(s cadata.Store) *mvm1.VM {
	return mvm1.New(0, s, mvm1.DefaultAccels())
}

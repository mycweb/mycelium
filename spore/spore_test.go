package spore

import (
	"strconv"
	"strings"
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/compile"
	"myceliumweb.org/mycelium/spore/parser"

	"github.com/stretchr/testify/require"
)

func TestCompileExpr(t *testing.T) {
	type testCase struct {
		I string
		O myc.Value
	}
	tcs := []testCase{
		{
			`(!comptime
				(lambda {} AnyValue (!anyValueFrom {}))
			)`,
			mkLambda(
				myc.ProductType{},
				myc.AnyValueType{},
				func(eb mycexpr.EB) *mycexpr.Expr {
					return eb.AnyValueFrom(mycexpr.EB{}.Product())
				},
			),
		},
		{
			`(!comptime (lambda
				{a: Bit b: Bit}
				Bit
				(!ZERO)
			))`,
			mkLambda(
				myc.ProductType{myc.BitType{}, myc.BitType{}},
				myc.BitType{},
				func(eb mycexpr.EB) *mycexpr.Expr {
					return eb.Bit(0)
				},
			),
		},
	}

	s := testutil.NewStore(t)
	sc := compile.New(s, preambleNS)

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
			if !expr.IsLiteral() {
				t.Fatalf("%v is not a literal", expr)
			}
			actual := expr.Value()

			if !myc.Equal(tc.O, actual) {
				t.Errorf("HAVE: %v\nWANT: %v", actual, tc.O)
				require.Equal(t, tc.O.Type(), actual.Type())
			}
		})
	}
}

func mkExpr(code spec.Op, args ...*mycexpr.Expr) *mycexpr.Expr {
	e, err := mycexpr.NewExpr(code, args...)
	if err != nil {
		panic(err)
	}
	return e
}

func mkLambda(in, out myc.Type, body func(eb mycexpr.EB) *mycexpr.Expr) *myc.Lambda {
	la, err := mycexpr.BuildLambda(in, out, body)
	if err != nil {
		panic(err)
	}
	return la
}

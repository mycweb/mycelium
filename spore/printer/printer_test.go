package printer

import (
	"testing"

	"myceliumweb.org/mycelium/spore/ast"

	"github.com/stretchr/testify/require"
)

func TestPrinter(t *testing.T) {
	type testCase struct {
		I ast.Node
		O string
	}
	tcs := []testCase{
		{
			I: ast.String("hello world"),
			O: `"hello world"`,
		},
		{
			I: ast.SExpr{ast.String("test"), ast.NewInt(123)},
			O: `("test" 123)`,
		},
		{
			I: ast.Table{
				mkRow(ast.String("k1"), ast.NewInt(1)),
				mkRow(ast.String("k2"), ast.NewInt(2)),
				mkRow(ast.String("k3"), ast.NewInt(3)),
			},
			O: `{"k1": 1,
"k2": 2,
"k3": 3,
}`,
		},
	}
	for _, tc := range tcs {
		p := Printer{}
		require.Equal(t, tc.O, p.PrintString(tc.I))
	}
}

func mkRow(k, v ast.Node) ast.Row {
	return ast.Row{Key: k, Value: v}
}

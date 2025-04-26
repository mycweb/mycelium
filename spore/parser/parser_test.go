package parser

import (
	"strconv"
	"strings"
	"testing"

	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/lexer"

	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	t.Parallel()
	type testCase struct {
		I string
		O Node
	}
	tcs := []testCase{
		{"1234", ast.NewUInt64(1234)},
		{"+1234", ast.NewUInt64(1234)},
		{`"hello world\n"`, ast.String("hello world\n")},
		{"(a b c)", ast.SExpr{ast.Symbol("a"), ast.Symbol("b"), ast.Symbol("c")}},
		{`'(a)`, ast.Quote{X: ast.SExpr{ast.Symbol("a")}}},
		{`[]`, ast.Array{}},
		{`[1 2 3 4]`, ast.Array{ast.NewUInt64(1), ast.NewUInt64(2), ast.NewUInt64(3), ast.NewUInt64(4)}},
		{`{"abc" 123}`, ast.Tuple{ast.String("abc"), ast.NewUInt64(123)}},
		{";; this is a comment", ast.Comment(" this is a comment")},
		{"%0", ast.Param(0)},
		{"%13", ast.Param(13)},
		{"{ k : v }", ast.Table{ast.Row{Key: ast.Symbol("k"), Value: ast.Symbol("v")}}},
	}
	for i, tc := range tcs {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			p := NewParser(strings.NewReader(tc.I))
			span, expr, err := p.ParseAST()
			require.NoError(t, err)
			require.Equal(t, tc.O, expr)
			require.Equal(t, lexer.Span{Begin: 0, End: Pos(len(tc.I))}, span.Bound)
		})
	}
}

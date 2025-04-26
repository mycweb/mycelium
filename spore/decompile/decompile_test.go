package decompile

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myctests"
	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/compile"
)

func TestDecompile(t *testing.T) {
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)

	vals := myctests.InterestingValues(s)
	dc := New(nil)
	preamble := make(map[string]*mycexpr.Expr)
	preamble["String"] = lit(myc.StringType())
	co := compile.New(s, preamble)

	for i, val := range vals {
		val := val
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, ok := val.(*myc.Prog); ok {
				t.SkipNow()
			}
			t.Log("VALUE:", val)
			node := dc.Decompile(val)
			t.Log("AST:", node)
			expr, err := co.CompileAST(ctx, node)
			require.NoError(t, err)
			require.NotNil(t, expr)

			var actual myc.Value
			if expr.IsLiteral() {
				actual = expr.Value()
			} else {
				vm := newVM(s)
				actual, err = eval[myc.Value](ctx, s, vm, expr)
				require.NoError(t, err)
			}
			if !myc.Equal(val, actual) {
				require.Equal(t, val, actual)
			}
		})
	}
}

func decompile(x myc.Value) ast.Node {
	dc := New(nil)
	return dc.Decompile(x)
}

func lit(x myc.Value) *mycexpr.Expr {
	return mycexpr.Literal(x)
}

func newVM(s cadata.Store) *mvm1.VM {
	return mvm1.New(0, s, mvm1.DefaultAccels())
}

func eval[T myc.Value](ctx context.Context, s cadata.Store, vm *mvm1.VM, x *mycexpr.Expr) (ret T, _ error) {
	vm.Reset()
	laz, err := mycexpr.BuildLazy(myc.AnyValueType{}, func(eb mycexpr.EB) *mycexpr.Expr {
		return eb.AnyValueFrom(x)
	})
	if err != nil {
		return ret, err
	}
	if err := vm.ImportLazy(ctx, s, laz); err != nil {
		return ret, err
	}
	vm.SetEval()
	vm.Run(ctx, math.MaxUint64)
	if err := vm.Err(); err != nil {
		return ret, err
	}
	av, err := vm.ExportAnyValue(ctx, s)
	if err != nil {
		return ret, err
	}
	anyval, err := myc.LoadRoot(ctx, s, av.AsBytes())
	if err != nil {
		return ret, err
	}
	ret, ok := anyval.Unwrap().(T)
	if !ok {
		return ret, fmt.Errorf("eval: bad result %v", anyval)
	}
	return ret, nil
}

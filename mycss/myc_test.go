package mycss

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/myccanon/mycjson"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/mycss/internal/migrations"
	"myceliumweb.org/mycelium/myctests"
)

func TestSchema(t *testing.T) {
	ctx := testutil.Context(t)
	db := dbutil.NewTestDB(t)
	require.NoError(t, migrations.Migrate(ctx, db, currentSchema))
}

func TestCreate(t *testing.T) {
	ctx := testutil.Context(t)
	s := newTestSys(t)
	p, err := s.Create(ctx)
	require.NoError(t, err)
	require.NotNil(t, p)
	t.Logf("created pod %v", p)
}

func TestPutGet1(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)

	s := testutil.NewStore(t)
	v1 := myc.NewB32(1234)
	err = p.Put(ctx, s, "a", v1)
	require.NoError(t, err)
	v2, err := p.Get(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, v1, v2)
}

func TestPutGet(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	s1 := testutil.NewStore(t)
	sys := NewTestSys(t)

	for i, val1 := range myctests.InterestingValues(s1) {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pod, err := sys.Create(ctx)
			require.NoError(t, err)
			require.NoError(t, pod.Put(ctx, s1, "x", val1))
			val2, err := pod.Get(ctx, "x")
			require.NoError(t, err)
			if !myc.Equal(val1, val2) {
				t.Errorf("WANT: %v HAVE: %v", val1, val2)
			}
			dst := testutil.NewStore(t)
			require.NoError(t, val2.PullInto(ctx, dst, pod.Store()))
		})
	}
}

func TestInvokeJSON(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)
	s := testutil.NewStore(t)
	require.NoError(t, p.Reset(ctx, s, map[string]myc.Value{
		"invokeJSON": mkLambda(
			myc.ProductType{myccanon.NS_Type, mycjson.JSONType()},
			mycjson.JSONType(),
			func(eb mycexpr.EB) *mycexpr.Expr { return eb.Arg(0, 1) },
		),
	}, PodConfig{}))

	in, err := json.Marshal(map[string]any{
		"k1": "hello world",
		"k2": 12345,
	})
	require.NoError(t, err)
	out, err := InvokeJSON(ctx, p, in)
	require.NoError(t, err)
	t.Logf("%q", out)
}

func newTestSys(t testing.TB) *System {
	ctx := testutil.Context(t)
	db := dbutil.NewTestDB(t)
	require.NoError(t, migrations.Migrate(ctx, db, currentSchema))
	return NewSystem(db)
}

func reset(t testing.TB, pod *Pod, s cadata.Getter, ns myccanon.Namespace, cfg PodConfig) {
	ctx := testutil.Context(t)
	err := pod.Reset(ctx, s, ns, cfg)
	require.NoError(t, err)
}

func eval(t testing.TB, pod *Pod, s cadata.Store, fn func(eb mycexpr.EB) *mycexpr.Expr) myc.Value {
	ctx := testutil.Context(t)
	out, err := Eval(ctx, pod, s, s, func(env myc.Value) *myc.Lazy {
		laz, err := mycexpr.BuildLazy(myc.Bottom(), func(eb mycexpr.EB) *mycexpr.Expr {
			return eb.LetVal(env, fn)
		})
		require.NoError(t, err)
		return laz
	})
	require.NoError(t, err)
	return out
}

func cellLoad(t testing.TB, pod *Pod, s cadata.Store, key string) myc.Value {
	return eval(t, pod, s, func(eb EB) *Expr {
		return eb.Input(
			GetCell(eb.P(0), key),
		)
	}).(*myc.AnyValue).Unwrap()
}

func cellCAS(t testing.TB, pod *Pod, s cadata.Store, key string, prev, next myc.Value) myc.Value {
	return eval(t, pod, s, func(eb EB) *Expr {
		return eb.Interact(
			GetCell(eb.P(0), key),
			mycexpr.Literal(myc.Product{myc.NewAnyValue(prev), myc.NewAnyValue(next)}),
		)
	}).(*myc.AnyValue).Unwrap()
}

func mkLambda(in, out myc.Type, bodyFn func(eb EB) *Expr) *myc.Lambda {
	body := EB{}.Lambda(in, out, bodyFn).Arg(2)
	la, err := myc.NewLambda(in, out, body.Build())
	if err != nil {
		panic(err)
	}
	return la
}

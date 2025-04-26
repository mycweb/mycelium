package mycss

import (
	"fmt"
	"testing"
	"time"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myctests"

	"github.com/stretchr/testify/require"
)

func TestPutNSGet(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	sys := NewTestSys(t)

	s1 := testutil.NewStore(t)
	for i, val1 := range myctests.InterestingValues(s1) {
		val1 := val1
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()
			pod, err := sys.Create(ctx)
			require.NoError(t, err)
			require.NoError(t, pod.Put(ctx, s1, "x", val1))
			s := testutil.NewStore(t)
			av := eval(t, pod, s, func(eb EB) *Expr {
				return myccanon.NSGetExpr(eb.P(0), "x")
			})
			val2 := av.(*myc.AnyValue).Unwrap()
			if !myc.Equal(val1, val2) {
				t.Errorf("WANT: %v HAVE: %v", val1, val2)
			}
		})
	}
}

func TestEval1(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)

	x := myc.NewB32(13)
	s := testutil.NewStore(t)
	out := eval(t, p, s, func(eb EB) *Expr {
		return eb.Lit(x)
	})
	require.NoError(t, err)
	require.Equal(t, x, out)
}

func TestCellRW(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)
	cfg := PodConfig{
		Devices: map[string]DeviceSpec{
			"cell0": DevCell(),
		},
	}
	s := testutil.NewStore(t)
	reset(t, p, s, myccanon.Namespace{
		"cell0": myc.NewB32(13),
	}, cfg)
	// read 13 from the cell
	require.Equal(t, myc.NewB32(13), cellLoad(t, p, s, "cell0"))
	// CAS to 22
	require.Equal(t, myc.NewB32(22), cellCAS(t, p, s, "cell0", myc.NewB32(13), myc.NewB32(22)))
}

func TestLocalPeer(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)
	s := testutil.NewStore(t)
	require.NoError(t, p.Reset(ctx, s, nil, PodConfig{
		Devices: map[string]DeviceSpec{
			"net0": DevNetwork(0),
		},
	}))
	out := eval(t, p, s, func(eb EB) *Expr {
		return LocalAddrExpr(
			GetNetwork(eb.P(0), "net0"),
		)
	})
	t.Log(out)
	require.Equal(t, addrTo(p.LocalAddrs()[0]), out)
}

func TestWallClock(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)
	s := testutil.NewStore(t)
	require.NoError(t, p.Reset(ctx, s, nil, PodConfig{
		Devices: map[string]DeviceSpec{
			"clock": DevWallClock(),
		},
	}))
	out := eval(t, p, s, func(eb EB) *Expr {
		return CurrentTimeExpr(
			GetWallClock(eb.P(0), "clock"),
		)
	})
	require.NoError(t, err)
	t.Log(out)

	secs := int64(*out.(myc.Product)[0].(*myc.B64))
	now := time.Now().Unix()
	require.Greater(t, secs, now-60)
}

func TestRandom(t *testing.T) {
	ctx := testutil.Context(t)
	sys := newTestSys(t)
	p, err := sys.Create(ctx)
	require.NoError(t, err)
	s := testutil.NewStore(t)
	require.NoError(t, p.Reset(ctx, s, nil, PodConfig{
		Devices: map[string]DeviceSpec{
			"random": DevRandom(),
		},
	}))
	out := eval(t, p, s, func(eb EB) *Expr {
		return GenRandomExpr(
			GetRandom(eb.P(0), "random"),
			256,
		)
	})
	t.Log(out)
	require.True(t, myc.Equal(out.Type(), myc.ListOf(myc.BitType{})))
}

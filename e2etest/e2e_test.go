package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"

	"github.com/stretchr/testify/require"
)

func TestReset(t *testing.T) {
	s1, s2 := newSide(t), newSide(t)
	p1, p2 := s1.createPod(t), s2.createPod(t)
	cfg := mycss.PodConfig{}

	reset(t, p1, s1.store, myccanon.Namespace{
		"a": myc.NewB32(117),
		"b": myc.NewB32(13),
	}, cfg)
	reset(t, p2, s2.store, myccanon.Namespace{
		"a": myc.NewB32(117),
	}, cfg)

	require.Equal(t, myc.NewAnyValue(myc.NewB32(117)), eval(t, p1, s1.store,
		myccanon.NSGetExpr(mycexpr.Param(0), "a"),
	))
}

func TestTellReceive(t *testing.T) {
	s1, s2 := newSide(t), newSide(t)
	p1, p2 := s1.createPod(t), s2.createPod(t)
	reset(t, p1, nil, nil, mycss.PodConfig{
		Devices: map[string]mycss.DeviceSpec{"net0": mycss.DevNetwork(0)},
	})
	reset(t, p2, nil, nil, mycss.PodConfig{
		Devices: map[string]mycss.DeviceSpec{"net0": mycss.DevNetwork(0)},
	})

	peer1 := eval(t, p1, s1.store, mycss.LocalAddrExpr(
		mycss.GetNetwork(mycexpr.Param(0), "net0"),
	))
	peer2 := eval(t, p2, s2.store, mycss.LocalAddrExpr(
		mycss.GetNetwork(mycexpr.Param(0), "net0"),
	))
	t.Log("peer1:", peer1)
	t.Log("peer2:", peer2)

	for _, addr := range p2.LocalAddrs() {
		s1.sys.AddLoc(addr.Peer, addr.Location)
	}
	for _, addr := range p1.LocalAddrs() {
		s2.sys.AddLoc(addr.Peer, addr.Location)
	}

	inVal := myc.NewAnyValue(myc.NewB32(100))
	eval(t, p1, s1.store, mycss.TellExpr(
		mycss.GetNetwork(mycexpr.Param(0), "net0"),
		peer2.(myc.Product),
		inVal,
	))

	outVal := eval(t, p2, s2.store, mycexpr.EB{}.Field(mycss.ReceiveExpr(
		mycss.GetNetwork(mycexpr.Param(0), "net0"),
	), 1))
	t.Skip() // TODO: get this last check working
	require.Equal(t, inVal, outVal)
}

func newSide(t testing.TB) side {
	sys := mycss.NewTestSys(t)
	return side{
		sys:   sys,
		store: testutil.NewStore(t),
	}
}

func reset(t testing.TB, pod *mycss.Pod, src cadata.Getter, ns myccanon.Namespace, cfg mycss.PodConfig) {
	t.Helper()
	ctx := testutil.Context(t)
	err := pod.Reset(ctx, src, ns, cfg)
	require.NoError(t, err)
}

func eval(t testing.TB, pod *mycss.Pod, s cadata.Store, v *mycexpr.Expr) myc.Value {
	t.Helper()
	ctx, cf := context.WithTimeoutCause(testutil.Context(t), 3*time.Second, errors.New("eval took too long"))
	defer cf()
	out, err := mycss.Eval(ctx, pod, s, s, func(env myc.Value) *myc.Lazy {
		laz, err := mycexpr.BuildLazy(myc.Bottom(), func(eb mycexpr.EB) *mycexpr.Expr {
			return eb.LetVal(env, func(eb mycexpr.EB) *mycexpr.Expr { return v })
		})
		require.NoError(t, err)
		return laz
	})
	require.NoError(t, err)
	return out
}

type side struct {
	sys *mycss.System
	// store can be used as scratch space for loading and retrieving values from the pod.
	store cadata.Store
}

func (s *side) createPod(t testing.TB) *mycss.Pod {
	ctx := testutil.Context(t)
	pod, err := s.sys.Create(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { s.sys.Drop(ctx, pod.ID()) })
	return pod
}

package testmycss

import (
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/mycss"

	"github.com/stretchr/testify/require"
)

func New(t testing.TB) *mycss.System {
	return mycss.NewTestSys(t)
}

func Create(t testing.TB, sys *mycss.System) *mycss.Pod {
	ctx := testutil.Context(t)
	pod, err := sys.Create(ctx)
	require.NoError(t, err)
	return pod
}

package build

import (
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"

	"github.com/stretchr/testify/require"
)

func TestStdLib(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)
	c := NewContext([]Source{StdLib()})

	pkgPaths, err := c.List("")
	require.NoError(t, err)
	t.Log(pkgPaths)
	for _, pkgPath := range pkgPaths {
		name := pkgPath
		t.Run(name, func(t *testing.T) {
			t.Log("PACKAGE: ", name)
			pkg, err := c.Build(ctx, s, name)
			require.NoError(t, err)
			ns := pkg.NS
			t.Log("CONTENT:", myccanon.NSPretty(ns))
			require.NotEmpty(t, ns)
		})
	}
}

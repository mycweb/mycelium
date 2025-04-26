package myczip

import (
	"archive/zip"
	"fmt"
	"io"
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"
	mycelium "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/myctests"

	"github.com/stretchr/testify/require"
)

func TestWriteRead(t *testing.T) {
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)
	v := mycelium.NewArray(mycelium.B32Type(), mycelium.NewB32(1), mycelium.NewB32(2), mycelium.NewB32(3))

	f := testutil.TempFile(t)
	require.NoError(t, WriteTo(ctx, s, v, f))

	_, err := f.Seek(0, io.SeekStart)
	require.NoError(t, err)
	finfo, err := f.Stat()
	require.NoError(t, err)
	zr, err := zip.NewReader(f, finfo.Size())
	require.NoError(t, err)
	actual, _, err := Load(zr)
	require.NoError(t, err)
	mycelium.Equal(actual, v)
}

func TestValues(t *testing.T) {
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)
	tcs := myctests.InterestingValues(s)
	for i, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Log(tc)
			f := testutil.TempFile(t)
			require.NoError(t, WriteTo(ctx, s, tc, f))
			_, err := f.Seek(0, io.SeekStart)
			require.NoError(t, err)
			finfo, err := f.Stat()
			require.NoError(t, err)
			zr, err := zip.NewReader(f, finfo.Size())
			require.NoError(t, err)
			actual, _, err := Load(zr)
			require.NoError(t, err)
			mycelium.Equal(actual, tc)
		})
	}
}

package mycmem

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/spec"
)

func TestNodeSize(t *testing.T) {
	type testCase struct {
		N Node
		S int
	}
	tcs := []testCase{
		{
			N: Node{code: spec.Pass, args: [3]uint32{1, 0, 0}},
			S: spec.OpBits + 8,
		},
		{
			N: Node{code: spec.LiteralAnyValue},
			S: spec.OpBits + spec.AnyValueBits,
		},
		{
			N: Node{code: spec.LiteralAnyType},
			S: spec.OpBits + spec.AnyTypeBits,
		},
	}
	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			require.Equal(t, tc.S, tc.N.Size())
		})
	}
}

func TestEncodeProg(t *testing.T) {
	ctx := testutil.Context(t)
	s := testutil.NewStore(t)
	type testCase struct {
		I Prog
	}
	tcs := []testCase{
		{
			I: Prog{
				{code: spec.LiteralAnyValue, litAnyValue: NewAnyValue(NewString("abcd"))},
				{code: spec.Pass, args: [3]uint32{1, 0, 0}},
			},
		},
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			for _, node := range tc.I {
				if node.litAnyValue != nil {
					_, err := Post(ctx, s, node.litAnyValue)
					require.NoError(t, err)
				}
				if node.litAnyType != nil {
					_, err := Post(ctx, s, node.litAnyType)
					require.NoError(t, err)
				}
			}
			// encode
			var data []byte
			var err error
			data, err = encodeProg(data, tc.I)
			require.NoError(t, err)
			t.Log(hex.Dump(data))
			require.Equal(t, tc.I.Size(), 8*len(data))
			// decode
			load := func(ref Ref) (Value, error) {
				return Load(ctx, s, ref)
			}
			prog, err := decodeProg(data, load)
			require.NoError(t, err)
			require.Equal(t, tc.I, prog)
		})
	}
}

package mycjson

import (
	"encoding/json"
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"

	"myceliumweb.org/mycelium/internal/cadata"

	"github.com/stretchr/testify/require"
)

func TestJSONPull(t *testing.T) {
	ctx := testutil.Context(t)
	src := testutil.NewStore(t)
	node1 := NewJSON(map[string]any{
		"a": []any{"string1", "string2", "string3"},
		"b": json.Number("1234"),
		"c": true,
		"d": false,
		"e": map[string]any{
			"k1": json.Number("1"),
			"k2": json.Number("2"),
		},
	})
	// encode
	val, err := EncodeJSON(node1)
	require.NoError(t, err)
	t.Log("encode complete, created", src.Len(), "objects")
	cadata.ForEach(ctx, src, cadata.Span{}, func(id cadata.ID) error {
		t.Log(id)
		return nil
	})
	// pull into dst from src
	dst := testutil.NewStore(t)
	require.NoError(t, PullJSON(ctx, dst, src, node1))

	// decode from dst
	node2, err := DecodeJSON(ctx, dst, val)
	require.NoError(t, err)
	require.Equal(t, node1, node2)
}

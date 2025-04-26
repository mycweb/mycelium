package mychtml

import (
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
)

func TestHTMLPull(t *testing.T) {
	ctx := testutil.Context(t)
	src := testutil.NewStore(t)
	node1 := mkDiv(
		mkDiv(mkP("this is some paragraph text")),
		mkDiv(mkDiv(mkP("other paragraph"))),
	)
	// encode
	val, err := EncodeHTML(ctx, src, node1)
	require.NoError(t, err)
	t.Log("encode complete, created", src.Len(), "objects")
	cadata.ForEach(ctx, src, cadata.Span{}, func(id cadata.ID) error {
		t.Log(id)
		return nil
	})
	// pull into dst from src
	dst := testutil.NewStore(t)
	_, err = DecodeHTML(ctx, dst, val) // this should fail because nothing has been synced.
	require.Error(t, err)
	require.NoError(t, PullHTML(ctx, dst, src, val))
	// decode from dst
	node2, err := DecodeHTML(ctx, dst, val)
	require.NoError(t, err)
	require.Equal(t, node1, node2)
}

func mkDiv(nodes ...HTML) HTML {
	return HTML{Type: "div", Children: nodes}
}

func mkP(text string) HTML {
	return HTML{Type: "p", Text: text}
}

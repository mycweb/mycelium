package mychui

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon/mychtml"
	mycelium "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"
	"myceliumweb.org/mycelium/mycss/testmycss"

	"github.com/stretchr/testify/require"
)

func TestRenderHTML(t *testing.T) {
	ctx := testutil.Context(t)
	sys := testmycss.New(t)
	pod := testmycss.Create(t, sys)
	lAddr := startServing(t, sys)

	node1 := mychtml.HTML{
		Type: "div",
		Children: []mychtml.HTML{
			{Type: "p", Text: "This is a paragraph"},
			{Type: "b", Text: "and this is in bold"},
		},
	}
	require.NoError(t, SetRenderHTML(ctx, pod, node1))

	u := mkURL(lAddr, pod) + "renderHTML"
	t.Log("url", u)
	resp, err := http.Get(u)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Logf("RESP: %q", data)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestBlob(t *testing.T) {
	ctx := testutil.Context(t)
	store := testutil.NewStore(t)
	sys := testmycss.New(t)
	pod := testmycss.Create(t, sys)
	lAddr := startServing(t, sys)

	ref, err := mycelium.Post(ctx, store, mycelium.NewB8(65))
	require.NoError(t, err)
	require.NoError(t, pod.Put(ctx, store, "data1", &ref))
	require.NoError(t, err)

	u := mkURL(lAddr, pod) + "blob/" + ref.String()[1:]
	t.Log("url", u)
	resp, err := http.Get(u)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Logf("RESP: %q", data)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func mkURL(addr net.Addr, pod *mycss.Pod) string {
	return fmt.Sprintf("http://%s/v1/pod/%d/", addr.String(), pod.ID())
}

func startServing(t testing.TB, sys *mycss.System) net.Addr {
	ctx := testutil.Context(t)
	srv := New(sys)
	lis := testutil.Listen(t)
	go srv.Serve(ctx, lis)
	return lis.Addr()
}

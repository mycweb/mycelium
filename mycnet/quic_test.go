package mycnet

import (
	"net/netip"
	"testing"

	"github.com/cloudflare/circl/sign/ed25519"
	"github.com/stretchr/testify/require"
	"go.brendoncarroll.net/p2p/p2ptest"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/testutil"
	myc "myceliumweb.org/mycelium/mycmem"
)

func TestTell(t *testing.T) {
	ctx := testutil.Context(t)
	type incomingTell struct {
		From  QUICAddr
		Value *Artifact
	}

	mailbox2 := make(chan incomingTell, 2)
	h1 := newHost(t, 1, nil, nil)
	h2 := newHost(t, 2, func(from QUICAddr, x Artifact) error {
		mailbox2 <- incomingTell{
			From:  from,
			Value: &x,
		}
		return nil
	}, nil)

	in := myc.NewB32(13)
	require.NoError(t, h1.TellAnyVal(ctx, h2.LocalAddr(), ArtifactFromMemory(myc.NewAnyValue(in))))
	x := <-mailbox2
	require.Equal(t, x.From, h1.LocalAddr())
	av, err := x.Value.Slurp(ctx, Limits{})
	require.NoError(t, err)
	require.Equal(t, in, av.Unwrap())
}

func TestAsk(t *testing.T) {
	ctx := testutil.Context(t)
	h1 := newHost(t, 1, nil, nil)
	h2 := newHost(t, 2, nil, func(from QUICAddr, x Artifact) (*Artifact, error) {
		av, err := x.Slurp(ctx, Limits{})
		if err != nil {
			return nil, err
		}
		y := *(av.Unwrap().(*myc.B32)) + 1
		ret := ArtifactFromMemory(myc.NewAnyValue(&y))
		return &ret, nil
	})

	resp, err := h1.AskAnyVal(ctx, h2.LocalAddr(), ArtifactFromMemory(myc.NewAnyValue(myc.NewB32(7))))
	require.NoError(t, err)
	respAV, err := resp.Slurp(ctx, Limits{})
	require.NoError(t, err)
	require.Equal(t, myc.NewB32(8), respAV.Unwrap())
}

func TestBlobPull(t *testing.T) {
	ctx := testutil.Context(t)
	h1 := newHost(t, 1, nil, nil)
	h2 := newHost(t, 2, nil, nil)

	var salt *cadata.ID
	data := []byte("hello world")
	targetID, err := h2.repo.s.Post(ctx, salt, data)
	require.NoError(t, err)

	// success
	buf := make([]byte, 1024)
	n, err := h1.client.blobPull(ctx, h2.tp.LocalAddr(), &targetID, salt, buf)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	// not found case
	nfID := mycelium.Hash(salt, []byte("does not exist"))
	_, err = h1.client.blobPull(ctx, h2.tp.LocalAddr(), &nfID, salt, make([]byte, 100))
	require.ErrorAs(t, err, &cadata.ErrNotFound{})
}

func newHost(t testing.TB, i int, onTell TellHandler[netip.AddrPort], onAsk AskHandler[netip.AddrPort]) *Host[netip.AddrPort] {
	ctx := testutil.Context(t)
	priv := ed25519.PrivateKey(p2ptest.NewTestKey(t, i))
	qt := NewQUIC(priv, testutil.NewPacketConn(t))
	h := NewHost(qt, onTell, onAsk)
	go h.Run(ctx)
	return h
}

package testutil

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.brendoncarroll.net/stdctx/logctx"
	"go.uber.org/zap"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
)

func Context(t testing.TB) context.Context {
	ctx := context.Background()
	ctx, cf := context.WithCancel(ctx)
	t.Cleanup(cf)
	l, err := zap.NewDevelopment()
	require.NoError(t, err)
	ctx = logctx.NewContext(ctx, l)
	return ctx
}

func NewStore(t testing.TB) *stores.Mem {
	return stores.NewMem(func(salt *cadata.ID, x []byte) (ret cadata.ID) {
		return mycelium.Hash(salt, x)
	}, 1<<21)
}

func NewPacketConn(t testing.TB) net.PacketConn {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		pc.Close()
	})
	return pc
}

func Listen(t testing.TB) net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })
	return l
}

// TempFile creates a temp file, unlinks it, and then returns the file.
// TempFile adds f.Close for Cleanup
func TempFile(t testing.TB) *os.File {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	require.NoError(t, os.Remove(f.Name()))
	return f
}

package mycnet

import (
	"context"
	"fmt"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/mycbytes"
	myc "myceliumweb.org/mycelium/mycmem"
)

type Addr[T comparable] struct {
	Peer     Peer
	Location T
}

func (a Addr[T]) String() string {
	return fmt.Sprintf("%v@%v", a.Peer, a.Location)
}

// Limits are contraints on values
type Limits struct {
	MaxDepth int
	MaxBlobs int
	MaxBits  int
}

// Handler is called to handle message.
// Tell interactions will pass nil for res.
// Ask interactions will pass non-nil for req and res.
// The response to the ask should be written to resp.
type MsgHandler[T comparable] = func(ctx context.Context, from Addr[T], req, resp *Message) error

// Transport provides message based communication between nodes.
type Transport[T comparable] interface {
	Tell(ctx context.Context, raddr Addr[T], msg *Message) error
	Ask(ctx context.Context, raddr Addr[T], req, res *Message) error
	Serve(ctx context.Context, fn MsgHandler[T]) error
	LocalAddr() Addr[T]
}

type TellHandler[T comparable] = func(Addr[T], Artifact) error

type AskHandler[T comparable] = func(Addr[T], Artifact) (*Artifact, error)

// Artifact is a Artifact and store containing all transitively reachable Values
type Artifact struct {
	Root  mycbytes.AnyValue
	Store cadata.Getter
}

// ArtifactFromMemory creates an Artifact using an in memory Mycelium Value.
func ArtifactFromMemory(av *myc.AnyValue) Artifact {
	s := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	if err := av.PullInto(context.TODO(), s, stores.Union{}); err != nil {
		panic(err)
	}
	return Artifact{
		Root:  mycbytes.AnyValue(myc.MarshalAppend(nil, av)),
		Store: s,
	}
}

func newArtifact(root []byte, s cadata.Getter) (Artifact, error) {
	if len(root) < len(mycbytes.AnyValue{}) {
		return Artifact{}, fmt.Errorf("wrong length for AnyValue")
	}
	return Artifact{Root: mycbytes.AnyValue(root), Store: s}, nil
}

func (af *Artifact) Slurp(ctx context.Context, lim Limits) (*myc.AnyValue, error) {
	return myc.LoadRoot(ctx, af.Store, af.Root[:])
}

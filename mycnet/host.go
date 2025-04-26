package mycnet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/mycbytes"
	myc "myceliumweb.org/mycelium/mycmem"
)

// Host is a Client and Server
type Host[T comparable] struct {
	tp Transport[T]

	repo   *repo
	client client[T]
	server server[T]
}

// NewHost creates a new host, which will communicate using tp
func NewHost[T comparable](tp Transport[T], onTell TellHandler[T], onAsk AskHandler[T]) *Host[T] {
	if onTell == nil {
		onTell = func(a Addr[T], rv Artifact) error { return fmt.Errorf("host does not support tells") }
	}
	if onAsk == nil {
		onAsk = func(a Addr[T], rv Artifact) (*Artifact, error) {
			return nil, fmt.Errorf("host does not support asks")
		}
	}
	repo := newRepo()
	client := client[T]{
		tp: tp,
	}
	return &Host[T]{
		tp: tp,

		repo:   repo,
		client: client,
		server: server[T]{
			tp:   tp,
			repo: repo,

			OnTellAnyVal: onTell,
			OnAskAnyVal:  onAsk,
		},
	}
}

func (h *Host[T]) LocalAddr() Addr[T] {
	return h.tp.LocalAddr()
}

func (h *Host[T]) TellAnyVal(ctx context.Context, dst Addr[T], msg Artifact) error {
	cleanup := h.repo.Pin(msg)
	time.AfterFunc(time.Minute, cleanup)
	return h.client.tellAnyVal(ctx, dst, msg.Root)
}

func (h *Host[T]) AskAnyVal(ctx context.Context, dst Addr[T], req Artifact) (*Artifact, error) {
	cleanupReq := h.repo.Pin(req)
	defer cleanupReq()
	resp, err := h.client.askAnyVal(ctx, dst, req.Root)
	if err != nil {
		return nil, err
	}
	return &Artifact{
		Store: &remoteStore[T]{
			raddr: dst,
			tp:    h.tp,
		},
		Root: *resp,
	}, nil
}

func (h *Host[T]) Run(ctx context.Context) error {
	return h.tp.Serve(ctx, h.server.Handle)
}

type client[T comparable] struct {
	tp Transport[T]
}

func (c client[T]) blobPull(ctx context.Context, dst Addr[T], id *cadata.ID, salt *cadata.ID, buf []byte) (int, error) {
	var req, resp Message
	req.SetBlobPull(*id)
	if err := c.tp.Ask(ctx, dst, &req, &resp); err != nil {
		return 0, err
	}
	if resp.Type() != MT_BLOB_PUSH {
		return 0, fmt.Errorf("response to blob pull must be blob push. HAVE: %v", resp.Type())
	}
	if bytes.HasPrefix(resp.Body(), id[:]) {
		return 0, cadata.ErrNotFound{Key: id}
	}
	if err := cadata.Check(mycelium.Hash, id, salt, resp.Body()); err != nil {
		return 0, err
	}
	return copy(buf[:], resp.Body()), nil
}

func (c client[T]) tellAnyVal(ctx context.Context, dst Addr[T], av mycbytes.AnyValue) error {
	var msg Message
	msg.SetAnyValTell(av[:])
	return c.tp.Tell(ctx, dst, &msg)
}

func (c client[T]) askAnyVal(ctx context.Context, dst Addr[T], av mycbytes.AnyValue) (*mycbytes.AnyValue, error) {
	var req, resp Message
	req.SetAnyValAsk(av[:])
	if err := c.tp.Ask(ctx, dst, &req, &resp); err != nil {
		return nil, err
	}
	if resp.Type() != MT_ANYVAL_REPLY {
		return nil, fmt.Errorf("response to blob pull must be blob push. HAVE: %v", resp.Type())
	}
	ret := mycbytes.AnyValue(resp.Body())
	return &ret, nil
}

func (c client[T]) RemoteStore(raddr Addr[T]) cadata.Getter {
	return &remoteStore[T]{raddr: raddr, tp: c.tp}
}

type server[T comparable] struct {
	tp   Transport[T]
	repo *repo

	OnTellAnyVal TellHandler[T]
	OnAskAnyVal  AskHandler[T]
}

func (s *server[T]) Handle(ctx context.Context, from Addr[T], req, res *Message) error {
	if res == nil {
		return s.handleTell(ctx, from, req)
	} else {
		return s.handleAsk(ctx, from, req, res)
	}
}

func (s *server[T]) handleTell(ctx context.Context, from Addr[T], req *Message) error {
	switch req.Type() {
	case MT_ANYVAL_TELL:
		af, err := newArtifact(req.Body(), &remoteStore[T]{tp: s.tp, raddr: from})
		if err != nil {
			return err
		}
		return s.OnTellAnyVal(from, af)
	default:
		return fmt.Errorf("message type %v not allowed in tell", req.Type())
	}
}

func (s *server[T]) handleAsk(ctx context.Context, from Addr[T], req, resp *Message) error {
	switch req.Type() {
	case MT_BLOB_PULL:
		id, err := req.AsID()
		if err != nil {
			return err
		}
		resp.setType(MT_BLOB_PUSH)
		store := s.repo.Open(from.Peer.ID())
		n, err := store.Get(ctx, &id, nil, resp.MaxBuf())
		if errors.As(err, &cadata.ErrNotFound{}) {
			resp.SetBlobNotFound(id)
			return nil
		} else if err != nil {
			return err
		}
		resp.SetLen(n)
		return nil
	case MT_ANYVAL_ASK:
		rs := &remoteStore[T]{tp: s.tp, raddr: from}
		rv, err := newArtifact(req.Body(), rs)
		if err != nil {
			return err
		}
		reply, err := s.OnAskAnyVal(from, rv)
		if err != nil {
			return err
		}
		cleanup := s.repo.Pin(*reply)
		time.AfterFunc(time.Minute, cleanup)
		resp.SetAnyValReply(reply.Root[:])
		return nil
	default:
		return fmt.Errorf("message type %v cannot initiate ask", req.Type())
	}
}

// RemoteStore implements a cadata.Getter using the BlobPull/Push protocol messages.
type remoteStore[T comparable] struct {
	tp    Transport[T]
	raddr Addr[T]
}

func (s *remoteStore[T]) Get(ctx context.Context, id *cadata.ID, salt *cadata.ID, buf []byte) (int, error) {
	c := client[T]{tp: s.tp}
	return c.blobPull(ctx, s.raddr, id, salt, buf)
}

func (s *remoteStore[T]) Hash(salt *cadata.ID, x []byte) cadata.ID {
	return mycelium.Hash(salt, x)
}

func (s *remoteStore[T]) MaxSize() int {
	return mycelium.MaxSizeBytes
}

// repo stores artifacts
type repo struct {
	s cadata.Store
}

func newRepo() *repo {
	return &repo{
		s: stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes),
	}
}

func (r *repo) Pin(x Artifact) func() {
	ctx := context.Background()
	av, err := myc.LoadRoot(ctx, x.Store, x.Root[:])
	if err != nil {
		panic(err)
	}
	if err := av.PullInto(ctx, r.s, x.Store); err != nil {
		panic(err)
	}
	return func() {}
}

func (r *repo) Open(peer PeerID) cadata.Getter {
	return r.s
}

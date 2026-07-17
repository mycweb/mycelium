package stores

import (
	"context"

	"blobcache.io/blobcache/src/blobcache"
	"myceliumweb.org/mycelium"
)

type CoW struct {
	Write mycelium.RW
	Read  mycelium.RO
}

func (s CoW) Post(ctx context.Context, tag *mycelium.CID, data []byte) (mycelium.CID, error) {
	return s.Write.Post(ctx, tag, data)
}

func (s CoW) Delete(ctx context.Context, id mycelium.CID) error {
	if d, ok := any(s.Write).(interface {
		Delete(context.Context, []mycelium.CID) error
	}); ok {
		return d.Delete(ctx, []mycelium.CID{id})
	}
	return nil
}

func (s CoW) Get(ctx context.Context, id mycelium.CID, salt *mycelium.CID, buf []byte) (int, error) {
	return Union{s.Write, s.Read}.Get(ctx, id, salt, buf)
}

func (s CoW) Exists(ctx context.Context, cids []mycelium.CID, bm *blobcache.BitMap) error {
	return Union{s.Write, s.Read}.Exists(ctx, cids, bm)
}

func (s CoW) KeyedHash(salt *mycelium.CID, data []byte) mycelium.CID {
	return s.Write.KeyedHash(salt, data)
}

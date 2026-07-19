package stores

import (
	"context"
	"errors"

	"blobcache.io/blobcache/src/blobcache"
	"myceliumweb.org/mycelium"
)

type GetExister interface {
	mycelium.RO
}

type Union []mycelium.RO

func (s Union) Get(ctx context.Context, id mycelium.CID, salt *mycelium.CID, buf []byte) (int, error) {
	for _, s2 := range s {
		n, err := s2.Get(ctx, id, salt, buf)
		if errors.As(err, &blobcache.ErrNotFound{}) {
			continue
		}
		return n, err
	}
	return 0, blobcache.ErrNotFound{CID: id}
}

func (s Union) Exists(ctx context.Context, cids []mycelium.CID, bm *blobcache.BitMap) error {
	for i, cid := range cids {
		for _, s2 := range s {
			var bm2 blobcache.BitMap
			if err := s2.Exists(ctx, []mycelium.CID{cid}, &bm2); err != nil {
				return err
			}
			if bm2.IsSet(0) {
				bm.Set(i)
				break
			}
		}
	}
	return nil
}

func (s Union) KeyedHash(salt *mycelium.CID, data []byte) mycelium.CID {
	if len(s) > 0 {
		return s[0].KeyedHash(salt, data)
	}
	return mycelium.Hash(salt, data)
}

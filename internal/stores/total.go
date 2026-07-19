package stores

import (
	"context"

	"blobcache.io/blobcache/src/blobcache"
	"myceliumweb.org/mycelium"
)

var _ mycelium.WO = &Total{}

type Total struct {
	hash    mycelium.KHashFunc
	maxSize int
}

func NewTotal(hash mycelium.KHashFunc, maxSize int) *Total {
	return &Total{maxSize: maxSize, hash: hash}
}

func (t Total) Post(ctx context.Context, tag *mycelium.CID, data []byte) (mycelium.CID, error) {
	return t.hash(tag, data), nil
}

func (t Total) Exists(ctx context.Context, cids []mycelium.CID, bm *blobcache.BitMap) error {
	for i := range cids {
		bm.Set(i)
	}
	return nil
}

func (t Total) KeyedHash(tag *mycelium.CID, x []byte) mycelium.CID {
	return t.hash(tag, x)
}

func (t Total) MaxSize() int {
	return t.maxSize
}

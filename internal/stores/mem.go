package stores

import (
	"context"

	"blobcache.io/blobcache/src/blobcache"
	"go.brendoncarroll.net/state"
	"go.brendoncarroll.net/state/kv"

	"myceliumweb.org/mycelium"
)

// var _ mycelium.RW = &Mem{}

type memEntry struct {
	salt  mycelium.CID
	value []byte
}

type Mem struct {
	hf      blobcache.KeyedHashFunc
	maxSize int
	kv      *kv.MemStore[mycelium.CID, memEntry]
}

func NewMem(hf blobcache.KeyedHashFunc, maxSize int) *Mem {
	return &Mem{
		kv: kv.NewMemStore[mycelium.CID, memEntry](func(a, b mycelium.CID) int {
			return a.Compare(b)
		}),
		hf:      hf,
		maxSize: maxSize,
	}
}

func (s *Mem) Post(ctx context.Context, salt *mycelium.CID, data []byte) (mycelium.CID, error) {
	if len(data) > s.maxSize {
		return mycelium.CID{}, blobcache.ErrTooLarge{MaxSize: s.maxSize, BlobSize: len(data)}
	}
	id := s.hf(salt, data)
	var storedSalt mycelium.CID
	if salt != nil {
		storedSalt = *salt
	}
	if err := s.kv.Put(ctx, id, memEntry{
		salt:  storedSalt,
		value: append([]byte{}, data...),
	}); err != nil {
		return mycelium.CID{}, err
	}
	return id, nil
}

func (s *Mem) Get(ctx context.Context, id mycelium.CID, salt *mycelium.CID, buf []byte) (int, error) {
	ent, err := kv.Get(ctx, s.kv, id)
	if err != nil {
		if state.IsErrNotFound[mycelium.CID](err) {
			return 0, blobcache.ErrNotFound{CID: id}
		}
		return 0, err
	}
	return copy(buf, ent.value), nil
}

func (s *Mem) Exists(ctx context.Context, cids []mycelium.CID, bm *blobcache.BitMap) error {
	for i, cid := range cids {
		yes, err := s.kv.Exists(ctx, cid)
		if err != nil {
			return err
		}
		if yes {
			bm.Set(i)
		}
	}
	return nil
}

func (s *Mem) Delete(ctx context.Context, cids []mycelium.CID) error {
	for _, cid := range cids {
		if err := s.kv.Delete(ctx, cid); err != nil {
			return err
		}
	}
	return nil
}

func (s *Mem) KeyedHash(salt *mycelium.CID, data []byte) mycelium.CID {
	return s.hf(salt, data)
}

// func (s *Mem) List(ctx context.Context, span mycelium.Span, ids []mycelium.CID) (int, error) {
// 	return s.kv.List(ctx, span, ids)
// }

func (s *Mem) All() (ret []mycelium.CID) {
	kv.ForEach(context.TODO(), s.kv, state.TotalSpan[mycelium.CID](), func(i mycelium.CID) error {
		ret = append(ret, i)
		return nil
	})
	return ret
}

func (s *Mem) Len() int {
	return s.kv.Len()
}

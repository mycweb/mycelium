package stores

import (
	"context"

	"go.brendoncarroll.net/state"
	"go.brendoncarroll.net/state/kv"

	"myceliumweb.org/mycelium/internal/cadata"
)

var _ cadata.Store = &Mem{}

type memEntry struct {
	salt  *cadata.ID
	value []byte
}

type Mem struct {
	hf      cadata.HashFunc
	maxSize int
	kv      *kv.MemStore[cadata.ID, memEntry]
}

func NewMem(hf cadata.HashFunc, maxSize int) *Mem {
	return &Mem{
		kv: kv.NewMemStore[cadata.ID, memEntry](func(a, b cadata.ID) int {
			return a.Compare(b)
		}),
		hf:      hf,
		maxSize: maxSize,
	}
}

func (s *Mem) Post(ctx context.Context, salt *cadata.ID, data []byte) (cadata.ID, error) {
	if len(data) > s.maxSize {
		return cadata.ID{}, cadata.ErrTooLarge
	}
	id := s.hf(salt, data)
	if err := s.kv.Put(ctx, id, memEntry{
		salt:  cloneSalt(salt),
		value: append([]byte{}, data...),
	}); err != nil {
		return cadata.ID{}, err
	}
	return id, nil
}

func (s *Mem) Get(ctx context.Context, id *cadata.ID, salt *cadata.ID, buf []byte) (int, error) {
	ent, err := kv.Get(ctx, s.kv, *id)
	if err != nil {
		if state.IsErrNotFound[cadata.ID](err) {
			return 0, cadata.ErrNotFound{Key: id}
		}
		return 0, err
	}
	return copy(buf, ent.value), nil
}

func (s *Mem) Exists(ctx context.Context, id *cadata.ID) (bool, error) {
	return s.kv.Exists(ctx, *id)
}

func (s *Mem) Delete(ctx context.Context, id *cadata.ID) error {
	return s.kv.Delete(ctx, *id)
}

func (s *Mem) List(ctx context.Context, span cadata.Span, ids []cadata.ID) (int, error) {
	return s.kv.List(ctx, span, ids)
}

func (s *Mem) All() (ret []cadata.ID) {
	kv.ForEach(context.TODO(), s.kv, state.TotalSpan[cadata.ID](), func(i cadata.ID) error {
		ret = append(ret, i)
		return nil
	})
	return ret
}

func (s *Mem) Len() int {
	return s.kv.Len()
}

func cloneSalt(salt *cadata.ID) *cadata.ID {
	if salt == nil {
		return nil
	}
	ret := *salt
	return &ret
}

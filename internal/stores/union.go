package stores

import (
	"context"
	"errors"

	"myceliumweb.org/mycelium/internal/cadata"
)

type GetExister interface {
	cadata.Getter
	cadata.Exister
}

type Union []cadata.Getter

func (s Union) Get(ctx context.Context, id, salt *cadata.ID, buf []byte) (int, error) {
	for _, s2 := range s {
		n, err := s2.Get(ctx, id, salt, buf)
		if errors.As(err, &cadata.ErrNotFound{}) {
			continue
		}
		return n, err
	}
	return 0, cadata.ErrNotFound{Key: id}
}

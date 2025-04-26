package stores

import (
	"context"

	"myceliumweb.org/mycelium/internal/cadata"
)

type CoW struct {
	Write cadata.Store
	Read  cadata.Getter
}

func (s CoW) Post(ctx context.Context, tag *cadata.ID, data []byte) (cadata.ID, error) {
	return s.Write.Post(ctx, tag, data)
}

func (s CoW) Delete(ctx context.Context, id *cadata.ID) error {
	return s.Write.Delete(ctx, id)
}

func (s CoW) Get(ctx context.Context, id, salt *cadata.ID, buf []byte) (int, error) {
	return Union{s.Write, s.Read}.Get(ctx, id, salt, buf)
}

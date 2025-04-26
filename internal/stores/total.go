package stores

import (
	"context"

	"myceliumweb.org/mycelium/internal/cadata"
)

var _ cadata.PostExister = &Total{}

type Total struct {
	hash    cadata.HashFunc
	maxSize int
}

func NewTotal(hash cadata.HashFunc, maxSize int) *Total {
	return &Total{maxSize: maxSize, hash: hash}
}

func (t Total) Post(ctx context.Context, tag *cadata.ID, data []byte) (cadata.ID, error) {
	return t.hash(tag, data), nil
}

func (t Total) Exists(ctx context.Context, id *cadata.ID) (bool, error) {
	return true, nil
}

func (t Total) Hash(tag *cadata.ID, x []byte) cadata.ID {
	return t.hash(tag, x)
}

func (t Total) MaxSize() int {
	return t.maxSize
}

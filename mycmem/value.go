package mycmem

import (
	"context"
	"iter"
	"reflect"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

// Value is the common interface implemented by all Values
// It mostly serves as a Sum type, but also contains the common method Type(), which all Values must implement
type Value interface {
	Type() Type
	Encode(BitBuf)
	Decode(BitBuf, LoadFunc) error
	PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error

	Components() iter.Seq[Value]
	isValue()
}

type LoadFunc = func(Ref) (Value, error)

// Fingerprint returns a unique 256 bit hash for x.
// Fingerprint is appropriate for use in hash tables.
func Fingerprint(x Value) [32]byte {
	bb := bitbuf.New(x.Type().SizeOf())
	x.Encode(bb)
	if k, ok := x.(*Kind); ok && k.class == spec.TC_Kind {
		return mycelium.Hash(nil, bb.Bytes())
	} else {
		salt := cadata.ID(Fingerprint(x.Type()))
		return mycelium.Hash(&salt, bb.Bytes())
	}
}

func Equal(a, b Value) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch a := a.(type) {
	case *Port:
		if bport, ok := b.(*Port); ok {
			return a.data == bport.data
		} else {
			return false
		}
	case *Kind:
		if bkind, ok := b.(*Kind); ok {
			return *a == *bkind
		} else {
			return false
		}
	}
	// DeepEqual is often faster than waiting for ContentID
	if reflect.DeepEqual(a, b) {
		return true
	}
	return Equal(a.Type(), b.Type()) && ContentID(a) == ContentID(b)
}

func mkRef(x Value) Ref {
	cid := ContentID(x)
	return *NewRef(x.Type(), cid)
}

func ptrTo[T any](x T) *T {
	return &x
}

func yieldAll(yield func(Value) bool, xs ...Value) {
	for _, x := range xs {
		if !yield(x) {
			return
		}
	}
}

var emptyIter = func(yield func(Value) bool) {}

type BitBuf = bitbuf.Buf

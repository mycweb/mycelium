package mycelium

import (
	"context"

	"blobcache.io/blobcache/src/bcsdk"
	"blobcache.io/blobcache/src/blobcache"
	"lukechampine.com/blake3"

	"myceliumweb.org/mycelium/spec"
)

const (
	// SizeBits is the number of bits needed to represent:
	// - the maximum length of an array
	// - the maximum size of an expression
	// - the maximum length (number of fields) in product or sum
	SizeBits = spec.SizeBits

	RefBits  = spec.RefBits
	PortBits = spec.PortBits

	MaxSizeBits  = 1 << 24
	MaxSizeBytes = MaxSizeBits / 8
)

type (
	// CID is a Content ID
	CID = blobcache.CID

	Store   = bcsdk.RW
	Getter  = bcsdk.RO
	Poster  = bcsdk.WO
	Exister = bcsdk.Exists

	PostExister = bcsdk.WO
	GetExister  = bcsdk.RO
)

// Hash calculates the hash of x.
// If tag == nil, then the hash is unkeyed.
// If tag != nil, then the hash will be keyed with the tag.
func Hash(tag *CID, x []byte) (ret CID) {
	var key []byte
	if tag != nil {
		key = tag[:]
	}
	h := blake3.New(32, key)
	h.Write(x)
	h.Sum(ret[:0])
	return ret
}

// KHashFunc is a keyed hash function
type KHashFunc = func(salt *CID, data []byte) CID

// RO is the read only store interface
type RO interface {
	Get(ctx context.Context, cid CID, salt *CID, buf []byte) (int, error)
	bcsdk.Exists
	KeyedHash(salt *CID, data []byte) CID
}

type WO interface {
	Post(ctx context.Context, salt *CID, data []byte) (CID, error)
	bcsdk.Exists
	KeyedHash(salt *CID, data []byte) CID
}

type RW interface {
	RO
	WO
}

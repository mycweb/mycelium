package mycelium

import (
	"lukechampine.com/blake3"

	"myceliumweb.org/mycelium/internal/cadata"
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
	CID = cadata.ID

	Store   = cadata.Store
	Getter  = cadata.Getter
	Poster  = cadata.Poster
	Exister = cadata.Exister

	PostExister = cadata.PostExister
	GetExister  = cadata.GetExister
)

// Hash calculates the hash of x.
// If tag == nil, then the hash is unkeyed.
// If tag != nil, then the hash will be keyed with the tag.
func Hash(tag *cadata.ID, x []byte) (ret cadata.ID) {
	var key []byte
	if tag != nil {
		key = tag[:]
	}
	h := blake3.New(32, key)
	h.Write(x)
	h.Sum(ret[:0])
	return ret
}

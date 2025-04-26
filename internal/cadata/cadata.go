// package cadata provides interfaces for Content Addressed Data Storage
//
// It is based on the package in go.brendoncarroll.net/state/cadata
// This provides "salted" stores, instead of simple content addressed stores.
package cadata

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"go.brendoncarroll.net/state"
	"go.brendoncarroll.net/state/kv"
)

var _ driver.Value = ID{}

const (
	IDSize = 32
	// Base64Alphabet is used when encoding IDs as base64 strings.
	// It is a URL and filepath safe encoding, which maintains ordering.
	Base64Alphabet = "-0123456789" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "_" + "abcdefghijklmnopqrstuvwxyz"
)

// ID identifies a particular piece of data
type ID [IDSize]byte

func IDFromBytes(x []byte) ID {
	id := ID{}
	copy(id[:], x)
	return id
}

var enc = base64.NewEncoding(Base64Alphabet).WithPadding(base64.NoPadding)

func (id ID) String() string {
	return enc.EncodeToString(id[:])
}

// MarshalBase64 encodes ID using Base64Alphabet
func (id ID) MarshalBase64() ([]byte, error) {
	buf := make([]byte, enc.EncodedLen(len(id)))
	enc.Encode(buf, id[:])
	return buf, nil
}

// UnmarshalBase64 decodes data into the ID using Base64Alphabet
func (id *ID) UnmarshalBase64(data []byte) error {
	n, err := enc.Decode(id[:], data)
	if err != nil {
		return err
	}
	if n != IDSize {
		return errors.New("base64 string is too short")
	}
	return nil
}

func (a ID) Equals(b ID) bool {
	return a.Compare(b) == 0
}

func (a ID) Compare(b ID) int {
	return bytes.Compare(a[:], b[:])
}

func (id ID) IsZero() bool {
	return id == (ID{})
}

func (id ID) MarshalJSON() ([]byte, error) {
	s := enc.EncodeToString(id[:])
	return json.Marshal(s)
}

func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	_, err := enc.Decode(id[:], []byte(s))
	return err
}

func (id *ID) Scan(x interface{}) error {
	switch x := x.(type) {
	case []byte:
		if len(x) != 32 {
			return fmt.Errorf("wrong length for cadata.ID HAVE: %d WANT: %d", len(x), IDSize)
		}
		*id = IDFromBytes(x)
		return nil
	default:
		return fmt.Errorf("cannot scan type %T", x)
	}
}

func (id ID) Value() (driver.Value, error) {
	return id[:], nil
}

// Successor returns the ID immediately after this ID
func (id ID) Successor() ID {
	for i := len(id) - 1; i >= 0; i-- {
		id[i]++
		if id[i] != 0 {
			break
		}
	}
	return id
}

type HashFunc = func(salt *ID, x []byte) ID

type Poster interface {
	Post(ctx context.Context, salt *ID, data []byte) (ID, error)
}

type Getter interface {
	Get(ctx context.Context, k *ID, salt *ID, buf []byte) (int, error)
}

type Exister interface {
	Exists(ctx context.Context, k *ID) (bool, error)
}

type Deleter interface {
	Delete(ctx context.Context, k *ID) error
}

type Adder interface {
	Add(ctx context.Context, k *ID) error
}

type Span = state.Span[ID]

type Lister interface {
	List(ctx context.Context, span Span, ids []ID) (int, error)
}

type PostExister interface {
	Poster
	Exister
}

type GetExister interface {
	Getter
	Exister
}

type Store interface {
	Poster
	Getter
	Exister
	Deleter
}

func ForEach(ctx context.Context, x Lister, span Span, fn func(ID) error) error {
	return kv.ForEach[ID](ctx, x, span, fn)
}

var (
	ErrTooLarge = errors.New("data is too large for store")
)

type ErrNotFound struct {
	Key *ID
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("no data found for %v in store", e.Key)
}

type ErrBadData struct {
	Have ID
	Want ID
}

func (e ErrBadData) Error() string {
	return fmt.Sprintf("bad data. HAVE: %v WANT: %v", e.Have, e.Want)
}

func Check(hf HashFunc, expectedID *ID, salt *ID, data []byte) error {
	actualID := hf(salt, data)
	if subtle.ConstantTimeCompare(actualID[:], expectedID[:]) != 1 {
		return ErrBadData{Have: actualID, Want: *expectedID}
	}
	return nil
}

func BeginFromSpan(x Span) ID {
	lb, ok := x.LowerBound()
	if !ok {
		return ID{}
	}
	if !x.IncludesLower() {
		lb = lb.Successor()
	}
	return lb
}

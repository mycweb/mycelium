// package mycbytes contains Mycelium types defined in terms of byte arrays.
// This library is more suitable than mycmem, when the values may not necessarily
// fit into memory.
package mycbytes

import (
	"context"
	"encoding/binary"
	"fmt"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type Ref [spec.RefBits / 8]byte

func (r *Ref) CID() cadata.ID {
	return cadata.ID(r[:])
}

type Expr [spec.ExprBits / 8]byte

func (e *Expr) GetProgRef() Ref {
	return Ref(e[0 : spec.RefBits/8])
}

func (e *Expr) GetProgLen() uint32 {
	return binary.LittleEndian.Uint32(e[spec.RefBits/8:])
}

type List [spec.ListBits / 8]byte

func (l *List) GetArrayRef() Ref {
	return Ref(l[0 : spec.RefBits/8])
}

func (l *List) GetArrayLen() uint32 {
	return binary.LittleEndian.Uint32(l[spec.RefBits/8:])
}

type Lazy [spec.LazyBits / 8]byte

func (la *Lazy) GetProgRef() Ref {
	return (*Expr)(la).GetProgRef()
}

func (la *Lazy) GetProgLen() uint32 {
	return (*Expr)(la).GetProgLen()
}

type Lambda [spec.LambdaBits / 8]byte

func (la *Lambda) GetProgRef() Ref {
	return (*Expr)(la).GetProgRef()
}

func (la *Lambda) GetProgLen() uint32 {
	return (*Expr)(la).GetProgLen()
}

type AnyType [spec.AnyTypeBits / 8]byte

func (at *AnyType) GetKind() Kind {
	return Kind(at[0:4])
}

type AnyValue [spec.AnyValueBits / 8]byte

func (av *AnyValue) GetAnyType() AnyType {
	return AnyType(av[0 : spec.AnyTypeBits/8])
}

func (av *AnyValue) GetRef() Ref {
	return Ref(av[spec.AnyTypeBits/8:])
}

func Load(ctx context.Context, s cadata.Getter, ref Ref, dst []byte) error {
	cid := ref.CID()
	n, err := s.Get(ctx, &cid, nil, dst)
	if err != nil {
		return err
	}
	if n != len(dst) {
		return fmt.Errorf("mycbytes.Load: read wrong number of bytes")
	}
	return nil
}

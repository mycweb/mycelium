package mycmem

import (
	"context"
	"encoding/base64"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"sync"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/spec"
)

const (
	RefBits  = spec.RefBits
	RefBytes = RefBits / 8
)

type RefType struct {
	elem Type
}

func NewRefType(elem Type) *RefType {
	if elem == nil {
		panic("NewRefType(nil)")
	}
	elem = unwrapAnyType(elem)
	return &RefType{elem: elem}
}

func (*RefType) isValue() {}
func (*RefType) isType()  {}

func (*RefType) Type() Type {
	return RefKind()
}

func (*RefType) SizeOf() int {
	return RefBits
}

func (r *RefType) Elem() Type {
	return r.elem
}

func (t *RefType) Supersets(x Type) bool {
	t2, ok := x.(*RefType)
	if !ok {
		return false
	}
	return Supersets(t.Elem(), t2.Elem())
}

func (t *RefType) Unmake() Value {
	return Product{NewAnyType(t.elem)}
}

func (t *RefType) String() string {
	return fmt.Sprintf("Ref[%v]", t.elem)
}

func (rt *RefType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return NewAnyType(rt.elem).PullInto(ctx, dst, src)
}

func (t *RefType) Encode(bb BitBuf) {
	t.Unmake().Encode(bb)
}

func (t *RefType) Decode(bb BitBuf, load LoadFunc) error {
	var av AnyType
	if err := av.Decode(bb, load); err != nil {
		return err
	}
	t.elem = av.Unwrap()
	return nil
}

func (rt *RefType) Zero() Value {
	ref := mkRef(rt.elem.Zero())
	return &ref
}

func (rt *RefType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(NewAnyType(rt.elem))
	}
}

type Ref struct {
	elem Type
	cid  cadata.ID
}

func NewRef(elem Type, data [32]byte) *Ref {
	if elem == nil {
		panic("NewRef(nil)")
	}
	return &Ref{
		elem: elem,
		cid:  cadata.ID(data[:32]),
	}
}

func (r *Ref) isValue() {}

func (r *Ref) Type() Type {
	return &RefType{r.elem}
}

func (r *Ref) Data() (ret [32]byte) {
	return r.cid
}

func (r *Ref) ElemType() Type {
	return r.elem
}

func (r *Ref) retype(ty Type) *Ref {
	r2 := *r
	r2.elem = ty
	return &r2
}

func (r *Ref) Bytes() (ret []byte) {
	ret = append(ret, r.cid[:]...)
	return ret
}

func (r Ref) String() string {
	enc := Base64Encoding()
	return fmt.Sprintf("@%s", enc.EncodeToString(r.Bytes()))
}

func (r *Ref) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	if yes, err := dst.Exists(ctx, &r.cid); err != nil {
		return err
	} else if yes {
		// This is where we avoid repeated work.
		return nil
	}
	val, err := Load(ctx, src, *r)
	if err != nil {
		return err
	}
	if err := val.PullInto(ctx, dst, src); err != nil {
		return err
	}
	if _, err := Post(ctx, dst, val); err != nil {
		return err
	}
	return nil
}

func (r *Ref) Encode(bb BitBuf) {
	bb.PutBytes(0, r.cid[:])
}

func (ref *Ref) Decode(bb BitBuf, ub LoadFunc) error {
	bb.GetBytes(0, RefBits, ref.cid[:])
	return nil
}

func (r *Ref) Components() iter.Seq[Value] {
	return emptyIter
}

func Base64Encoding() *base64.Encoding {
	return base64.NewEncoding(cadata.Base64Alphabet).WithPadding(base64.NoPadding)
}

// listRefs returns all the references contained in Value.
func forEachRef(v Value, fn func(ref *Ref) error) error {
	worklist := []Value{v}
	for len(worklist) > 0 {
		l := len(worklist)
		x := worklist[l-1]
		worklist = worklist[:l-1]
		if ref, ok := x.(*Ref); ok {
			if err := fn(ref); err != nil {
				return err
			}
		} else {
			worklist = slices.AppendSeq(worklist, x.Components())
		}
	}
	return nil
}

func pullIntoBatch(ctx context.Context, dst cadata.PostExister, src cadata.Getter, vals ...Value) error {
	for _, val := range vals {
		if err := val.PullInto(ctx, dst, src); err != nil {
			return err
		}
	}
	return nil
}

// Post marshals v, and adds it to the store.
// If v contains Refs, post checks that they exist, before adding v.
// If ty == nil, then v.Type() is assumed as a default.
// The returned Ref will always have ty as an element type if ty != nil
func Post(ctx context.Context, s cadata.PostExister, v Value) (Ref, error) {
	cid := ContentID(v)
	if exists, err := s.Exists(ctx, &cid); err == nil && exists {
		return *NewRef(v.Type(), cid), nil
	}
	if err := v.PullInto(ctx, s, stores.Union{}); err != nil {
		return Ref{}, err
	}

	// check that all the deps exist.
	if err := forEachRef(v, func(ref *Ref) error {
		if exists, err := s.Exists(ctx, &ref.cid); err != nil {
			return err
		} else if !exists && !reflect.DeepEqual(v, ref) {
			return ErrDanglingRef{Value: v, Ref: ref}
		}
		return nil
	}); err != nil {
		return Ref{}, err
	}

	// encode and post to store
	ty := v.Type()
	if ft, ok := ty.(*FractalType); ok {
		ty = ft.Expanded()
	}
	typeID := saltForValueOfType(ty)
	buf := bitbuf.New(ty.SizeOf())
	v.Encode(buf)

	cid, err := s.Post(ctx, typeID, buf.Bytes())
	if err != nil {
		return Ref{}, err
	}
	return Ref{
		cid:  cid,
		elem: ty,
	}, nil
}

// Load loads a Mycelium value from storage.
func Load(ctx context.Context, src cadata.Getter, ref Ref) (Value, error) {
	ty := ref.ElemType()
	if tt, ok := ty.(*FractalType); ok {
		ty = tt.expanded
	}
	salt := saltForValueOfType(ty)
	buf := acquireBuffer()
	defer releaseBuffer(buf)
	n, err := src.Get(ctx, &ref.cid, salt, buf[:])
	if err != nil {
		return nil, err
	}
	if n*8 < ty.SizeOf() {
		return nil, fmt.Errorf("load: got short value HAVE: %d, WANT: %d", n*8, ty.SizeOf())
	}
	data := buf[:n]
	bb := bitbuf.FromBytes(data).Slice(0, ty.SizeOf())
	load := func(ref Ref) (Value, error) {
		return Load(ctx, src, ref)
	}
	val := ty.Zero()
	if err := val.Decode(bb, load); err != nil {
		return nil, err
	}
	return val, nil
}

// ContentID returns the Content-ID or the Hash from content-addressed storage.
// Values that only contain bits may have the same ContentID even if they are different types.
// This is to improve convergence and deduplication.
// ContentID will always match the value returned by Post
func ContentID(x Value) cadata.ID {
	bb := bitbuf.New(int(x.Type().SizeOf()))
	x.Encode(bb)
	salt := saltForValueOfType(x.Type())
	return mycelium.Hash(salt, bb.Bytes())
}

func saltForValueOfType(x Type) *cadata.ID {
	if !requiresSalt(x) {
		return nil
	}
	ret := ContentID(x)
	return &ret
}

// requiresSalt returns true if values of x could contain a Ref
func requiresSalt(x Type) bool {
	switch x := x.(type) {
	case *Kind:
		kc := x.TypeCode()
		switch kc {
		case spec.TC_Kind, spec.TC_Bit, spec.TC_Prog, spec.TC_AnyType, spec.TC_AnyValue:
			return false
		case spec.TC_Array,
			spec.TC_Ref,
			spec.TC_Lambda,
			spec.TC_Lazy,
			spec.TC_List,
			spec.TC_Fractal,
			spec.TC_Port,
			spec.TC_Distinct:
			return true
		case spec.TC_Sum, spec.TC_Product:
			return x.data > 0
		default:
			panic(kc)
		}
	case BitType:
		// Bits do not contain refs
		return false
	case *ArrayType:
		// depends on element type
		return requiresSalt(x.Elem())
	case *ProgType:
		// Expr can contain literals of any value
		return true
	case *RefType:
		// trivial case
		return true
	case SumType:
		// depends on any of the elements
		return forAny(x, requiresSalt)
	case ProductType:
		// depends on any of the elements
		return forAny(x, requiresSalt)
	case *ListType:
		// Lists are implemented as Product[Ref, Size]
		return true
	case *LazyType:
		// Lazy is implemented as a Product[Ref, Size]
		return true
	case *LambdaType:
		// Lambda is implemented as a Product[Ref, Size]
		return true
	case *FractalType:
		// FractalType is implemented as a Product[Ref, Size]
		return true
	case *PortType:
		// Ports do not contain refs
		return false
	case *DistinctType:
		// distinct types are only nominally different from their base type, so defer to base.
		return requiresSalt(x.Base())
	case AnyProgType:
		// ExprType is implemented as Product[Ref, ProgType]
		return true
	case AnyTypeType:
		// AnyType is implemented as Product[Ref, Kind]
		return true
	case AnyValueType:
		// AnyValue is implemented as Product[Ref, AnyType]
		return true
	case *AnyType:
		return requiresSalt(x.Unwrap())
	default:
		// anything not considered is a bug
		panic(x)
	}
}

var bufPool = sync.Pool{
	New: func() any {
		return new([mycelium.MaxSizeBytes]byte)
	},
}

func acquireBuffer() *[mycelium.MaxSizeBytes]byte {
	return bufPool.Get().(*[mycelium.MaxSizeBytes]byte)
}

func releaseBuffer(x *[mycelium.MaxSizeBytes]byte) {
	bufPool.Put(x)
}

package mvm1

import (
	"context"
	"fmt"
	"math/bits"

	"myceliumweb.org/mycelium"

	"myceliumweb.org/mycelium/internal/cadata"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

// Type contains info about a Type
// It is a representation used by the compiler, and is not represented in the VM.
type Type struct {
	Type2 Type2
	Ref   Ref
	Data  []Word
	Size  int
}

func (t *Type) SizeWords() int {
	return AlignSize(t.Size) / WordBits
}

func (t *Type) AsAnyType() (ret AnyType) {
	ret[len(ret)-1] = t.Type2[0]
	copy(ret[:8], t.Ref[:])
	return ret
}

func (t *Type) Equals(x *Type) bool {
	return t.Type2 == x.Type2 && t.Ref == x.Ref
}

func (t *Type) IsAnyTypeType() bool {
	return t.Type2.TypeCode() == spec.TC_AnyType
}

func (t *Type) IsAnyValueType() bool {
	return t.Type2.TypeCode() == spec.TC_AnyValue
}

func (t Type) IsZero() bool {
	return t.Ref == (Ref{})
}

func (t Type) Fingerprint() Fingerprint {
	t2 := newType2(spec.TC_Kind, 0)
	return fingerprint(t2, t.Type2[:], t.Data, len(t.Data)*WordBits)
}

func (t Type) String() string {
	return fmt.Sprintf("Type{%v: %v}", t.Type2.TypeCode(), t.Data)
}

// evalTypeNow evaluates a type expression at compile time
func (c *Compiler) evalTypeNow(ctx context.Context, mc machCtx, xE myc.Prog) (Type, error) {
	x, err := c.compile(ctx, mc, xE)
	if err != nil {
		return Type{}, err
	}
	if !x.IsType() {
		return Type{}, fmt.Errorf("not an type: %v %v", x.Type.Type2, x.Type.Data)
	}
	val, err := c.evalNow(ctx, x, &x.Type)
	if err != nil {
		return Type{}, err
	}
	// If the value is an AnyType, then load it.
	if x.Type.Type2.TypeCode() == spec.TC_AnyType {
		return c.loadType(ctx, AnyType(val))
	}
	// otherwise we have the full type data in val
	return c.makeType(ctx, Type2(x.Type.Data), val)
}

func (c *Compiler) supersets(ctx context.Context, a, b *Type) (bool, error) {
	// everything supersets bottom
	if b.Type2.TypeCode() == spec.TC_Sum && b.Type2.Data() == 0 {
		return true, nil
	}
	if a.Size != b.Size {
		// shortcut, if the sizes are different there's no way a >= b
		return false, nil
	}
	// Equal types superset one another
	if a.Equals(b) {
		return true, nil
	}
	// expand FractalTypes
	for _, ty := range []**Type{&a, &b} {
		if (*ty).Type2.TypeCode() == spec.TC_Fractal {
			ty2, err := c.expandFractalType(ctx, FractalType(a.Data))
			if err != nil {
				return false, err
			}
			*ty = &ty2
		}
	}
	// at this point, after fractals have been expanded, a mismatched Type2 will be false
	if a.Type2.TypeCode() != b.Type2.TypeCode() {
		return false, nil
	}
	allTypesSuperset := func(n int) (bool, error) {
		for i := 0; i < n; i++ {
			aElem, err := c.getTypeParam(ctx, a, i)
			if err != nil {
				return false, err
			}
			bElem, err := c.getTypeParam(ctx, b, i)
			if err != nil {
				return false, err
			}
			ss, err := c.supersets(ctx, &aElem, &bElem)
			if err != nil {
				return false, err
			}
			if !ss {
				return false, nil
			}
		}
		return true, nil
	}
	switch a.Type2.TypeCode() {
	case spec.TC_Array:
		// TODO: length
		yes, err := allTypesSuperset(1)
		if err != nil {
			return false, err
		}
		if !yes {
			return false, nil
		}
		return a.Data[8] == b.Data[8], nil
	case spec.TC_Ref, spec.TC_Lazy, spec.TC_List:
		return allTypesSuperset(1)
	case spec.TC_Lambda:
		return allTypesSuperset(2)
	case spec.TC_Sum:
		n := int(a.Type2.Data())
		return allTypesSuperset(n)
	case spec.TC_Product:
		n := int(a.Type2.Data())
		return allTypesSuperset(n)
	case spec.TC_Distinct:
		yes, err := allTypesSuperset(1)
		if err != nil {
			return false, err
		}
		if !yes {
			return false, nil
		}
		aMark := AnyValue(a.Data[AnyTypeBits/WordBits:])
		bMark := AnyValue(b.Data[AnyTypeBits/WordBits:])
		return aMark == bMark, nil
	}
	return false, nil
}

func (c *Compiler) checkSupersets(ctx context.Context, a, b *Type) error {
	yes, err := c.supersets(ctx, a, b)
	if err != nil {
		return err
	}
	if !yes {
		return fmt.Errorf("%v does not superset %v", a, b)
	}
	return nil
}

func (c *Compiler) getTypeParam(ctx context.Context, x *Type, i int) (Type, error) {
	switch x.Type2.TypeCode() {
	case spec.TC_Array, spec.TC_List, spec.TC_Lazy, spec.TC_Ref, spec.TC_Distinct:
		if i > 0 {
			panic(x.Type2.TypeCode())
		}
	case spec.TC_Lambda:
		if i > 1 {
			panic(i)
		}
	case spec.TC_Port:
		if i > 3 {
			panic(i)
		}
	case spec.TC_Product, spec.TC_Sum:
		n := int(x.Type2.Data())
		if i >= n {
			return Type{}, fmt.Errorf("getTypeParam i=%d on Product or Sum arity=%d", i, n)
		}
	default:
		panic(x)
	}
	beg := i * AnyTypeBits / WordBits
	end := beg + AnyTypeBits/WordBits
	at := AnyType(x.Data[beg:end])
	return c.loadType(ctx, at)
}

func (c *Compiler) sumTagOffset(ctx context.Context, x *Type) (int, error) {
	st := SumType(x.Data)
	return sumTagOffset(ctx, c.store, st)
}

func (c *Compiler) productOffsets(ctx context.Context, x *Type) ([]int, error) {
	if x.Type2.TypeCode() != spec.TC_Product {
		return nil, fmt.Errorf("%v is not a ProductType", x)
	}
	if offsets, exists := c.prodOffCache[x.Fingerprint()]; exists {
		return offsets, nil
	}
	pt := ProductType(x.Data)
	offsets, err := productOffsets(ctx, c.store, pt)
	if err != nil {
		return nil, err
	}
	if c.prodOffCache == nil {
		c.prodOffCache = make(map[Fingerprint][]int)
	}
	c.prodOffCache[x.Fingerprint()] = offsets
	return offsets, nil
}

func (c *Compiler) bitType() Type {
	cid := mycelium.Hash(nil, nil)
	t2 := newType2(spec.TC_Bit, 0)
	return Type{
		Type2: t2,
		Data:  nil,
		Ref:   RefFromCID(cid),
		Size:  1,
	}
}

func (c *Compiler) sizeType(ctx context.Context) (Type, error) {
	return c.arrayType(ctx, c.bitType(), 32)
}

// arrayType creates a new arrayType
func (c *Compiler) arrayType(ctx context.Context, elem Type, l int) (Type, error) {
	var ws []Word
	elemAt := elem.AsAnyType()
	ws = append(ws, elemAt[:]...)
	ws = append(ws, Word(l))
	t2 := newType2(spec.TC_Array, 0)
	return c.makeType(ctx, t2, ws)
}

func (c *Compiler) refType(ctx context.Context, elem Type) (Type, error) {
	var ws []Word
	elemAt := elem.AsAnyType()
	ws = append(ws, elemAt[:]...)
	t2 := newType2(spec.TC_Ref, 0)
	return c.makeType(ctx, t2, ws)
}

func (c *Compiler) makeType(ctx context.Context, t2 Type2, data []Word) (Type, error) {
	wsize := AlignSize(t2.SizeOf()) / WordBits
	if wsize != len(data) {
		return Type{}, fmt.Errorf("wrong len of data provided to create value of code=%v data=%v HAVE: %v WANT: %d", t2.TypeCode(), t2.Data(), data, wsize)
	}
	// choose salt
	var salt *Ref
	switch t2.TypeCode() {
	case spec.TC_Kind, spec.TC_Bit, spec.TC_AnyType, spec.TC_AnyValue:
	default:
		// Every other type encodes AnyValues or AnyTypes, which both contain references.
		if wsize >= RefBits/WordBits {
			at, err := t2.PostAnyType(ctx, c.store)
			if err != nil {
				return Type{}, err
			}
			salt = new(Ref)
			*salt = at.GetRef()
		}
	}
	ref, err := c.postWords(ctx, salt, t2.SizeOf(), data)
	if err != nil {
		return Type{}, err
	}
	ret := Type{
		Type2: t2,
		Ref:   ref,
		Data:  data,
	}
	size, err := SizeOf(ctx, c.store, ret.AsAnyType())
	if err != nil {
		return Type{}, err
	}
	ret.Size = size
	return ret, nil
}

// makeHType makes a higher order type
func (c *Compiler) makeHType(ctx context.Context, kc spec.TypeCode, elems []Type) (Type, error) {
	// assemble data
	var ws []Word
	for _, elem := range elems {
		elemAt := elem.AsAnyType()
		ws = append(ws, elemAt[:]...)
	}
	// type2
	t2 := newType2(kc, uint32(len(elems)))
	return c.makeType(ctx, t2, ws)
}

// distinctType creates a new DistinctType
func (c *Compiler) distinctType(ctx context.Context, base AnyType, mark AnyValue) (Type, error) {
	var ws []Word
	ws = append(ws, base[:]...)
	ws = append(ws, mark[:]...)
	buf := make([]byte, len(ws)*WordBits/8)

	var salt cadata.ID
	cid, err := c.store.Post(ctx, &salt, buf)
	if err != nil {
		return Type{}, err
	}
	ref := RefFromCID(cid)
	t2 := newType2(spec.TC_Distinct, 0)

	ret := Type{
		Type2: t2,
		Ref:   ref,
		Data:  ws,
	}
	size, err := SizeOf(ctx, c.store, ret.AsAnyType())
	if err != nil {
		return Type{}, err
	}
	ret.Size = size
	return ret, nil
}

func (c *Compiler) anyTypeType(ctx context.Context) (Type, error) {
	return c.makeHType(ctx, spec.TC_AnyType, nil)
}

func (c *Compiler) anyValueType(ctx context.Context) (Type, error) {
	return c.makeHType(ctx, spec.TC_AnyValue, nil)
}

// loadType loads a Type from an AnyType
func (c *Compiler) loadType(ctx context.Context, at AnyType) (Type, error) {
	size, err := SizeOf(ctx, c.store, at)
	if err != nil {
		return Type{}, err
	}
	t2 := at.GetType()
	ref := at.GetRef()
	words := make([]Word, t2.SizeOf()/WordBits)
	if err := c.loadWords(ctx, ref, words); err != nil {
		return Type{}, err
	}
	return Type{
		Type2: t2,
		Ref:   ref,
		Data:  words,
		Size:  size,
	}, nil
}

func (c *Compiler) expandFractalType(ctx context.Context, ft FractalType) (Type, error) {
	body, err := loadExpr(ctx, c.store, ft.GetRef(), ft.GetExprType())
	if err != nil {
		return Type{}, err
	}
	ft2, err := myc.NewFractalType(body)
	if err != nil {
		return Type{}, err
	}
	data := myc.MarshalAppend(nil, myc.NewAnyType(ft2.Expanded()))
	var expandedAT AnyType
	expandedAT.FromBytes(data)
	return c.loadType(ctx, expandedAT)
}

// SizeOf returns the Size of a type passed inside an AnyType
func SizeOf(ctx context.Context, s cadata.Getter, x AnyType) (int, error) {
	kc := x.GetType().TypeCode()
	switch kc {
	case spec.TC_Kind:
		var t2 Type2
		if err := Load(ctx, s, x.GetRef(), &t2); err != nil {
			return 0, err
		}
		return t2.SizeOf(), nil
	case spec.TC_Bit:
		return BitBits, nil
	case spec.TC_Array:
		var at ArrayType
		if err := Load(ctx, s, x.GetRef(), &at); err != nil {
			return 0, err
		}
		elemSize, err := SizeOf(ctx, s, at.Elem())
		if err != nil {
			return 0, err
		}
		return at.Len() * elemSize, nil
	case spec.TC_Prog:
		var et ProgType
		if err := Load(ctx, s, x.GetRef(), &et); err != nil {
			return 0, err
		}
		return et.SizeOf(), nil
	case spec.TC_Ref:
		return RefBits, nil
	case spec.TC_Sum:
		st := makeSumType(int(x.GetType().Data()))
		if err := Load(ctx, s, x.GetRef(), &st); err != nil {
			return 0, err
		}
		var ret int
		for i := 0; i < st.Len(); i++ {
			size2, err := SizeOf(ctx, s, st.At(i))
			if err != nil {
				return 0, err
			}
			ret = max(ret, size2)
		}
		if st.Len() > 1 {
			ret += bits.Len64(uint64(st.Len()))
		}
		return ret, nil
	case spec.TC_Product:
		pt := makeProductType(int(x.GetType().Data()))
		if err := Load(ctx, s, x.GetRef(), &pt); err != nil {
			return 0, err
		}
		var ret int
		for i := 0; i < pt.Len(); i++ {
			size2, err := SizeOf(ctx, s, pt.At(i))
			if err != nil {
				return 0, err
			}
			ret += size2
		}
		return ret, nil
	case spec.TC_List:
		return ListBits, nil
	case spec.TC_Lazy:
		return LazyBits, nil
	case spec.TC_Lambda:
		return LambdaBits, nil
	case spec.TC_Fractal:
		var ft FractalType
		if err := loadWords(ctx, s, x.GetRef(), ft[:]); err != nil {
			return 0, err
		}
		body, err := loadExpr(ctx, s, ft.GetRef(), ft.GetExprType())
		if err != nil {
			return 0, err
		}
		ft2, err := myc.NewFractalType(body)
		if err != nil {
			return 0, err
		}
		return ft2.SizeOf(), nil
	case spec.TC_Port:
		return PortBits, nil
	case spec.TC_Distinct:
		var at DistinctType
		if err := Load(ctx, s, x.GetRef(), &at); err != nil {
			return 0, err
		}
		return SizeOf(ctx, s, at.Base())
	case spec.TC_AnyType:
		return AnyTypeBits, nil
	case spec.TC_AnyValue:
		return AnyValueBits, nil
	default:
		panic(kc)
	}
}

// NeedsSalt returns true if values of type ty need a salt when posting
func NeedsSalt(ctx context.Context, s cadata.Getter, x AnyType) (bool, error) {
	kc := x.GetType().TypeCode()
	switch kc {
	case spec.TC_Kind:
		var t2 Type2
		if err := Load(ctx, s, x.GetRef(), &t2); err != nil {
			return false, err
		}
		return t2.NeedsSalt(), nil
	case spec.TC_Bit:
		return false, nil
	case spec.TC_Array:
		var at ArrayType
		if err := Load(ctx, s, x.GetRef(), &at); err != nil {
			return false, err
		}
		if at.Len() == 0 {
			return false, nil
		}
		return NeedsSalt(ctx, s, at.Elem())
	case spec.TC_Sum:
		st := makeSumType(int(x.GetType().Data()))
		if err := Load(ctx, s, x.GetRef(), &st); err != nil {
			return false, err
		}
		for i := 0; i < st.Len(); i++ {
			yes, err := NeedsSalt(ctx, s, st.At(i))
			if err != nil {
				return false, err
			}
			if yes {
				return true, nil
			}
		}
		return false, nil
	case spec.TC_Product:
		pt := makeProductType(int(x.GetType().Data()))
		if err := Load(ctx, s, x.GetRef(), &pt); err != nil {
			return false, err
		}
		for i := 0; i < pt.Len(); i++ {
			yes, err := NeedsSalt(ctx, s, pt.At(i))
			if err != nil {
				return false, err
			}
			if yes {
				return true, nil
			}
		}
		return false, nil
	case spec.TC_Distinct:
		var dt DistinctType
		if err := Load(ctx, s, x.GetRef(), &dt); err != nil {
			return false, err
		}
		return NeedsSalt(ctx, s, dt.Base())
	case spec.TC_Port:
		return false, nil
	default:
		return true, nil
	}
}

func sumTagOffset(ctx context.Context, s cadata.Store, st SumType) (ret int, _ error) {
	for i := 0; i < st.Len(); i++ {
		size, err := SizeOf(ctx, s, st.At(i))
		if err != nil {
			return 0, err
		}
		ret = max(ret, size)
	}
	return ret, nil
}

// productOffsets calculates the offsets in bits for each member of the product
func productOffsets(ctx context.Context, s cadata.Store, pt ProductType) ([]int, error) {
	var acc int
	ret := make([]int, pt.Len())
	for i := range ret {
		ret[i] = acc
		size, err := SizeOf(ctx, s, pt.At(i))
		if err != nil {
			return nil, err
		}
		acc += size
	}
	return ret, nil
}

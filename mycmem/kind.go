package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"

	"go.brendoncarroll.net/exp/slices2"
)

type Kind struct {
	class spec.TypeCode
	// data is 24 bits of type dependent data
	data uint32
}

func BitKind() *Kind {
	return &Kind{class: spec.TC_Bit}
}

func ArrayKind() *Kind {
	return &Kind{class: spec.TC_Array}
}

func ListKind() *Kind {
	return &Kind{class: spec.TC_List}
}

func RefKind() *Kind {
	return &Kind{class: spec.TC_Ref}
}

func SumKind(a int) *Kind {
	if a > low24BitMask {
		panic(a)
	}
	return &Kind{class: spec.TC_Sum, data: uint32(a)}
}

func ProductKind(a int) *Kind {
	if a > low24BitMask {
		panic(a)
	}
	return &Kind{class: spec.TC_Product, data: uint32(a)}
}

func ProgKind() *Kind {
	return &Kind{class: spec.TC_Prog}
}

func LazyKind() *Kind {
	return &Kind{class: spec.TC_Lazy}
}

func LambdaKind() *Kind {
	return &Kind{class: spec.TC_Lambda}
}

func DistinctKind() *Kind {
	return &Kind{class: spec.TC_Distinct}
}

func PortKind() *Kind {
	return &Kind{class: spec.TC_Port}
}

func KindKind() *Kind {
	return &Kind{class: spec.TC_Kind}
}

func FractalKind() *Kind {
	return &Kind{class: spec.TC_Fractal}
}

func AnyProgKind() *Kind {
	return &Kind{class: spec.TC_AnyProg}
}

func AnyValueKind() *Kind {
	return &Kind{class: spec.TC_AnyValue}
}

func AnyTypeKind() *Kind {
	return &Kind{class: spec.TC_AnyType}
}

func (*Kind) isValue() {}
func (*Kind) isType()  {}

// BitArity is the number of bits that the type contains
func (t *Kind) BitArity() int {
	switch t.class {
	case spec.TC_Array, spec.TC_Prog, spec.TC_Kind:
		return 32
	default:
		return 0
	}
}

func (t *Kind) ValueArity() int {
	switch t.class {
	case spec.TC_Distinct:
		return 1
	default:
		return 0
	}
}

func (t *Kind) TypeArity() int {
	switch t.class {
	case spec.TC_Array, spec.TC_List, spec.TC_Ref, spec.TC_Distinct, spec.TC_Lazy:
		return 1
	case spec.TC_Lambda:
		return 2
	case spec.TC_Port:
		return 4
	case spec.TC_Product, spec.TC_Sum:
		return int(t.data & low24BitMask)
	default:
		return 0
	}
}

func (t *Kind) LazyTypeArity() int {
	switch t.class {
	case spec.TC_Fractal:
		return 1
	default:
		return 0
	}
}

func (t *Kind) Type() Type {
	return KindKind()
}

func (t *Kind) String() string {
	ret := t.class.String()[3:] + "Kind"
	switch t.class {
	case spec.TC_Product, spec.TC_Sum:
		ret += fmt.Sprintf("[%v]", t.data)
	}
	return ret
}

func (t *Kind) SizeOf() int {
	switch t.class {
	case spec.TC_Kind:
		return spec.KindBits
	case spec.TC_Bit:
		return spec.BitTypeBits
	case spec.TC_Array:
		return spec.ArrayTypeBits
	case spec.TC_List:
		return spec.ListTypeBits
	case spec.TC_Ref:
		return spec.RefTypeBits
	case spec.TC_Sum:
		return spec.SumTypeBits(int(t.data))
	case spec.TC_Product:
		return spec.ProductTypeBits(int(t.data))
	case spec.TC_Prog:
		return spec.ProgTypeBits
	case spec.TC_Lazy:
		return spec.LazyTypeBits
	case spec.TC_Lambda:
		return spec.LambdaTypeBits
	case spec.TC_Fractal:
		return spec.FractalTypeBits
	case spec.TC_Port:
		return spec.PortTypeBits
	case spec.TC_Distinct:
		return spec.DistinctTypeBits
	case spec.TC_AnyType:
		return spec.AnyTypeTypeBits
	case spec.TC_AnyValue:
		return spec.AnyValueTypeBits
	default:
		panic(t.class)
	}
}

func (t *Kind) TypeCode() spec.TypeCode {
	return t.class
}

func (k *Kind) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (k *Kind) Encode(dstBuf BitBuf) {
	dstBuf.Put8(0, uint8(k.class))
	dstBuf.Put8(8, uint8(k.data))
	dstBuf.Put8(16, uint8(k.data>>8))
	dstBuf.Put8(24, uint8(k.data>>16))
}

func (k *Kind) Decode(srcBuf BitBuf, load LoadFunc) error {
	if srcBuf.Len() < spec.KindBits {
		return fmt.Errorf("buffer too short to be Kind %d < %d", srcBuf.Len(), spec.KindBits)
	}
	x := srcBuf.Get32(0)
	*k = Kind{
		class: spec.TypeCode(x & 0xff),
		data:  x >> 8,
	}
	if err := k.Validate(); err != nil {
		return err
	}
	return k.Validate()
}

func (k *Kind) Zero() Value {
	switch k.class {
	case spec.TC_Kind:
		return KindKind()
	case spec.TC_Bit:
		return BitType{}
	case spec.TC_Array:
		return ArrayOf(Bottom(), 0)
	case spec.TC_Prog:
		return &ProgType{}
	case spec.TC_Ref:
		return NewRefType(ProductType{})
	case spec.TC_Sum:
		st := make(SumType, k.data)
		for i := range st {
			st[i] = Bottom()
		}
		return st
	case spec.TC_Product:
		pt := make(ProductType, k.data)
		for i := range pt {
			pt[i] = Bottom()
		}
		return pt
	case spec.TC_List:
		return ListOf(Bottom())
	case spec.TC_Lazy:
		return NewLazyType(ProductType{})
	case spec.TC_Lambda:
		return NewLambdaType(SumType{}, ProductType{})
	case spec.TC_Fractal:
		ft, err := NewFractalType(NewAnyProg(Prog{
			Literal(SumType{}),
		}))
		if err != nil {
			panic(err)
		}
		return ft
	case spec.TC_Port:
		return NewPortType(Bottom(), Bottom(), Bottom(), Bottom())
	case spec.TC_Distinct:
		return NewDistinctType(ProductType{}, ProductType{})
	case spec.TC_AnyProg:
		return AnyProgType{}
	case spec.TC_AnyType:
		return AnyTypeType{}
	case spec.TC_AnyValue:
		return AnyValueType{}
	default:
		panic(k.class)
	}
}

func (k Kind) Make(x Value) Value {
	switch k.class {
	case spec.TC_Bit:
		return BitType{}
	case spec.TC_Array:
		args := x.(Product)
		return ArrayOf(args[0].(Type), int(*args[1].(*Size)))
	case spec.TC_List:
		args := x.(Product)
		return ListOf(args[0].(Type))
	case spec.TC_Ref:
		args := x.(Product)
		return NewRefType(args[0].(Type))
	case spec.TC_Sum:
		arr := x.(*Array)
		return SumType(slices2.Map(arr.vs, func(x Value) Type {
			return x.(*AnyType).Unwrap()
		}))
	case spec.TC_Product:
		arr := x.(*Array)
		return ProductType(slices2.Map(arr.vs, func(x Value) Type {
			return x.(*AnyType).Unwrap()
		}))
	case spec.TC_Prog:
		args := x.(Product)
		return NewProgType(int(*args[0].(*Size)))
	case spec.TC_Lazy:
		args := x.(Product)
		return NewLazyType(args[0].(Type))
	case spec.TC_Lambda:
		args := x.(Product)
		return NewLambdaType(args[0].(Type), args[1].(Type))
	case spec.TC_Port:
		args := x.(Product)
		return NewPortType(args[0].(Type), args[1].(Type), args[2].(Type), args[3].(Type))
	case spec.TC_Distinct:
		args := x.(Product)
		return NewDistinctType(args[0].(Type), args[1])
	case spec.TC_AnyProg:
		return AnyProgType{}
	case spec.TC_AnyType:
		return AnyTypeType{}
	case spec.TC_AnyValue:
		return AnyValueType{}
	default:
		panic(x)
	}
}

func (k *Kind) Validate() error {
	var k2 *Kind
	switch k.class {
	case spec.TC_Bit:
		k2 = BitKind()
	case spec.TC_Ref:
		k2 = RefKind()
	case spec.TC_Array:
		k2 = ArrayKind()
	case spec.TC_List:
		k2 = ListKind()
	case spec.TC_Lazy:
		k2 = LazyKind()
	case spec.TC_Lambda:
		k2 = LambdaKind()
	case spec.TC_Prog:
		k2 = ProgKind()
	case spec.TC_Fractal:
		k2 = FractalKind()
	case spec.TC_Distinct:
		k2 = DistinctKind()
	case spec.TC_Port:
		k2 = PortKind()
	case spec.TC_Kind:
		k2 = KindKind()
	case spec.TC_AnyProg:
		k2 = AnyProgKind()
	case spec.TC_AnyValue:
		k2 = AnyValueKind()
	case spec.TC_AnyType:
		k2 = AnyTypeKind()
	case spec.TC_Product, spec.TC_Sum:
		return nil
	default:
		return fmt.Errorf("invalid kind class=%v", k.class)
	}
	if k2.data != k.data {
		return fmt.Errorf("invalid kind class=%v data=%v", k.class, k.data)
	}
	return nil
}

func (k Kind) Components() iter.Seq[Value] {
	return emptyIter
}

const low24BitMask = 0x00ff_ffff

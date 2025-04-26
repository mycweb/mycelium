package mycmem

import (
	"context"
	"fmt"
	"reflect"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/spec"

	"golang.org/x/exp/constraints"
)

// ConvertableTo is implemented by types which can be converted to mycelium.Values
type ConvertableTo interface {
	// ToMycelium returns a Mycelium Value for the receiver
	ToMycelium() Value
	MyceliumType() Type
}

// ConvertTo converts a Go value to a Mycelium Value
// If x implement ToMyceliumI than that implementation is used instead.
func ConvertTo(x any) Value {
	if _, ok := x.(reflect.Value); ok {
		panic(x)
	}
	if m, ok := x.(ConvertableTo); ok {
		return m.ToMycelium()
	}
	rv := reflect.ValueOf(x)
	rty := rv.Type()
	switch rty.Kind() {
	case reflect.Pointer:
		return ConvertTo(rv.Elem().Interface())
	case reflect.Array:
		if rv.Len() == 0 {
			return NewArray(Bottom())
		}
		vals := make([]Value, rv.Len())
		for i := 0; i < len(vals); i++ {
			vals[0] = ConvertTo(rv.Index(i).Interface())
		}
		return NewArray(vals[0].Type(), vals...)
	case reflect.Slice:
		if rv.Len() == 0 {
			return NewList(Bottom())
		}
		vals := make([]Value, rv.Len())
		for i := 0; i < len(vals); i++ {
			vals[0] = ConvertTo(rv.Index(i).Interface())
		}
		return NewList(vals[0].Type(), vals...)
	case reflect.Struct:
		ret := make(Product, 0, rv.NumField())
		for i := 0; i < rv.NumField(); i++ {
			if rty.Field(i).IsExported() {
				ret = append(ret, ConvertTo(rv.Field(i).Interface()))
			}
		}
		return ret
	case reflect.Uint8:
		return ptrTo(B8(rv.Uint()))
	case reflect.Uint16:
		return ptrTo(B16(rv.Uint()))
	case reflect.Uint32:
		return ptrTo(B32(rv.Uint()))
	case reflect.Uint64:
		return ptrTo(B64(rv.Uint()))
	default:
		panic(fmt.Sprintf("can't convert %v : %T", x, x))
	}
}

// ConvertableFrom is implemented by types which can be converted from Mycelium values
type ConvertableFrom interface {
	// FromMycelium sets the receiver according to a Mycelium Value
	FromMycelium(Value) error
	MyceliumType() Type
}

// ConvertFrom converts a Mycelium value x to a Go value dst, which must be a pointer to write to.
func ConvertFrom(x Value, dst any) error {
	if um, ok := dst.(ConvertableFrom); ok {
		return um.FromMycelium(x)
	}
	rty := reflect.TypeOf(dst)
	if rty.Kind() != reflect.Pointer || reflect.ValueOf(dst).IsNil() {
		return fmt.Errorf("ConvertFrom must take a pointer. HAVE: %T", dst)
	}
	rty = rty.Elem()
	rv := reflect.ValueOf(dst).Elem()
	switch rty.Kind() {
	case reflect.Struct:
		return convertProduct(x, rv)
	case reflect.Uint8:
		return convertUint[B8, uint8](x, rv)
	case reflect.Uint16:
		return convertUint[B16, uint16](x, rv)
	case reflect.Uint32:
		return convertUint[B32, uint32](x, rv)
	case reflect.Uint64:
		return convertUint[B64, uint64](x, rv)
	}
	return fmt.Errorf("can't Unmarshal %v into %T", x, dst)
}

func convertProduct(x Value, dst reflect.Value) error {
	p, ok := x.(Product)
	if !ok {
		return fmt.Errorf("not a product. HAVE: %v : %v", x, x.Type())
	}
	rty := dst.Type()
	for i := 0; i < rty.NumField(); i++ {
		field := rty.Field(i)
		if !field.IsExported() {
			continue
		}
		if err := ConvertFrom(p[i], dst.Field(i).Addr().Interface()); err != nil {
			return err
		}
	}
	return nil
}

type uintValue interface {
	constraints.Unsigned
}

func convertUint[M uintValue, T constraints.Unsigned](x any, dst reflect.Value) error {
	b8, ok := x.(*M)
	if !ok {
		return fmt.Errorf("unmarshal: %v", x)
	}
	dst.Set(reflect.ValueOf(T(*b8)))
	return nil
}

func scanString(dst *string, x Value) error {
	switch x := x.(type) {
	case *List:
		if elemType := x.a.Elem(); elemType != ByteType() {
			return fmt.Errorf("ScanString called on a List of %v", elemType)
		}
		return scanString(dst, x.a)
	case ByteArray:
		*dst = x.AsString()
		return nil
	default:
		return fmt.Errorf("ScanString on %v :: %v", x, x.Type())
	}
}

// MarshalAppend encodes x using it's type as a codec, and append the data to out
func MarshalAppend(out []byte, x Value) []byte {
	ty := x.Type()
	buf := bitbuf.New(ty.SizeOf())
	x.Encode(buf)
	return append(out, buf.Bytes()...)
}

// SaveRoot prepares an AnyValue containing v, writing additional data to the store, and then
// encodes the root and returns the root bytes.
func SaveRoot(ctx context.Context, dst cadata.PostExister, av *AnyValue) ([]byte, error) {
	buf := bitbuf.New(av.Type().SizeOf())
	if err := av.PullInto(ctx, dst, stores.Union{}); err != nil {
		return nil, err
	}
	av.Encode(buf)
	return buf.Bytes(), nil
}

// LoadRoot decodes an AnyValue from data, unwraps it and returns the Value
func LoadRoot(ctx context.Context, s cadata.Getter, data []byte) (*AnyValue, error) {
	if len(data)*8 < spec.AnyValueBits {
		return nil, fmt.Errorf("buffer too short to be an AnyValue")
	}
	buf := bitbuf.FromBytes(data).Slice(0, spec.AnyValueBits)
	load := func(ref Ref) (Value, error) {
		return Load(ctx, s, ref)
	}
	var av AnyValue
	if err := av.Decode(buf, load); err != nil {
		return nil, err
	}
	return &av, nil
}

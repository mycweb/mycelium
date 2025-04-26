package mycjson

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"

	myc "myceliumweb.org/mycelium/mycmem"
)

func JSONMapType() myc.Type {
	return myc.ListOf(JSONType())
}

func JSONType() *myc.FractalType {
	ty, err := mycexpr.BuildFractalType(func(eb myccanon.EB) *myccanon.Expr {
		return eb.SumType(
			eb.ProductType(),                  // 0: null
			mycexpr.Literal(myc.BitType{}),    // 1: bool
			mycexpr.Literal(myc.StringType()), // 2: number
			mycexpr.Literal(myc.StringType()), // 3: string

			eb.ListType(eb.Self()), // 4: list
			eb.ListType(
				eb.ProductType(mycexpr.Literal(myc.StringType()), eb.Self()),
			), // 5: map
		)
	})
	if err != nil {
		panic(err)
	}
	return ty
}

// JSON is any JSON object
type JSON interface {
	isJSON()
}

type JSONNull struct{}

func (JSONNull) isJSON() {}

type JSONBool bool

func (JSONBool) isJSON() {}

type JSONString string

func (JSONString) isJSON() {}

type JSONNumber json.Number

func (JSONNumber) isJSON() {}

type JSONMap map[JSONString]JSON

func (JSONMap) isJSON() {}

type JSONList []JSON

func (JSONList) isJSON() {}

// NewJSON creates a JSON from the format produced by json.Unmarshal
func NewJSON(x any) JSON {
	switch x := x.(type) {
	case nil:
		return JSONNull{}
	case bool:
		return JSONBool(x)
	case json.Number:
		return JSONNumber(x)
	case float64:
		s := fmt.Sprint(x)
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
		return NewJSON(json.Number(s))
	case []any:
		ret := JSONList{}
		for i := range x {
			ret = append(ret, NewJSON(x[i]))
		}
		return ret
	case string:
		return JSONString(x)
	case map[string]any:
		ret := JSONMap{}
		for k, v := range x {
			ret[NewJSON(k).(JSONString)] = NewJSON(v)
		}
		return ret
	default:
		panic(x)
	}
}

func EncodeJSON(x JSON) (myc.Value, error) {
	jsonType := JSONType().Expanded().(myc.SumType)
	switch x := x.(type) {
	case JSONNull:
		return jsonType.New(0, myc.Product{})
	case JSONBool:
		return jsonType.New(1, myc.BitFromBool(bool(x)))
	case JSONNumber:
		return jsonType.New(2, myc.NewString(string(x)))
	case JSONString:
		return jsonType.New(3, myc.NewString(string(x)))
	case JSONList:
		var vals []myc.Value
		for _, x2 := range x {
			val, err := EncodeJSON(x2)
			if err != nil {
				return nil, err
			}
			vals = append(vals, val)
		}
		return jsonType.New(4, myc.NewList(JSONType(), vals...))
	case JSONMap:
		pairType := myc.ProductType{myc.StringType(), JSONType()}
		var vals []myc.Value
		for k, v := range x {
			k2 := myc.NewString(string(k))
			v2, err := EncodeJSON(v)
			if err != nil {
				return nil, err
			}
			vals = append(vals, myc.Product{k2, v2})
		}
		return jsonType.New(5, myc.NewList(pairType, vals...))
	default:
		panic(x)
	}
}

func PostJSON(ctx context.Context, dst cadata.PostExister, x JSON) (myc.Ref, error) {
	val, err := EncodeJSON(x)
	if err != nil {
		return myc.Ref{}, err
	}
	return myc.Post(ctx, dst, val)
}

func DecodeJSON(ctx context.Context, src cadata.Getter, x myc.Value) (JSON, error) {
	if !myc.TypeContains(JSONType(), x) {
		return nil, fmt.Errorf("decode json: wrong type HAVE: %v :: %v", x, x.Type())
	}
	sum := x.(*myc.Sum)
	tag := sum.Tag()
	switch tag {
	case 0:
		return JSONNull{}, nil
	case 1:
		y := sum.Get(tag).(*myc.Bit)
		return JSONBool(y.AsBool()), nil
	case 2:
		return JSONNumber(sum.Unwrap().(*myc.List).Array().(myc.ByteArray).AsString()), nil
	case 3:
		y := myccanon.AsString(sum.Get(tag))
		return JSONString(y), nil
	case 4:
		y := sum.Get(tag).(*myc.List)
		ret := JSONList{}
		for i := 0; i < y.Len(); i++ {
			y2, err := DecodeJSON(ctx, src, y.Get(i))
			if err != nil {
				return nil, err
			}
			ret = append(ret, y2)
		}
		return ret, nil
	case 5:
		y := sum.Get(tag).(*myc.List)
		ret := JSONMap{}
		for i := 0; i < y.Len(); i++ {
			y2 := y.Get(i)
			k, v, err := decodeJSONPair(ctx, src, y2)
			if err != nil {
				return nil, err
			}
			ret[k] = v
		}
		return ret, nil
	default:
		panic(sum)
	}
}

func decodeJSONPair(ctx context.Context, src cadata.Getter, x myc.Value) (JSONString, JSON, error) {
	ty := myc.ProductType{myc.StringType(), JSONType()}
	if !myc.TypeContains(ty, x) {
		return "", nil, fmt.Errorf("decodeJSONPair: wrong type. HAVE: %v", x)
	}
	tup := x.(myc.Product)
	k := myccanon.AsString(tup[0])
	v, err := DecodeJSON(ctx, src, tup[1])
	if err != nil {
		return "", nil, err
	}
	return JSONString(k), v, nil
}

func PullJSON(ctx context.Context, dst cadata.PostExister, src cadata.Getter, x JSON) error {
	val, err := EncodeJSON(x)
	if err != nil {
		return err
	}
	return val.PullInto(ctx, dst, src)
}

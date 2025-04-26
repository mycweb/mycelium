package mycmem

import (
	"context"
	"fmt"
	"iter"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type FractalType struct {
	expr     Expr
	expanded Type
}

func NewFractalType(body *Expr) (*FractalType, error) {
	if err := ValidateBody(body, 0, true); err != nil {
		return nil, err
	}
	ft := &FractalType{
		expr: *body,
	}
	expanded, err := makeType(ft, body.prog)
	if err != nil {
		return nil, err
	}
	ft.expanded = expanded
	return ft, nil
}

func (*FractalType) isValue() {}
func (*FractalType) isType()  {}

func (*FractalType) Type() Type { return FractalKind() }

func (te *FractalType) SizeOf() int {
	return te.expanded.SizeOf()
}

func (t1 *FractalType) Supersets(x Type) bool {
	if t2, ok := x.(*FractalType); ok {
		if t1 == t2 {
			return true
		}
		if equalProgs(t1.expr.prog, t2.expr.prog) {
			return true
		}
	}
	return Supersets(t1.expanded, x)
}

func (tt *FractalType) Zero() Value {
	return tt.expanded.Zero()
}

func (tt *FractalType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	return tt.expr.PullInto(ctx, dst, src)
}

func (tt *FractalType) Encode(bb bitbuf.Buf) {
	tt.expr.Encode(bb)
}

func (ft *FractalType) Decode(bb bitbuf.Buf, load LoadFunc) error {
	if err := ft.expr.Decode(bb, load); err != nil {
		return err
	}
	expanded, err := makeType(ft, ft.expr.prog)
	if err != nil {
		return err
	}
	ft.expanded = expanded
	return nil
}

func (tt *FractalType) String() string {
	return fmt.Sprintf("Fractal{%s}", tt.expr.String())
}

func (te *FractalType) Body() *Expr {
	return &te.expr
}

func (te *FractalType) Expanded() Type {
	return te.expanded
}

func (ft *FractalType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		yield(&ft.expr)
	}
}

// makeType is used to expand an expression into a type
func makeType(self *FractalType, x Prog) (Type, error) {
	val, err := makeValue(self, x)
	if err != nil {
		return nil, err
	}
	ty, ok := val.(Type)
	if !ok {
		return nil, fmt.Errorf("%v is not a type", val)
	}
	return ty, nil
}

func makeValue(self *FractalType, x Prog) (Value, error) {
	node := x.Root()
	if node.IsLiteral() {
		return node.Literal(), nil
	}
	if node.IsSelf() {
		return self, nil
	}
	switch node.code {
	case spec.ArrayEmpty:
		ty, err := makeType(self, x.Input(0))
		if err != nil {
			return nil, err
		}
		return NewArray(ty), nil
	case spec.ProductEmpty:
		return Product{}, nil
	case spec.ArrayUnit:
		val, err := makeValue(self, x.Input(0))
		if err != nil {
			return nil, err
		}
		return NewArray(val.Type(), val), nil
	case spec.ProductUnit:
		val, err := makeValue(self, x.Input(0))
		if err != nil {
			return nil, err
		}
		return Product{val}, nil
	case spec.Concat:
		leafType, acc, err := evalConcat(nil, self, x)
		if err != nil {
			return nil, err
		}
		if leafType == spec.ArrayUnit {
			return NewArray(acc[0].Type(), acc...), nil
		} else {
			return Product(acc), nil
		}
	case spec.AnyTypeFrom:
		ty, err := makeType(self, x.Input(0))
		if err != nil {
			return nil, err
		}
		return NewAnyType(ty), nil
	case spec.AnyValueFrom:
		val, err := makeValue(self, x.Input(0))
		if err != nil {
			return nil, err
		}
		return NewAnyValue(val), nil
	case spec.Craft:
		maker, err := makeValue(self, x.Input(0))
		if err != nil {
			return nil, err
		}
		data, err := makeValue(self, x.Input(1))
		if err != nil {
			return nil, err
		}
		return maker.(Maker).Make(data), nil
	case spec.ZERO:
		return NewBit(0), nil
	case spec.ONE:
		return NewBit(1), nil

	default:
		return nil, fmt.Errorf("%v not allowed in Fractal body", node.code)
	}
}

func evalConcat(out []Value, self *FractalType, x Prog) (spec.Op, []Value, error) {
	node := x.Root()
	switch {
	case node.IsCode(spec.Concat):
		leafType, out, err := evalConcat(out, self, x.Input(0))
		if err != nil {
			return 0, nil, err
		}
		leafType2, out, err := evalConcat(out, self, x.Input(1))
		if err != nil {
			return 0, nil, err
		}
		if leafType != leafType2 {
			return 0, nil, fmt.Errorf("mismatched leaf types %v and %v", leafType, leafType2)
		}
		return leafType, out, nil
	case node.IsCode(spec.ProductUnit, spec.ArrayUnit):
		val, err := makeValue(self, x.Input(0))
		if err != nil {
			return 0, nil, err
		}
		return node.code, append(out, val), nil
	case node.IsLiteral():
		if al, ok := node.Literal().(ArrayLike); ok {
			var vals []Value
			for i := 0; i < al.Len(); i++ {
				vals = append(vals, al.Get(i))
			}
			return spec.ArrayUnit, append(out, vals...), nil
		}
		if prod, ok := node.Literal().(Product); ok {
			return spec.ProductUnit, append(out, prod...), nil
		}
	}
	return 0, nil, fmt.Errorf("%v in concat tree", x)
}

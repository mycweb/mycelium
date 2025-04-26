package mycgui

import (
	"fmt"
	"image/color"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
)

var (
	GUI_Type = myc.ProductType{
		GUI_RenderLambdaType,
	}
	GUI_RenderLambdaType = myc.NewLambdaType(
		myc.ProductType{
			myccanon.NS_Type, // env
			GUI_OplistType,
		},
		myc.ProductType{},
	)
	GUI_OpType     = Op{}.MyceliumType()
	GUI_OplistType = myc.NewPortType(
		myc.Bottom(),
		myc.Bottom(),
		GUI_OpType,
		myc.ProductType{},
	)
)

func GetGUIExpr(ns *mycexpr.Expr, k string) *mycexpr.Expr {
	eb := mycexpr.EB{}
	return eb.AnyValueTo(myccanon.NSGetExpr(ns, k), GUI_Type)
}

func GetRenderExpr(ns *mycexpr.Expr, k string) *mycexpr.Expr {
	eb := mycexpr.EB{}
	gui := GetGUIExpr(ns, k)
	return eb.Field(gui, 0)
}

var _ myc.ConvertableTo = Op{}

type Op struct {
	Fill      *FillOp
	FillShape *FillShapeOp
}

func (o Op) ToMycelium() myc.Value {
	st := o.MyceliumType().(myc.SumType)
	var ret myc.Value
	var err error
	switch {
	case o.Fill != nil:
		ret, err = st.New(0, myc.ConvertTo(o.Fill))
	case o.FillShape != nil:
		ret, err = st.New(1, myc.ConvertTo(o.FillShape))
	}
	if err != nil {
		panic(err)
	}
	return ret
}

func (o *Op) FromMycelium(x myc.Value) error {
	var fill FillOp
	var fillShape FillShapeOp
	tag, err := unmarshalSum(x, &fill, &fillShape)
	if err != nil {
		return err
	}
	switch tag {
	case 0:
		o.Fill = &fill
	case 1:
		o.FillShape = &fillShape
	}
	return nil
}

func (o Op) MyceliumType() myc.Type {
	return myc.SumType{
		FillOp{}.MyceliumType(),
		// TODO
		// FillShapeOp{}.MyceliumType(),
	}
}

func (o Op) AddTo(ops *op.Ops) {
	switch {
	case o.Fill != nil:
		paint.Fill(ops, color.NRGBA(o.Fill.Color))
	case o.FillShape != nil:
		paint.FillShape(ops, color.NRGBA(o.FillShape.Color), o.FillShape.Clip.Op())
	}
}

type Color color.NRGBA

func (c Color) MyceliumType() myc.Type {
	return myc.ProductType{myc.B8Type(), myc.B8Type(), myc.B8Type(), myc.B8Type()}
}

// FillOp fills the whole frame with a Color
type FillOp struct {
	Color Color
}

var _ myc.ConvertableFrom = &FillOp{}

func (o *FillOp) FromMycelium(x myc.Value) error {
	return unmarshalProduct(x, &o.Color)
}

func (o FillOp) MyceliumType() myc.Type {
	return myc.ProductType{
		myc.ProductType{myc.B8Type(), myc.B8Type(), myc.B8Type(), myc.B8Type()},
	}
}

type FillShapeOp struct {
	Color Color
	Clip  clip.Rect
}

func newIfNil[T any](x *T) {
	if x == nil {
		x = new(T)
	}
}

func unmarshalProduct(x myc.Value, dsts ...any) error {
	p, ok := x.(myc.Product)
	if !ok {
		return fmt.Errorf("not a product")
	}
	if len(p) != len(dsts) {
		return fmt.Errorf("product wrong length. HAVE: %d WANT: %d", len(p), len(dsts))
	}
	for i := range dsts {
		if err := myc.ConvertFrom(p[i], dsts[i]); err != nil {
			return err
		}
	}
	return nil
}

func unmarshalSum(x myc.Value, dsts ...any) (int, error) {
	s, ok := x.(*myc.Sum)
	if !ok {
		return -1, fmt.Errorf("op is a Sum type")
	}
	if err := myc.ConvertFrom(s.Unwrap(), dsts[s.Tag()]); err != nil {
		return -1, err
	}
	return s.Tag(), nil
}

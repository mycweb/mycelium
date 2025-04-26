package compile

import (
	"fmt"

	"go.brendoncarroll.net/exp/slices2"

	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/ast"
)

var (
	AST_Number = myc.NewDistinctType(myc.ListOf(myc.BitType{}), myc.NewString("spore/ast.Number"))
	AST_String = myc.NewDistinctType(myc.StringType(), myc.NewString("spore/ast.String"))
	AST_Param  = myc.NewDistinctType(myc.B32Type(), myc.NewString("spore/ast.Param"))
	AST_Symbol = myc.NewDistinctType(myc.StringType(), myc.NewString("spore/ast.Symbol"))
	AST_Op     = myc.NewDistinctType(myc.B8Type(), myc.NewString("spore/ast.Op"))
)

var AST_Node = mkFractal(func(eb EB) *Expr {
	return eb.DistinctType(eb.SumType(
		eb.Lit(AST_Number),
		eb.Lit(AST_String),
		eb.Lit(AST_Param),
		eb.Lit(AST_Symbol),
		eb.Lit(AST_Op),

		eb.DistinctType(
			eb.ListType(eb.Self()),
			eb.String("spore/ast.Expr"),
		),
		eb.DistinctType(
			eb.ListType(eb.Self()),
			eb.String("spore/ast.Tuple"),
		),
		eb.DistinctType(
			eb.ListType(eb.Self()),
			eb.String("spore/ast.Array"),
		),
		eb.DistinctType(
			eb.ListType(eb.ProductType(eb.Self(), eb.Self())),
			eb.String("spore/ast.Table"),
		),
	), eb.String("spore/ast.Node"))
})

var (
	AST_Expr = myc.NewDistinctType(
		myc.ListOf(AST_Node),
		myc.NewString("spore/ast.Expr"),
	)
	AST_Tuple = myc.NewDistinctType(
		myc.ListOf(AST_Node),
		myc.NewString("spore/ast.Tuple"),
	)
	AST_Array = myc.NewDistinctType(
		myc.ListOf(AST_Node),
		myc.NewString("spore/ast.Array"),
	)

	AST_Row   = myc.ProductType{AST_Node, AST_Node}
	AST_Table = myc.NewDistinctType(
		myc.ListOf(AST_Row),
		myc.NewString("spore/ast.Table"),
	)
)

var MacroType = myc.NewLambdaType(myc.ListOf(AST_Node), AST_Node)

func ASTToMycelium(x ast.Node) myc.Value {
	wrap := func(i int, x myc.Value) myc.Value {
		dt := AST_Node.Expanded().(*myc.DistinctType)
		st := dt.Base().(myc.SumType)
		sum, err := st.New(i, x)
		if err != nil {
			panic(err)
		}
		d, err := dt.New(sum)
		if err != nil {
			panic(err)
		}
		return d
	}
	switch x := x.(type) {
	case ast.Int:
		bi := x.BigInt()
		bs := make([]myc.Value, bi.BitLen())
		for i := 0; i < bi.BitLen(); i++ {
			bs[i] = myc.NewBit(bi.Bit(i))
		}
		ret, err := AST_Number.New(myc.NewList(myc.BitType{}, bs...))
		if err != nil {
			panic(err)
		}
		return wrap(0, ret)
	case ast.String:
		ret, err := AST_String.New(myc.NewString(string(x)))
		if err != nil {
			panic(err)
		}
		return wrap(1, ret)
	case ast.Param:
		ret, err := AST_Param.New(myc.NewB32(x))
		if err != nil {
			panic(err)
		}
		return wrap(2, ret)
	case ast.Symbol:
		ret, err := AST_Symbol.New(myc.NewString(string(x)))
		if err != nil {
			panic(err)
		}
		return wrap(3, ret)
	case ast.Op:
		opc := Primitives()[x]
		ret, err := AST_Op.New(myc.NewB8(opc))
		if err != nil {
			panic(err)
		}
		return wrap(4, ret)

	case ast.SExpr:
		ret, err := AST_Expr.New(myc.NewList(AST_Node, slices2.Map(x, ASTToMycelium)...))
		if err != nil {
			panic(err)
		}
		return wrap(5, ret)
	case ast.Tuple:
		ret, err := AST_Tuple.New(myc.NewList(AST_Node, slices2.Map(x, ASTToMycelium)...))
		if err != nil {
			panic(err)
		}
		return wrap(6, ret)
	case ast.Array:
		ret, err := AST_Array.New(myc.NewList(AST_Node, slices2.Map(x, ASTToMycelium)...))
		if err != nil {
			panic(err)
		}
		return wrap(7, ret)
	case ast.Table:
		ret, err := AST_Table.New(myc.NewList(AST_Row, slices2.Map(x, func(row ast.Row) myc.Value {
			return myc.Product{
				ASTToMycelium(row.Key),
				ASTToMycelium(row.Value),
			}
		})...))
		if err != nil {
			panic(err)
		}
		return wrap(8, ret)
	default:
		panic(x)
	}
}

func ASTFromMycelium(x myc.Value) (ast.Node, error) {
	if !myc.Supersets(AST_Node, x.Type()) {
		return nil, fmt.Errorf("cannot convert %v to an AST Node", x)
	}
	sum := x.(*myc.Distinct).Unwrap().(*myc.Sum)
	makeNodes := func() ([]ast.Node, error) {
		li := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.List)
		ys := make([]ast.Node, li.Len())
		for i := 0; i < li.Len(); i++ {
			y, err := ASTFromMycelium(li.Get(i))
			if err != nil {
				return nil, err
			}
			ys[i] = y
		}
		return ys, nil
	}
	switch sum.Tag() {
	case 0: // Number
		bitArr := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.List).Array().(myc.AsBitArray).AsBitArray()
		// TODO:
		return ast.NewUInt64(uint64(bitArr.AsUint32())), nil
	case 1: // String
		s := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.List).Array().(myc.ByteArray).AsString()
		return ast.String(s), nil
	case 2: // Param
		n := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.B32)
		return ast.Param(*n), nil
	case 3: // Symbol
		s := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.List).Array().(myc.ByteArray).AsString()
		return ast.Symbol(s), nil
	case 4: // Op
		opc := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.B8)
		return ast.Op(spec.Op(*opc).String()), nil
	case 5: // Expr
		ys, err := makeNodes()
		if err != nil {
			return nil, err
		}
		return ast.SExpr(ys), nil
	case 6: // Tuple
		ys, err := makeNodes()
		if err != nil {
			return nil, err
		}
		return ast.Tuple(ys), nil
	case 7: // Array
		ys, err := makeNodes()
		if err != nil {
			return nil, err
		}
		return ast.Tuple(ys), nil
	case 8: // Table
		li := sum.Unwrap().(*myc.Distinct).Unwrap().(*myc.List)
		rows := make([]ast.Row, li.Len())
		for i := 0; i < li.Len(); i++ {
			y := li.Get(i).(myc.Product)
			key, err := ASTFromMycelium(y[0])
			if err != nil {
				return nil, err
			}
			val, err := ASTFromMycelium(y[0])
			if err != nil {
				return nil, err
			}
			rows = append(rows, ast.Row{Key: key, Value: val})
		}
		return ast.Table(rows), nil
	default:
		panic(sum)
	}
}

func mkFractal(fn func(eb EB) *Expr) *myc.FractalType {
	var eb EB
	body := eb.Fractal(fn).Arg(0)
	ft, err := myc.NewFractalType(body.Build())
	if err != nil {
		panic(err)
	}
	return ft
}

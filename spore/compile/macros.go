package compile

import (
	"fmt"
	"math/big"

	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/ast"

	"go.brendoncarroll.net/exp/slices2"
)

const (
	defPrim   = ast.Op("def")
	scopePrim = ast.Op("scope")
)

func fixedBitArray(bi *big.Int, l int) ast.Node {
	lzs := l - bi.BitLen()
	if lzs == 0 {
		return ast.NewBigInt(bi)
	} else if lzs == l {
		return mkZeros(lzs)
	} else {
		return ast.SExpr{ast.Op("concat"),
			ast.NewBigInt(bi),
			mkZeros(lzs),
		}
	}
}

func mkZeros(n int) (ret ast.Array) {
	for i := 0; i < n; i++ {
		ret = append(ret, ast.NewUInt64(0))
	}
	return ret
}

func makeB8(e ast.SExpr) (ast.Node, error) {
	i, ok := e[0].(ast.Int)
	if !ok {
		return nil, fmt.Errorf("makeUint8 requires an ast.Int")
	}
	return fixedBitArray(i.BigInt(), 8), nil
}

func makeB32(e ast.SExpr) (ast.Node, error) {
	i, ok := e[0].(ast.Int)
	if !ok {
		return nil, fmt.Errorf("makeUint32 requires an ast.Int")
	}
	return fixedBitArray(i.BigInt(), 32), nil
}

func makeDistinct(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("distinct requires 2 args HAVE: %v", e)
	}
	return ast.SExpr{ast.Op("craft"), e[0], e[1]}, nil
}

func mkType(kc spec.TypeCode, args ...ast.Node) ast.Node {
	return ast.SExpr{
		ast.Op("craft"),
		ast.SExpr{ast.Op("kind"), ast.NewUInt64(uint64(kc))},
		ast.Tuple(args),
	}
}

func makeBitType(e ast.SExpr) (ast.Node, error) {
	return mkType(spec.TC_Bit), nil
}

func makeArrayType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("Array takes 2 args.  HAVE: %v", e)
	}
	l := e[1]
	if i, ok := l.(ast.Int); ok {
		l = fixedBitArray(i.BigInt(), 32)
	}
	elem := anyTypeFrom(e[0])
	return mkType(spec.TC_Array, elem, l), nil
}

func makeListType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 1 {
		return nil, fmt.Errorf("List takes 1 args.  HAVE: %v", e)
	}
	e = wrapInAnyType(e)
	return mkType(spec.TC_List, e[0]), nil
}

func makeRefType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 1 {
		return nil, fmt.Errorf("Ref takes 1 args.  HAVE: %v", e)
	}
	e = wrapInAnyType(e)
	return mkType(spec.TC_Ref, e[0]), nil
}

func makeSumType(e ast.SExpr) (ast.Node, error) {
	e = wrapInAnyType(e)
	return ast.SExpr{
		ast.Op("craft"),
		ast.SExpr{
			ast.Op("kind"),
			ast.NewInt(int(spec.TC_Sum)),
			ast.NewInt(len(e)),
		},
		ast.Array(e),
	}, nil
}

func makeProductType(e ast.SExpr) (ast.Node, error) {
	e = wrapInAnyType(e)
	return ast.SExpr{
		ast.Op("craft"),
		ast.SExpr{
			ast.Op("kind"),
			ast.NewInt(int(spec.TC_Product)),
			ast.NewInt(len(e)),
		},
		ast.Array(e),
	}, nil
}

func makeLazyType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 1 {
		return nil, fmt.Errorf("Lazy takes 1 args.  HAVE: %v", e)
	}
	e = wrapInAnyType(e)
	return mkType(spec.TC_Lazy, e[0]), nil
}

func makeLambdaType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("Lambda takes 2 args. HAVE: %v", e)
	}
	e = wrapInAnyType(e)
	return mkType(spec.TC_Lambda, e[0], e[1]), nil
}

func makeFractalType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 1 {
		return nil, fmt.Errorf("Fractal takes 1 args. HAVE: %v", e)
	}
	return ast.SExpr{ast.Op("fractal"), e[0]}, nil
}

func makeDistinctType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("Distinct takes 2 args.  HAVE: %v", e)
	}
	base := anyTypeFrom(e[0])
	mark := anyValueFrom(e[1])
	return mkType(spec.TC_Distinct, base, mark), nil
}

func makePortType(e ast.SExpr) (ast.Node, error) {
	if len(e) != 4 {
		return nil, fmt.Errorf("Port takes 4 args. HAVE: %v", e)
	}
	e = wrapInAnyType(e)
	return mkType(spec.TC_Port, e[0], e[1], e[2], e[3]), nil
}

func anyTypeType(e ast.SExpr) (ast.Node, error) {
	return ast.SExpr{
		ast.Op("craft"),
		ast.SExpr{ast.Op("kind"), ast.NewInt(13)},
		ast.Tuple{},
	}, nil
}

func anyValueType(e ast.SExpr) (ast.Node, error) {
	return ast.SExpr{
		ast.Op("craft"),
		ast.SExpr{ast.Op("kind"), ast.NewInt(int(spec.TC_AnyValue))},
		ast.Tuple{},
	}, nil
}

func anyTypeFrom(x ast.Node) ast.Node {
	return ast.SExpr{ast.Op("anyTypeFrom"), x}
}

func anyValueFrom(x ast.Node) ast.Node {
	return ast.SExpr{ast.Op("anyValueFrom"), x}
}

func wrapInAnyType(xs ast.SExpr) ast.SExpr {
	return slices2.Map(xs, anyTypeFrom)
}

// let creates a new scope and adds definitions to it.
func letMacro(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("let requires 2 arguments")
	}
	bindingsNode, body := e[0], e[1]
	bindings, ok := bindingsNode.(ast.Table)
	if !ok {
		return nil, fmt.Errorf("first argument to let must be a Table. HAVE: %v", bindingsNode)
	}
	for i := len(bindings) - 1; i >= 0; i-- {
		row := bindings[i]
		k := row.Key.(ast.Symbol)
		v := row.Value
		body = LetOne(k, v, body)
	}
	return body, nil
}

func lambdaMacro(e ast.SExpr) (ast.Node, error) {
	if len(e) < 3 {
		return nil, fmt.Errorf("lambda requires 3 arguments")
	}

	var inType ast.Node
	innerScope := map[ast.Symbol]ast.Node{}
	switch x := e[0].(type) {
	case ast.Table:
		var inTypes []ast.Node
		for i, row := range x {
			sym, ok := row.Key.(ast.Symbol)
			if !ok {
				return nil, fmt.Errorf("expected symbol in lambda arg %v", row.Key)
			}
			if _, exists := innerScope[sym]; exists {
				return nil, fmt.Errorf("arg %v is defined twice", sym)
			}
			inTypes = append(inTypes, row.Value)
			innerScope[sym] = Field(ast.Param(0), uint32(i))
		}
		inType = append(ast.SExpr{ast.Symbol("Product")}, inTypes...)
	case ast.SExpr:
		if len(x) == 0 {
			inType = ast.SExpr{ast.Symbol("Product")}
		} else {
			// if a Table is not passed as the first arg, then the whole node is the input type
			inType = x[0]
		}
	case ast.Tuple:
		if len(x) == 0 {
			inType = ast.SExpr{ast.Symbol("Product")}
		} else {
			return nil, fmt.Errorf("lambda input cannot be %T %v", x, x)
		}
	default:
		return nil, fmt.Errorf("lambda input cannot be %T %v", x, x)
	}
	outType := e[1]
	body := ast.SExpr{ast.Op("scope")}
	for sym, val := range innerScope {
		body = append(body, Def(sym, val))
	}
	body = append(body, e[2:]...)
	return ast.SExpr{
		ast.Op("lambda"),
		inType,
		outType,
		body,
	}, nil
}

func defMacro(e ast.SExpr) (ast.Node, error) {
	return append(ast.SExpr{defPrim}, e...), nil
}

func defcMacro(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("defc requires 2 arguments")
	}
	return ast.SExpr{ast.Op("def"), e[0], comptimeExpr(e[1])}, nil
}

func deflMacro(e ast.SExpr) (ast.Node, error) {
	if len(e) < 4 {
		return nil, fmt.Errorf("defl requires 4 arguments")
	}
	name, ok := e[0].(ast.Symbol)
	if !ok {
		return nil, fmt.Errorf("first argument to defl must be symbol. HAVE %v", e[0])
	}
	args := e[1:]
	e2, err := lambdaMacro(args)
	if err != nil {
		return nil, err
	}
	return Defc(name, e2), nil
}

func defmMacro(e ast.SExpr) (ast.Node, error) {
	sym, ok := e[0].(ast.Symbol)
	if !ok {
		return nil, fmt.Errorf("defm arg0 must be symbol. HAVE: %v", e[0])
	}
	return Def(
		sym,
		append(ast.SExpr{ast.Op("macro")}, e[1:]...),
	), nil
}

func pubMacro(e ast.SExpr) (ast.Node, error) {
	return append(ast.SExpr{ast.Op("pub")}, e...), nil
}

func ifMacro(e ast.SExpr) (ast.Node, error) {
	if len(e) != 3 {
		return nil, fmt.Errorf("if requires 3 args. HAVE: %v", e)
	}
	return ast.SExpr{
		ast.Op("branch"),
		e[0],
		e[2],
		e[1],
	}, nil
}

func eqMacro(e ast.SExpr) (ast.Node, error) {
	if len(e) != 2 {
		return nil, fmt.Errorf("eq requires 2 args. HAVE %v", e)
	}
	return ast.SExpr{ast.Op("equal"), e[0], e[1]}, nil
}

func selfMacro(e ast.SExpr) (ast.Node, error) {
	return ast.SExpr{ast.Op("self")}, nil
}

func doMacro(e ast.SExpr) (ast.Node, error) {
	return append(ast.SExpr{ast.Op("do")}, e...), nil
}

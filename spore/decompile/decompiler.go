package decompile

import (
	"encoding/hex"
	"math/big"

	"go.brendoncarroll.net/exp/slices2"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
	"myceliumweb.org/mycelium/spore/ast"
	"myceliumweb.org/mycelium/spore/compile"
)

type (
	Value   = mycmem.Value
	Product = mycmem.Product
	Expr    = mycexpr.Expr
)

type Decompiler struct {
	salt  *[32]byte
	dict  map[cadata.ID]ast.Node
	prims map[spec.Op]ast.Op
}

func New(dict map[cadata.ID]ast.Node) *Decompiler {
	if dict == nil {
		dict = make(map[cadata.ID]ast.Node)
	}
	prims := make(map[spec.Op]ast.Op)
	for name, code := range compile.Primitives() {
		code := code
		prims[code] = name
	}
	return &Decompiler{
		dict:  dict,
		salt:  new([32]byte),
		prims: prims,
	}
}

func (dc *Decompiler) Decompile(x mycmem.Value) ast.Node {
	return dc.comptime(dc.decompile(x))
}

func (dc *Decompiler) decompile(x mycmem.Value) (ret ast.Node) {
	fp := mycmem.Fingerprint(x)
	if node, exists := dc.dict[fp]; exists {
		return node
	}
	defer func() {
		if ret != nil {
			dc.dict[fp] = ret
		}
	}()

	switch x := x.(type) {
	// Primitive Values
	case *mycmem.Kind:
		return dc.astFromKind(x)
	case *mycmem.Bit:
		return ast.NewUInt64(uint64(*x))
	case *mycmem.Array:
		return dc.astFromArray(x)
	case *mycmem.Prog:
		expr := mycexpr.FromMycelium(mycmem.NewAnyProg(*x))
		return dc.astFromExpr(expr)
	case *mycmem.Ref:
		return ast.SExpr{ast.Op("ref"), dc.decompile(x.ElemType()), ast.Ref(x.Data())}
	case *mycmem.Sum:
		tag := x.Tag()
		st := dc.decompile(x.Type().(mycmem.SumType))
		elem := dc.decompile(x.Get(tag))
		return dc.mkPrim(spec.MakeSum, st, ast.NewUInt64(uint64(tag)), elem)
	case mycmem.Product:
		return append(ast.Tuple{}, dc.astFromValues(x)...)
	case *mycmem.List:
		if mycmem.Equal(x.Elem(), mycmem.ByteType()) {
			return ast.String(x.Array().(mycmem.ByteArray).AsString())
		}
		return ast.SExpr{ast.Op("listFrom"), dc.astFromArray(x.Array())}
	case *mycmem.Lazy:
		expr := mycexpr.FromMycelium(x.Body())
		return dc.mkPrim(spec.Lazy, dc.astFromExpr(expr))
	case *mycmem.Lambda:
		lt := x.LambdaType()
		return dc.mkPrim(spec.Lambda,
			dc.decompile(lt.In()), dc.decompile(lt.Out()),
			dc.decompile(x.Body()),
		)
	case *mycmem.Port:
		data := x.Data()
		return dc.call("port", ast.String(hex.EncodeToString(data[:])))
	case *mycmem.Distinct:
		return dc.call("distinct", dc.decompile(x.Type()), dc.decompile(x.Unwrap()))
	case *mycmem.AnyType:
		return dc.mkPrim(spec.AnyTypeFrom, dc.decompile(x.Unwrap()))
	case *mycmem.AnyValue:
		return dc.mkPrim(spec.AnyValueFrom, dc.decompile(x.Unwrap()))

	// Primitive Types
	case mycmem.BitType:
		return dc.call("Bit")
	case *mycmem.ArrayType:
		return dc.call("Array", dc.decompile(x.Elem()), ast.NewUInt64(uint64(x.Len())))
	case *mycmem.RefType:
		return dc.call("Ref", dc.decompile(x.Elem()))
	case mycmem.SumType:
		tup := slices2.Map(x, func(x mycmem.Type) mycmem.Value { return x })
		return dc.call("Sum", dc.astFromValues(tup)...)
	case mycmem.ProductType:
		tup := slices2.Map(x, func(x mycmem.Type) mycmem.Value { return x })
		return dc.call("Product", dc.astFromValues(tup)...)
	case *mycmem.ListType:
		// TODO: the dictionary should allow this sort of special casing to work without the check here.
		if x == mycmem.StringType() {
			return ast.Symbol("String")
		}
		return dc.call("List", dc.decompile(x.Elem()))
	case *mycmem.LazyType:
		return dc.call("Lazy", dc.decompile(x.Elem()))
	case *mycmem.LambdaType:
		return dc.call("Lambda", dc.decompile(x.In()), dc.decompile(x.Out()))
	case *mycmem.DistinctType:
		return dc.call("Distinct", dc.decompile(x.Base()), dc.decompile(x.Mark()))
	case *mycmem.FractalType:
		expr := mycexpr.FromMycelium(x.Body())
		return dc.mkPrim(spec.Fractal, dc.astFromExpr(expr))
	case *mycmem.PortType:
		return dc.call("Port",
			dc.decompile(x.Output),
			dc.decompile(x.Input),
			dc.decompile(x.Request),
			dc.decompile(x.Response),
		)
	case mycmem.AnyValueType:
		return dc.mkPrim(spec.Craft, dc.decompile(mycmem.AnyValueKind()), dc.decompile(Product{}))
	case mycmem.AnyTypeType:
		return dc.mkPrim(spec.Craft, dc.decompile(mycmem.AnyTypeKind()), dc.decompile(Product{}))

	// Optimized types
	case *mycmem.B8:
		return dc.astFromBitArray(x.AsBitArray())
	case *mycmem.B16:
		return dc.astFromBitArray(x.AsBitArray())
	case *mycmem.B32:
		return dc.astFromBitArray(x.AsBitArray())
	case *mycmem.B64:
		return dc.astFromBitArray(x.AsBitArray())
	case mycmem.BitArray:
		return dc.astFromBitArray(x)
	case mycmem.ByteArray:
		return dc.astFromArray(x)
	default:
		panic(x)
	}
}

func (dc *Decompiler) mkPrim(code spec.Op, args ...ast.Node) ast.Node {
	// if code.InDegree() == 0 {
	// 	e, err := mycelium.NewExpr(code)
	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	cid := mycelium.ContentID(e)
	// 	if node, exists := dc.dict[cid]; exists {
	// 		return node
	// 	}
	// 		return ast.SExpr{ast.Symbol()}
	// }
	return append(ast.SExpr{ast.Op(dc.prims[code])}, args...)
}

func (dc *Decompiler) astFromArray(x mycmem.ArrayLike) ast.Node {
	if x.Len() == 0 {
		return dc.mkPrim(spec.ArrayEmpty, dc.mkPrim(spec.AnyTypeFrom, dc.decompile(x.Elem())))
	}
	e := ast.Array{}
	for i := 0; i < x.Len(); i++ {
		e = append(e, dc.decompile(x.Get(i)))
	}
	return e
}

// astFromExpr takes a mycelium expression and converts it into an SExpr
// if x.Op == LambdaApply, then arg[0] is used as the first element of the SExpr
// and arg[1] is expanded into the remaining elements of the SExpr
func (dc *Decompiler) astFromExpr(x *Expr) ast.Node {
	if x.IsParam() {
		return ast.Param(x.Param())
	}
	if x.IsSelf() {
		return ast.SExpr{ast.Symbol("self")}
	}
	if x.IsLiteral() {
		return dc.decompile(x.Value())
	}
	name := dc.prims[x.OpCode()]
	return append(ast.SExpr{name}, slices2.Map(x.ArgExprs(), dc.astFromExpr)...)
}

func (dc *Decompiler) astFromValues(x []Value) (ret ast.SExpr) {
	for i := range x {
		ret = append(ret, dc.decompile(x[i]))
	}
	return ret
}

func (dc *Decompiler) astFromBitArray(x mycmem.BitArray) ast.Node {
	switch x.Len() {
	case 0:
		return dc.mkPrim(spec.ArrayEmpty, dc.mkPrim(spec.AnyTypeFrom, dc.decompile(x.Elem())))
	case 1:
		return ast.Array{dc.decompile(x.Get(0))}
	case 8:
		return ast.SExpr{ast.Symbol("b8"), ast.NewInt(int(x.AsUint32()))}
	case 32:
		return ast.SExpr{ast.Symbol("b32"), ast.NewInt(int(x.AsUint32()))}
	}
	bi := new(big.Int)
	for i := 0; i < x.Len(); i++ {
		bi.SetBit(bi, int(i), uint(x.At(i)))
	}
	return fixedBitArray(bi, x.Len())
}

func (dc *Decompiler) astFromKind(k *mycmem.Kind) ast.Node {
	switch k.TypeCode() {
	case spec.TC_Product, spec.TC_Sum:
		return ast.SExpr{ast.Op("kind"),
			ast.NewUInt64(uint64(k.TypeCode())),
			ast.NewUInt64(uint64(k.TypeArity())),
		}
	default:
		return ast.SExpr{
			ast.Op("kind"),
			ast.NewUInt64(uint64(k.TypeCode())),
		}
	}
}

func (dc *Decompiler) call(name string, args ...ast.Node) ast.Node {
	return append(ast.SExpr{ast.Symbol(name)}, args...)
}

func (dc *Decompiler) comptime(x ast.Node) ast.Node {
	return ast.SExpr{ast.Op("comptime"), x}
}

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

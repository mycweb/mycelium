package myctests

import (
	"context"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

// InterestingVaues returns a list of values worth testing against.
func InterestingValues(s cadata.PostExister) []myc.Value {
	eb := EB{}
	mkStr := myc.NewString
	return []myc.Value{
		// bits
		BitType{},
		bit(0),
		bit(1),

		// Arrays
		myc.NewArray(BitType{}),
		myc.NewArray(BitType{}, bit(0), bit(1), bit(1), bit(0)),
		myc.NewArray(myc.B32Type(), b32(1), b32(2), b32(3)),

		// Optimized bitArrays
		b8(123),
		b16(123),
		b32(123),
		b64(123),
		myc.NewBitArray(0, 1, 1),

		// Optimized ByteArrays (Strings)
		mkStr(""),
		mkStr("abcd"),
		mkStr("abcdefghijklmnopqrstuvwxyzABCDEF"),
		myc.NewByteArray([]byte("hello world")),

		// Sums
		mkSum(SumType{myc.B32Type(), BitType{}}, 0, b32(123)),
		mkSum(SumType{myc.B32Type(), BitType{}}, 1, bit(1)),
		mkSum(SumType{myc.B32Type(), myc.NewRefType(myc.B32Type()), myc.B32Type()}, 1, mkRef(s, b32(100))),

		// Products
		Product{b32(1234), mkStr("abcd"), mkStr("a")},
		Product{b32(1234), mkStr("abcd1234567890"), myc.NewArray(myc.B32Type(), b32(1), b32(2)), mkStr("abc")},

		mkRef(s, b32(100)),
		mkRef(s, b32(1000)),
		mkRef(s, myc.NewArray(myc.BitType{}, bit(1), bit(1))),
		mkRef(s, myc.Product{bit(1), bit(1)}),

		// ArrayType
		myc.ArrayOf(myc.B32Type(), 100),

		// ProductType
		ProductType{},
		ProductType{BitType{}},
		ProductType{myc.B32Type(), myc.B32Type(), myc.B32Type()},

		// RefType
		myc.NewRefType(myc.BitType{}),
		myc.NewRefType(myc.ArrayOf(myc.BitType{}, 100)),
		myc.NewRefType(myc.ProductType{myc.B8Type(), myc.B16Type(), myc.B32Type()}),
		myc.NewRefType(myc.SumType{StringType(), myc.B32Type()}),
		myc.NewRefType(myc.NewRefType(myc.B32Type())),

		// Port
		myc.NewPortType(myc.B32Type(), StringType(), myc.BitType{}, myc.NewRefType(myc.StringType())),

		// Distinct
		mkDistinct(myc.NewString("my distinct 32 bit array"), b32(123)),
		mkDistinct(b64(10), mkRef(s, b16(10))),

		// Fractal
		mkFractal(eb.ListType(mycexpr.Self())),

		// Kinds
		myc.PortKind(),
		myc.KindKind(),
		myc.BitKind(),
		myc.RefKind(),
		myc.DistinctKind(),
		myc.ListKind(),
		myc.LazyKind(),
		myc.ArrayKind(),
		myc.ProductKind(4),
		myc.SumKind(5),

		// Expr
		progFor(myc.PortKind()),
		progFor(myc.NewPortType(myc.B8Type(), myc.B16Type(), myc.B32Type(), myc.B64Type())),
		progFor(bit(0)),
		progFor(b32(123456)),
		progFor(mkRef(s, b64(1234567))),
		// progFor(mkExpr(spec.ZERO)),
		// progFor(mkExpr(spec.Concat,
		// 	mkExpr(spec.ProductUnit, mkExpr(spec.ZERO)),
		// 	mkExpr(spec.ProductUnit, mkExpr(spec.ZERO)),
		// )),
		// mkExpr(spec.Equal,
		// 	mkExpr(spec.ZERO), mkExpr(spec.ZERO),
		// ),

		// Ref chain
		mkRef(s, mkRef(s, b32(123))),
		myc.NewArray(myc.NewRefType(myc.B32Type()),
			mkRef(s, b32(0)), mkRef(s, b32(1)), mkRef(s, b32(2)),
		),
	}
}

type (
	Value = myc.Value
	Type  = myc.Type

	BitType = myc.BitType

	Ref     = myc.Ref
	Sum     = myc.Sum
	Product = myc.Product
	Lazy    = myc.Lazy
	Lambda  = myc.Lambda

	ArrayType   = myc.ArrayType
	RefType     = myc.RefType
	ProductType = myc.ProductType
	SumType     = myc.SumType
)

func bit(x int) *myc.Bit {
	return myc.NewBit(x)
}

func b8(x uint8) *myc.B8 {
	return myc.NewB8(x)
}

func b16(x uint16) *myc.B16 {
	return myc.NewB16(x)
}

func b32(x int) *myc.B32 {
	return myc.NewB32(x)
}

func b64(x uint64) *myc.B64 {
	return myc.NewB64(x)
}

func StringType() Type {
	return myc.ListOf(myc.ByteType())
}

func mkRef(s cadata.PostExister, x Value) *Ref {
	ref, err := myc.Post(context.TODO(), s, x)
	if err != nil {
		panic(err)
	}
	return &ref
}

func mkSum(st SumType, tag int, x Value) *myc.Sum {
	s, err := st.New(tag, x)
	if err != nil {
		panic(err)
	}
	return s
}

func mkDistinct(mark Value, x Value) *myc.Distinct {
	dt := myc.NewDistinctType(x.Type(), mark)
	out, err := dt.New(x)
	if err != nil {
		panic(err)
	}
	return out
}

func mkExpr(code spec.Op, args ...*Expr) *Expr {
	e, err := mycexpr.NewExpr(code, args...)
	if err != nil {
		panic(err)
	}
	return e
}

func mkLambda(in, out Type, fn func(eb EB) *Expr) *Lambda {
	la, err := mycexpr.BuildLambda(in, out, fn)
	if err != nil {
		panic(err)
	}
	return la
}

func mkFractal(e *Expr) *myc.FractalType {
	ft, err := mycexpr.BuildFractalType(func(eb EB) *Expr { return e })
	if err != nil {
		panic(err)
	}
	return ft
}

func progFor(x myc.Value) *myc.Prog {
	ret := mycexpr.Literal(x).Build().Prog()
	return &ret
}

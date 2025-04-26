package myccanon

import (
	myc "myceliumweb.org/mycelium/mycmem"
)

var (
	Float16 = myc.ProductType{
		myc.ArrayOf(myc.BitType{}, 12), // mantissa
		myc.ArrayOf(myc.BitType{}, 4),  // exp
		myc.BitType{},                  // sign
	}
)

var (
	Float32 = myc.ProductType{
		myc.ArrayOf(myc.BitType{}, 23), // mantissa
		myc.ArrayOf(myc.BitType{}, 8),  // exp
		myc.BitType{},                  // sign
	}

	Float32_Neg   = lambda(Float32, Float32, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f32_neg")) })
	Float32_Recip = lambda(Float32, Float32, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f32_recip")) })

	Float32_Add = lambda(ProductType{Float32, Float32}, Float32, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f32_add")) })
	Float32_Sub = lambda(ProductType{Float32, Float32}, Float32, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f32_sub")) })
	Float32_Mul = lambda(ProductType{Float32, Float32}, Float32, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f32_mul")) })
	Float32_Div = lambda(ProductType{Float32, Float32}, Float32, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f32_div")) })
)

var (
	Float64 = myc.ProductType{
		myc.ArrayOf(myc.BitType{}, 53), // mantissa
		myc.ArrayOf(myc.BitType{}, 10), // exp
		myc.BitType{},                  // sign
	}

	Float64_Neg   = lambda(Float64, Float64, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f64_neg")) })
	Float64_Recip = lambda(Float64, Float64, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f64_recip")) })

	Float64_Add = lambda(ProductType{Float64, Float64}, Float64, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f64_add")) })
	Float64_Sub = lambda(ProductType{Float64, Float64}, Float64, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f64_sub")) })
	Float64_Mul = lambda(ProductType{Float64, Float64}, Float64, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f64_mul")) })
	Float64_Div = lambda(ProductType{Float64, Float64}, Float64, func(eb EB) *Expr { return eb.Fault(eb.String("must accelerate f64_div")) })
)

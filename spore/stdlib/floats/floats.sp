;; package floats implements floating point numbers according to IEEE-754
;; https://en.wikipedia.org/wiki/IEEE_754

(import "bits")

(defl f32_sign {x: Float32} Bit
    (!field x 2)
)

(defl f32_exp {x: Float32} bits.B8
    (!field x 1)
)

(defl f32_mantissa {x: Float32} (Array Bit 23)
    (!field x 0)
)

(pub Float16)

(pub Float32 f32_add f32_sub f32_mul f32_div)

(pub Float64 f64_add f64_sub f64_mul f64_div)
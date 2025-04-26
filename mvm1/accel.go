package mvm1

import (
	"errors"
	"math"
	"math/bits"

	"golang.org/x/exp/maps"

	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
)

// AccelFunc is the type of accelerators
type AccelFunc = func([]Word) error

func DefaultAccels() map[Fingerprint]AccelFunc {
	return maps.Clone(defaultAccels)
}

var defaultAccels = map[Fingerprint]AccelFunc{
	// Bit
	accKey(myccanon.NOT): func(x []Word) error {
		x[0] = (^x[0]) & 1
		return nil
	},
	accKey(myccanon.AND): func(x []Word) error {
		x[0] = (x[0] >> 0) & (x[0] >> 1) & 1
		return nil
	},
	accKey(myccanon.OR): func(x []Word) error {
		x[0] = ((x[0] >> 0) | (x[0] >> 1)) & 1
		return nil
	},
	accKey(myccanon.XOR): func(x []Word) error {
		x[0] = ((x[0] >> 0) ^ (x[0] >> 1)) & 1
		return nil
	},

	// B32
	accKey(myccanon.B32_NOT): func(x []Word) error {
		x[0] = ^x[0]
		return nil
	},
	accKey(myccanon.B32_AND): func(x []Word) error {
		x[0] = x[0] & x[1]
		return nil
	},
	accKey(myccanon.B32_OR): func(x []Word) error {
		x[0] = x[0] | x[1]
		return nil
	},
	accKey(myccanon.B32_XOR): func(x []Word) error {
		x[0] = x[0] ^ x[1]
		return nil
	},
	accKey(myccanon.B32_POPCOUNT): func(x []Word) error {
		x[0] = Word(bits.OnesCount32(x[0]))
		return nil
	},
	accKey(myccanon.B32_Neg): func(x []Word) error {
		x[0] = 0 - x[0]
		return nil
	},
	accKey(myccanon.B32_Add): func(x []Word) error {
		x[0] = x[0] + x[1]
		return nil
	},
	accKey(myccanon.B32_Sub): func(x []Word) error {
		x[0] = x[0] - x[1]
		return nil
	},
	accKey(myccanon.B32_Mul): func(x []Word) error {
		x[0] = x[0] * x[1]
		return nil
	},
	accKey(myccanon.B32_Div): func(x []Word) error {
		if x[1] == 0 {
			return errors.New("divide by 0")
		}
		x[0] = x[0] / x[1]
		return nil
	},

	// B64
	accKey(myccanon.B64_POPCOUNT): func(x []Word) error {
		y := getUint64(x)
		y = uint64(bits.OnesCount64(y))
		putUint64(x, y)
		return nil
	},
	accKey(myccanon.B64_Neg): func(x []Word) error {
		y := getUint64(x)
		y = 0 - y
		putUint64(x, y)
		return nil
	},
	accKey(myccanon.B64_Add): func(x []Word) error {
		b64BinaryOp(x, func(a, b uint64) uint64 {
			return a + b
		})
		return nil
	},
	accKey(myccanon.B64_Sub): func(x []Word) error {
		b64BinaryOp(x, func(a, b uint64) uint64 {
			return a - b
		})
		return nil
	},
	accKey(myccanon.B64_Mul): func(x []Word) error {
		b64BinaryOp(x, func(a, b uint64) uint64 {
			return a * b
		})
		return nil
	},
	accKey(myccanon.B64_Div): func(x []Word) error {
		if x[2] == 0 && x[3] == 0 {
			return errors.New("divide by 0")
		}
		b64BinaryOp(x, func(a, b uint64) uint64 {
			return a / b
		})
		return nil
	},

	accKey(myccanon.Float32_Neg): func(x []Word) error {
		f := math.Float32frombits(x[0])
		f = 0 - f
		x[0] = math.Float32bits(f)
		return nil
	},
	accKey(myccanon.Float32_Recip): func(x []Word) error {
		f := math.Float32frombits(x[0])
		f = 1 / f
		x[0] = math.Float32bits(f)
		return nil
	},
	accKey(myccanon.Float32_Add): func(x []Word) error {
		float32BinaryOp(x, func(a, b float32) float32 {
			return a + b
		})
		return nil
	},
	accKey(myccanon.Float32_Sub): func(x []Word) error {
		float32BinaryOp(x, func(a, b float32) float32 {
			return a - b
		})
		return nil
	},
	accKey(myccanon.Float32_Mul): func(x []Word) error {
		float32BinaryOp(x, func(a, b float32) float32 {
			return a * b
		})
		return nil
	},
	accKey(myccanon.Float32_Div): func(x []Word) error {
		float32BinaryOp(x, func(a, b float32) float32 {
			return a / b
		})
		return nil
	},
}

func getUint64(ws []Word) uint64 {
	return (uint64(ws[0]) << 0) | (uint64(ws[1]) << 32)
}

func putUint64(ws []Word, x uint64) {
	ws[0], ws[1] = uint32(x), uint32(x>>32)
}

func b64BinaryOp(ws []Word, fn func(a, b uint64) uint64) {
	a := getUint64(ws[0:2])
	b := getUint64(ws[2:4])
	y := fn(a, b)
	putUint64(ws, y)
}

func float32BinaryOp(ws []Word, fn func(a, b float32) float32) {
	a := math.Float32frombits(ws[0])
	b := math.Float32frombits(ws[1])
	o := fn(a, b)
	ws[0] = math.Float32bits(o)
}

// accKey is the key used for accelerators
func accKey(lam *myc.Lambda) (ret Fingerprint) {
	fp := myc.Fingerprint(lam)
	bytesToWords(fp[:], ret[:])
	return ret
}

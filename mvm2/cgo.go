package mvm2

//go:generate zig build-lib -femit-bin=libmvm2zig.a src/root.zig

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: ${SRCDIR}/libmvm2zig.a
#include "mvm2.h"
*/
import "C"

func twoTimes(x int) int {
    return int(C.mvm2_two_times(C.int(x)))
}

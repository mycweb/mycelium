package mycss

import (
	"context"

	"go.brendoncarroll.net/tai64"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
)

var (
	TAI64N = myc.ProductType{
		myc.B64Type(),
		myc.B32Type(),
	}
	DEV_WALLCLOCK_Type = myc.NewPortType(
		myc.Bottom(),
		TAI64N,
		myc.Bottom(),
		myc.Bottom(),
	)
)

func GetWallClock(ns *Expr, k string) *Expr {
	eb := EB{}
	return eb.AnyValueTo(
		myccanon.NSGetExpr(ns, k),
		DEV_WALLCLOCK_Type,
	)
}

func CurrentTimeExpr(clk *Expr) *Expr {
	eb := EB{}
	return eb.Input(clk)
}

type wallClockDev struct {
}

func (cs wallClockDev) portInput(ctx context.Context, _ cadata.PostExister, buf []mvm1.Word) error {
	ts := tai64.Now()
	buf[0] = uint32(ts.Seconds)
	buf[1] = uint32(ts.Nanoseconds)
	return nil
}

func (cs wallClockDev) PortType() *myc.PortType {
	return DEV_WALLCLOCK_Type
}

func (cs wallClockDev) Port() mvm1.PortBackend {
	return mvm1.PortBackend{
		Input: cs.portInput,
	}
}

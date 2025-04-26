package mycss

import (
	"context"
	"io"
	"sync"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/spec"
)

var (
	DEV_CONSOLE_Type = myc.NewPortType(
		myc.StringType(), // accept strings
		myc.Bottom(),     // never receive
		myc.Bottom(),     // never request
		myc.Bottom(),     // never respond
	)
)

func GetConsole(env *Expr, k string) *Expr {
	return EB{}.AnyValueTo(
		myccanon.NSGetExpr(env, k),
		DEV_CONSOLE_Type,
	)
}

type consoleDev struct {
	mu  sync.Mutex
	out io.Writer
}

func newConsoleSvc(out io.Writer) *consoleDev {
	return &consoleDev{out: out}
}

func (c *consoleDev) output(ctx context.Context, s cadata.Getter, buf []mvm1.Word) error {
	data := wordsToBytes(buf)
	x := myc.StringType().Zero().(*myc.List)
	load := func(ref myc.Ref) (myc.Value, error) {
		return myc.Load(ctx, s, ref)
	}
	if err := x.Decode(bitbuf.FromBytes(data).Slice(0, spec.ListBits), load); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.out.Write(x.Array().(myc.ByteArray).AsBytes())
	return err
}

func (c *consoleDev) PortType() *myc.PortType {
	return DEV_CONSOLE_Type
}

func (c *consoleDev) Port() mvm1.PortBackend {
	return mvm1.PortBackend{
		Output: c.output,
	}
}

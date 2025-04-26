package mycss

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.brendoncarroll.net/stdctx/logctx"
	"go.uber.org/zap"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/spec"
)

var (
	DEV_CELL_Type = myc.NewPortType(
		myc.AnyValueType{}, // put
		myc.AnyValueType{}, // load
		myc.ProductType{myc.AnyValueType{}, myc.AnyValueType{}}, // cas (prev, next)
		myc.AnyValueType{}, // cas returns current value
	)
)

func GetCell(env *Expr, k string) *Expr {
	return EB{}.AnyValueTo(myccanon.NSGetExpr(env, k), DEV_CELL_Type)
}

type cell struct {
	p   *Pod
	pid ProcID
	k   string
}

func newCell(p *Pod, procID ProcID, k string) *cell {
	return &cell{
		p:   p,
		pid: procID,
		k:   k,
	}
}

func (c *cell) Load(ctx context.Context) (*myc.AnyValue, error) {
	if c.pid == 0 {
		panic("cell: pid not set")
	}
	logctx.Debug(ctx, "cell load", zap.String("key", c.k))
	val, err := dbutil.DoTx1(ctx, c.p.env.DB, func(tx *sqlx.Tx) (myc.Value, error) {
		if err := c.p.checkProcAlive(tx, c.pid); err != nil {
			return nil, err
		}
		v, err := c.p.nsGet(tx, c.k)
		if err != nil {
			return nil, err
		}
		return v, nil
	})
	if err != nil {
		return nil, err
	}
	return myc.NewAnyValue(val), nil
}

func (c *cell) CAS(ctx context.Context, prev, next *myc.AnyValue) (*myc.AnyValue, error) {
	if c.pid == 0 {
		panic("cell: pid not set")
	}
	val, err := dbutil.DoTx1(ctx, c.p.env.DB, func(tx *sqlx.Tx) (myc.Value, error) {
		if err := c.p.checkProcAlive(tx, c.pid); err != nil {
			return nil, err
		}
		return c.p.nsCAS(ctx, tx, c.k, prev.Unwrap(), next.Unwrap())
	})
	if err != nil {
		return nil, err
	}
	return myc.NewAnyValue(val), nil
}

func (c *cell) PortType() *myc.PortType {
	return DEV_CELL_Type
}

func (c *cell) Port() mvm1.PortBackend {
	return mvm1.PortBackend{
		Interact: func(ctx context.Context, s cadata.Store, buf []mvm1.Word) error {
			reqData := wordsToBytes(buf)
			req := myc.Product{&myc.AnyValue{}, &myc.AnyValue{}}
			load := func(ref myc.Ref) (myc.Value, error) {
				return myc.Load(ctx, s, ref)
			}
			if err := req.Decode(bitbuf.FromBytes(reqData).Slice(0, 2*spec.AnyValueBits), load); err != nil {
				return err
			}

			prev, next := req[0], req[1]
			av, err := c.CAS(ctx, prev.(*myc.AnyValue), next.(*myc.AnyValue))
			if err != nil {
				return err
			}
			data := myc.MarshalAppend(nil, av)
			if err := av.PullInto(ctx, s, c.p.newStore()); err != nil {
				return err
			}
			return bytesToWords(data, buf[:spec.AnyValueBits/mvm1.WordBits])
		},
		Input: func(ctx context.Context, dst cadata.PostExister, buf []mvm1.Word) error {
			av, err := c.Load(ctx)
			if err != nil {
				return err
			}
			data := myc.MarshalAppend(nil, av)
			if err := av.PullInto(ctx, dst, c.p.newStore()); err != nil {
				return err
			}
			return bytesToWords(data, buf)
		},
	}
}

func wordsToBytes(x []mvm1.Word) []byte {
	ret := make([]byte, len(x)*mvm1.WordBytes)
	for i := range x {
		binary.LittleEndian.PutUint32(ret[i*4:], x[i])
	}
	return ret
}

func bytesToWords(src []byte, dst []mvm1.Word) error {
	if len(dst)*mvm1.WordBytes < len(src) {
		return fmt.Errorf("port: incorrect buffer length. HAVE: %d WANT: %d", len(dst)*mvm1.WordBytes, len(src))
	}
	for len(src)%mvm1.WordBytes != 0 {
		src = append(src, 0)
	}
	for i := range dst {
		dst[i] = binary.LittleEndian.Uint32(src[i*4:])
	}
	return nil
}

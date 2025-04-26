package mycgui

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"
)

// Main must be called on the main thread.
func Main(ctx context.Context, pod *mycss.Pod) {
	go func() {
		window := new(app.Window)
		err := run(ctx, window, pod)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(ctx context.Context, window *app.Window, pod *mycss.Pod) error {
	// theme := material.NewTheme()
	return pod.DoInProcess(ctx, func(pctx mycss.ProcCtx) error {
		guiVal, exists := pctx.NS()["GUI"]
		if !exists {
			return fmt.Errorf("namespace is missing GUI entry")
		}
		if !myc.TypeContains(GUI_Type, guiVal) {
			return fmt.Errorf("wrong type for GUI. HAVE: %v, WANT: %v", guiVal.Type(), GUI_Type)
		}
		var ops op.Ops

		// Setup port for the OpList
		port := myc.NewRandPort(GUI_OplistType)
		vm := pctx.VM()
		vm.PutPort(mvm1.PortFromBytes(port.Data()), mvm1.PortBackend{
			Interact: func(ctx context.Context, s cadata.Store, buf []mvm1.Word) error {
				data := bytesFromWords(buf)
				load := func(ref myc.Ref) (myc.Value, error) {
					return myc.Load(ctx, s, ref)
				}
				mop := GUI_OpType.Zero()
				if err := mop.Decode(bitbuf.FromBytes(data), load); err != nil {
					return err
				}
				var op Op
				if err := op.FromMycelium(mop); err != nil {
					return err
				}
				op.AddTo(&ops)
				return nil
			},
		})
		for {
			switch e := window.Event().(type) {
			case app.DestroyEvent:
				return e.Err
			case app.FrameEvent:
				// e.Source.Event(pointer.Filter{Kinds: pointer.Press})

				// // Pass the drawing operations to the GPU.
				// e.Frame(gtx.Ops)
				if err := RenderFrame(ctx, pctx, &ops, port, e); err != nil {
					return err
				}
			}
		}
	})
}

func RenderFrame(ctx context.Context, pctx mycss.ProcCtx, ops *op.Ops, port *myc.Port, fe app.FrameEvent) error {
	ops.Reset()

	laz, err := mycexpr.BuildLazy(myc.ProductType{}, func(eb mycexpr.EB) *mycexpr.Expr {
		return eb.Apply(
			GetRenderExpr(mycexpr.Param(0), "GUI"),
			eb.Product(eb.P(0), eb.Lit(port)),
		)
	})
	if err != nil {
		return err
	}
	if _, err = pctx.Eval(ctx, laz); err != nil {
		return err
	}
	fe.Frame(ops)
	return nil
}

func greyGrid(ops *op.Ops, fe app.FrameEvent) {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			paint.FillShape(ops,
				grey(uint8(math.Sin(float64(fe.Now.UnixNano())/5e8)*128+128)),
				clip.Rect{
					Min: image.Point{X: i*120 + 10, Y: j*120 + 10 + 300},
					Max: image.Point{X: i*120 + 110, Y: j*120 + 110 + 300},
				}.Op(),
			)
		}
	}
}

func grey(x uint8) color.NRGBA {
	return color.NRGBA{R: x, G: x, B: x, A: 255}
}

func bytesFromWords(ws []mvm1.Word) []byte {
	ret := make([]byte, len(ws)*mvm1.WordBytes)
	for i := range ws {
		binary.LittleEndian.PutUint32(ret[i*4:], ws[i])
	}
	return ret
}

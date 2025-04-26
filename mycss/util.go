package mycss

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/myccanon/mycjson"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/myczip"
)

// ResetZipFile resets a pod to the contents of a zip file
func ResetZipFile(ctx context.Context, pod *Pod, f myczip.File, cfg PodConfig) error {
	v, s, err := myczip.LoadFromFile(ctx, f)
	if err != nil {
		return err
	}
	ns := myccanon.Namespace{}
	if err := ns.FromMycelium(v); err != nil {
		return err
	}
	return pod.Reset(ctx, s, ns, cfg)
}

// ResetZipReader resets a pod to a namespace encoded in the myczip format.
func ResetZipReader(ctx context.Context, pod *Pod, zr *zip.Reader, cfg PodConfig) error {
	val, s, err := myczip.Load(zr)
	if err != nil {
		return err
	}
	ns := myccanon.Namespace{}
	if err := ns.FromMycelium(val); err != nil {
		return err
	}
	return pod.Reset(ctx, s, ns, cfg)
}

func NewTestSys(t testing.TB) *System {
	ctx := testutil.Context(t)
	db := dbutil.NewTestDB(t)
	require.NoError(t, SetupDB(ctx, db))
	return NewSystem(db)
}

// Eval spawns a new Process to evaluate the expr.
func Eval(ctx context.Context, p *Pod, dst cadata.PostExister, src cadata.GetExister, fn func(env myc.Value) *myc.Lazy) (myc.Value, error) {
	var out myc.Value
	if err := p.DoInProcess(ctx, func(pc ProcCtx) error {
		laz := fn(pc.NS().ToMycelium())
		if err := laz.PullInto(ctx, pc.Store(), stores.Union{src, p.Store()}); err != nil {
			return err
		}
		var err error
		out, err = pc.Eval(ctx, laz)
		if err != nil {
			return err
		}
		return out.PullInto(ctx, dst, stores.Union{pc.Store(), p.Store()})
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// Main spawns a process to evaluate the symbol "" in the namespace, and blocks until it completes
func Main(ctx context.Context, p *Pod) error {
	return p.DoInProcess(ctx, func(pc ProcCtx) error {
		entry, exists := pc.NS()[""]
		if !exists {
			return fmt.Errorf("start failed: pod namespace is not executable")
		}
		laz, err := mycexpr.BuildLazy(myc.ProductType{}, func(eb EB) *Expr {
			return eb.LetVal(pc.NS().ToMycelium(), func(eb EB) *Expr {
				return eb.Apply(mycexpr.Literal(entry), eb.P(0))
			})
		})
		if err != nil {
			return err
		}
		_, err = pc.Eval(ctx, laz)
		return err
	})
}

// InvokeJSONType returns the type of the Lambda called by InvokeJSON
func InvokeJSONType() *myc.LambdaType {
	return myc.NewLambdaType(
		myc.ProductType{myccanon.NS_Type, mycjson.JSONType()},
		mycjson.JSONType(),
	)
}

// InvokeJSON looks for a entry in the Pod's namespace called "invokeJSON" and calls it with
// a JSON value parsed from input.
func InvokeJSON(ctx context.Context, pod *Pod, input json.RawMessage) (json.RawMessage, error) {
	const invokeJSONKey = "invokeJSON"
	inputObj := map[string]any{}
	if err := json.Unmarshal(input, &inputObj); err != nil {
		return nil, err
	}
	inputVal, err := mycjson.EncodeJSON(mycjson.NewJSON(inputObj))
	if err != nil {
		return nil, err
	}
	s := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)

	out, err := Eval(ctx, pod, s, s, func(env myc.Value) *myc.Lazy {
		laz, err := mycexpr.BuildLazy(mycjson.JSONType(), func(eb EB) *Expr {
			return eb.LetVal(env, func(eb EB) *Expr {
				return eb.Apply(
					eb.AnyValueTo(myccanon.NSGetExpr(eb.P(0), invokeJSONKey), InvokeJSONType()),
					eb.Product(eb.P(0), mycexpr.Literal(inputVal)),
				)
			})
		})
		if err != nil {
			panic(err)
		}
		return laz
	})
	if err != nil {
		return nil, err
	}
	jv, err := mycjson.DecodeJSON(ctx, s, out)
	if err != nil {
		return nil, err
	}
	return json.Marshal(jv)
}

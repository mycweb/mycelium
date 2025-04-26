package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/internal/testutil"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"
	"myceliumweb.org/mycelium/myczip"
	"myceliumweb.org/mycelium/spore/build"
)

func TestSpore(t *testing.T) {
	const dataDir = "./testdata"
	ctx := testutil.Context(t)
	ents, err := os.ReadDir(dataDir)
	require.NoError(t, err)
	for _, ent := range ents {
		name := ent.Name()
		root := os.DirFS(filepath.Join(dataDir, name))
		t.Run(name, func(t *testing.T) {
			c := build.NewContext([]build.Source{
				{Prefix: name, FS: root},
				build.StdLib(),
			})
			pkgFile := testutil.TempFile(t)
			require.NoError(t, c.WriteZip(ctx, name, false, pkgFile))
			_, err = pkgFile.Seek(0, io.SeekStart)
			require.NoError(t, err)

			root, _, err := myczip.LoadFromFile(ctx, pkgFile)
			require.NoError(t, err)
			ns := myccanon.Namespace{}
			require.NoError(t, ns.FromMycelium(root))

			t.Log(root)
			// TODO: add this back
			// t.Log(spore.PrintString(root))

			s := newSide(t)
			pod := s.createPod(t)
			devs := map[string]mycss.DeviceSpec{
				"console": mycss.DevConsole(),
			}
			t.Log("DEVICES:", devs)
			for k := range ns {
				if strings.HasPrefix(k, "cell") {
					devs[k] = mycss.DevCell()
				}
			}
			require.NoError(t, mycss.ResetZipFile(ctx, pod, pkgFile, mycss.PodConfig{
				Devices: devs,
			}))
			actualNS, err := pod.GetAll(ctx)
			require.NoError(t, err)
			// we add 1 for the pod services:
			// - console
			require.Len(t, actualNS, len(ns)+1)

			store := testutil.NewStore(t)
			out, err := mycss.Eval(ctx, pod, store, store, func(env myc.Value) *myc.Lazy {
				laz, err := mycexpr.BuildLazy(myc.Bottom(), func(eb mycexpr.EB) *mycexpr.Expr {
					return eb.LetVal(env, func(eb mycexpr.EB) *mycexpr.Expr {
						return eb.Apply(
							eb.AnyValueTo(
								myccanon.NSGetExpr(mycexpr.Param(0), "main"),
								myc.NewLambdaType(
									myc.ProductType{myccanon.NS_Type},
									myc.ProductType{},
								),
							),
							mycexpr.EB{}.Product(mycexpr.Param(0)), // whole namespace as first argument
						)
					})
				})
				require.NoError(t, err)
				return laz
			})
			require.NoError(t, err)

			t.Logf("out: %v %v", out.Type(), out)
			// TODO: add this back
			// t.Logf("out: %v %v", spore.PrintString(out.Type()), spore.PrintString(out))
		})
	}
}

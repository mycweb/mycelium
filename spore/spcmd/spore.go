package spcmd

import (
	"go.brendoncarroll.net/star"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/myccmd"
	"myceliumweb.org/mycelium/mycexpr"
	"myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"
	"myceliumweb.org/mycelium/spore"
	"myceliumweb.org/mycelium/spore/printer"
)

func Root() star.Command {
	return rootCmd
}

var rootCmd = star.NewDir(star.Metadata{
	Short: "work with spore programs and expressions",
}, map[star.Symbol]star.Command{
	"eval":  spEval,
	"build": spBuild,
	"test":  spTest,

	"run":     spRun,
	"run-gui": spRunGui,
})

var spEval = star.Command{
	Metadata: star.Metadata{
		Short: "evaluate a spore expression in the context of a pod",
	},
	Flags: []star.IParam{dbParam, podIDParam},
	Pos:   []star.IParam{exprParam},
	F: func(c star.Context) error {
		db := dbParam.Load(c)
		sys := mycss.NewSystem(db)
		pod, err := sys.Get(c, myccmd.PodIDParam.Load(c))
		if err != nil {
			return err
		}
		exprStr := exprParam.Load(c)
		expr, err := spore.CompileSnippet([]byte(exprStr))
		if err != nil {
			return err
		}
		store := newMemStore()
		out, err := mycss.Eval(c, pod, store, store, func(env mycmem.Value) *mycmem.Lazy {
			laz, err := mycexpr.BuildLazy(mycmem.Bottom(), func(eb mycexpr.EB) *mycexpr.Expr {
				return eb.LetVal(env, func(eb mycexpr.EB) *mycexpr.Expr { return expr })
			})
			if err != nil {
				panic(err)
			}
			return laz
		})
		if err != nil {
			return err
		}
		printer.Printer{}.Print(c.StdOut, spore.Decompile(out))
		c.Printf("\n%v :: %v\n", out, out.Type())
		return nil
	},
}

var exprParam = star.Param[string]{Name: "expr", Parse: star.ParseString}

var (
	dbParam    = myccmd.DBParam
	podIDParam = myccmd.PodIDParam

	cellParam    = myccmd.CellParam
	netParam     = myccmd.NetNodeParam
	consoleParam = myccmd.ConsoleParam
)

func newMemStore() cadata.Store {
	return stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
}

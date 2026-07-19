package spcmd

import (
	"bufio"

	"github.com/jmoiron/sqlx"
	"go.brendoncarroll.net/star"

	"myceliumweb.org/mycelium"
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
}, map[string]star.Command{
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
	Flags: map[string]star.Flag{"db": &myccmd.DBParam, "pod": &myccmd.PodIDParam},
	Pos:   []star.Positional{&exprParam},
	F: func(c star.Context) error {
		db, err := loadDB(c)
		if err != nil {
			return err
		}
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
		w := bufio.NewWriter(c.StdOut)
		if err := (printer.Printer{}).Print(w, spore.Decompile(out)); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
		c.Printf("\n%v :: %v\n", out, out.Type())
		return nil
	},
}

var exprParam = star.Required[string]{PosName: "expr", Parse: star.ParseString}

func newMemStore() mycelium.RW {
	return stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
}

func loadDB(c star.Context) (*sqlx.DB, error) {
	if db, ok := myccmd.DBParam.LoadOpt(c); ok {
		return db, nil
	}
	return myccmd.DBParam.Parse(":memory:")
}

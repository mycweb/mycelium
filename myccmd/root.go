// package myccmd implemnts the myc command line tool.
package myccmd

import (
	"context"
	"net"

	"github.com/jmoiron/sqlx"
	"go.brendoncarroll.net/star"

	"myceliumweb.org/mycelium/mycss"
)

func Root() star.Command {
	return root
}

var root = star.NewDir(star.Metadata{
	Short: "Mycelium Web Platform",
}, map[star.Symbol]star.Command{
	// pod commands
	"run":         run,
	"run-pods":    runPods,
	"serve":       serve,
	"invoke-json": invokeJSON,

	"create": create,
	"list":   list,
	"drop":   drop,
	"reset":  reset,

	"status": status,
	"zip":    zipCmd,
})

var status = star.Command{
	Flags: []star.IParam{DBParam},
	Pos:   []star.IParam{},
	F: func(ctx star.Context) error {
		ctx.Printf("STATUS\n")
		db := DBParam.Load(ctx)
		if err := db.Ping(); err != nil {
			return err
		}
		return db.Close()
	},
}

var DBParam = star.Param[*sqlx.DB]{
	Name:    "db",
	Default: star.Ptr(":memory:"),
	Parse: func(x string) (*sqlx.DB, error) {
		db, err := mycss.OpenDB(x)
		if err != nil {
			return nil, err
		}
		if err := mycss.SetupDB(context.Background(), db); err != nil {
			return nil, err
		}
		return db, nil
	},
}

var ListenerParam = star.Param[net.Listener]{
	Name:    "l",
	Default: star.Ptr("127.0.0.1:6666"),
	Parse: func(x string) (net.Listener, error) {
		return net.Listen("tcp", x)
	},
}

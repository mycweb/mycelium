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
}, map[string]star.Command{
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
	Flags: map[string]star.Flag{"db": &DBParam},
	Pos:   []star.Positional{},
	F: func(ctx star.Context) error {
		ctx.Printf("STATUS\n")
		db, err := loadDB(ctx)
		if err != nil {
			return err
		}
		if err := db.Ping(); err != nil {
			return err
		}
		return db.Close()
	},
}

var DBParam = star.Optional[*sqlx.DB]{
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

var ListenerParam = star.Optional[net.Listener]{
	Parse: func(x string) (net.Listener, error) {
		return net.Listen("tcp", x)
	},
}

func loadDB(c star.Context) (*sqlx.DB, error) {
	if db, ok := DBParam.LoadOpt(c); ok {
		return db, nil
	}
	return DBParam.Parse(":memory:")
}

func loadListener(c star.Context) (net.Listener, error) {
	if lis, ok := ListenerParam.LoadOpt(c); ok {
		return lis, nil
	}
	return ListenerParam.Parse("127.0.0.1:6666")
}

package mycss

import (
	"context"

	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/mycss/internal/migrations"
	"myceliumweb.org/mycelium/mycss/internal/sqlstores"

	"github.com/jmoiron/sqlx"
)

func OpenDB(p string) (*sqlx.DB, error) {
	return dbutil.Open(p)
}

func SetupDB(ctx context.Context, db *sqlx.DB) error {
	return migrations.Migrate(ctx, db, currentSchema)
}

var currentSchema = func() *migrations.State {
	x := migrations.InitialState()
	x = sqlstores.Migration(x)
	x = x.ApplyStmt(`CREATE TABLE pods (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		store_id INTEGER NOT NULL,
		secret BLOB NOT NULL,
		last_proc_id INTEGER NOT NULL DEFAULT 0,
		dead_lteq INTEGER NOT NULL DEFAULT 0,
		config TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

		FOREIGN KEY(store_id) REFERENCES store_id
	)`)
	x = x.ApplyStmt(`CREATE TABLE pod_ns (
		pod_id NOT NULL,
		k TEXT NOT NULL,
		v BLOB NOT NULL,

		FOREIGN KEY(pod_id) REFERENCES pods(id),
		PRIMARY KEY(pod_id, k)
	)`)
	return x
}()

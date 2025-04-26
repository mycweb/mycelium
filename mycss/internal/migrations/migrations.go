package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"myceliumweb.org/mycelium/mycss/internal/dbutil"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/sha3"
)

type TxAPI interface {
	sqlx.Execer
}

type MigrationFunc = func(tx TxAPI) error

type State struct {
	n        int
	previous *State
	fn       MigrationFunc
}

func InitialState() *State {
	return &State{
		fn: func(tx TxAPI) error {
			_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS migrations (
				id INTEGER PRIMARY KEY,
				sql_json BLOB NOT NULL,
				fingerprint BLOB NOT NULL
			)`)
			return err
		},
	}
}

func (s *State) N() int {
	return s.n
}

func (s *State) Apply(fn MigrationFunc) *State {
	return &State{
		n:        s.n + 1,
		previous: s,
		fn:       fn,
	}
}

func (s *State) ApplyStmt(q string) *State {
	return s.Apply(func(tx TxAPI) error {
		_, err := tx.Exec(q)
		return err
	})
}

func Migrate(ctx context.Context, db *sqlx.DB, desired *State) error {
	return dbutil.DoTx(ctx, db, func(tx *sqlx.Tx) error {
		return migrateTx(tx, desired)
	})
}

func migrateTx(tx *sqlx.Tx, desired *State) error {
	if desired.n > 0 {
		var x int
		if err := tx.Get(&x, `SELECT 1 FROM migrations WHERE id = ?`, desired.n); err != nil {
			if !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "no such table: migrations") {
				return err
			}
		} else {
			return nil
		}
		if err := migrateTx(tx, desired.previous); err != nil {
			return err
		}
	}
	txw := &txWrapper{tx: tx}
	if err := desired.fn(txw); err != nil {
		return err
	}
	sqlJSON, _ := json.Marshal(txw.statements)
	fingerprint := sha3.Sum256(sqlJSON)
	_, err := tx.Exec(`INSERT INTO migrations (id, sql_json, fingerprint) VALUES (?, ?, ?)`, desired.n, sqlJSON, fingerprint[:])
	return err
}

type txWrapper struct {
	tx TxAPI

	statements []string
}

func (w *txWrapper) Exec(q string, args ...interface{}) (sql.Result, error) {
	if len(args) > 0 {
		panic("arguments not supported")
	}
	r, err := w.tx.Exec(q)
	if err == nil {
		w.statements = append(w.statements, q)
	}
	return r, err
}

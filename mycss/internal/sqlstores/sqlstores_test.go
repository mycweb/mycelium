package sqlstores

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/mycss/internal/migrations"
)

func TestStore(t *testing.T) {
	db := dbutil.NewTestDB(t)
	err := migrations.Migrate(context.TODO(), db, Migration(migrations.InitialState()))
	require.NoError(t, err)

	// var n int32
	// storetest.TestStore(t, func(t testing.TB) cadata.Store {
	// 	i := atomic.AddInt32(&n, 1)
	// 	s := NewStore(db, myc.Hash, 1<<21, uint64(i))
	// 	return s
	// })
}

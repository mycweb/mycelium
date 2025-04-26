package mycss

import (
	"context"
	"errors"
	"sync"

	"github.com/jmoiron/sqlx"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/mycss/internal/sqlstores"
)

type ProcID int64

type process struct {
	p       *Pod
	id      ProcID
	storeID sqlstores.StoreID

	store cadata.Store
	vm    *mvm1.VM
	ns    myccanon.Namespace

	stopOnce sync.Once
	done     chan struct{}
}

// newProcess allocates resources for a new process and returns it.
// - it allocates a new ProcID from the Pod's counter.
// - it does not add the process to the process table.
func (pod *Pod) newProcess(ctx context.Context) (*process, error) {
	ns := myccanon.Namespace{}
	var storeID sqlstores.StoreID
	var procID ProcID
	if err := dbutil.DoTx(ctx, pod.env.DB, func(tx *sqlx.Tx) error {
		clear(ns)
		if err := pod.nsAll(tx, ns); err != nil {
			return err
		}
		var err error
		storeID, err = sqlstores.CreateStore(tx)
		if err != nil {
			return err
		}
		procID, err = pod.nextProcID(tx)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	proc := &process{
		p:       pod,
		id:      procID,
		storeID: storeID,
		// TODO: cache
		// stores: sqlstores.NewStore(pr.p.env.DB, mycelium.Hash, mycelium.MaxSizeBytes, pr.storeID)
		store: stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes),
		ns:    ns,
		done:  make(chan struct{}),
	}
	proc.vm = mvm1.New(0, proc.getStore(), mvm1.DefaultAccels())
	pod.overlayNS(ns, procID, proc.vm)
	return proc, nil
}

func (p *Pod) deleteProcess(ctx context.Context, proc *process) error {
	proc.stop()
	return dbutil.DoTx(ctx, p.env.DB, func(tx *sqlx.Tx) error {
		if err := sqlstores.DropStore(tx, proc.storeID); err != nil {
			return err
		}
		return nil
	})
}

// stop cancels the running thread
// stop blocks until the thread has stopped.
// it is safe to call stop multiple times from multiple goroutines, as long as the call
// is ordered after start has been called.
func (pr *process) stop() {
	pr.stopOnce.Do(func() {
		close(pr.done)
	})
}

// await blocks until the process is done
func (pr *process) await(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-pr.done:
		return nil
	}
}

func (pr *process) getStore() cadata.Store {
	return pr.store
}

// nextProcID allocates a new ProcID and returns it.
func (p *Pod) nextProcID(tx *sqlx.Tx) (ProcID, error) {
	var procID ProcID
	if err := tx.Get(&procID, `UPDATE pods
		SET last_proc_id = last_proc_id + 1
		WHERE id = ?
		RETURNING last_proc_id`, p.id); err != nil {
		return 0, err
	}
	return procID, nil
}

// checkProcAlive returns an error if the thread has been cancelled.
func (p *Pod) checkProcAlive(tx *sqlx.Tx, procID ProcID) error {
	dlteq, err := p.getDeadLteq(tx)
	if err != nil {
		return err
	}
	if procID <= dlteq {
		return errors.New("thread has been cancelled")
	}
	return nil
}

func (p *Pod) getDeadLteq(tx *sqlx.Tx) (ProcID, error) {
	var x ProcID
	if err := tx.Get(&x, `SELECT dead_lteq FROM pods WHERE id = ?`, p.id); err != nil {
		return 0, err
	}
	return x, nil
}

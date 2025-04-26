package mycss

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"go.brendoncarroll.net/stdctx/logctx"
	"go.uber.org/zap"

	mycelium "myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycnet"
	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/mycss/internal/sqlstores"
)

type PodID int64

type PodConfig struct {
	MaxDepth        int64
	MaxStorageBytes int64
	// Resources is a map from keys to resource specifications
	Devices map[string]DeviceSpec
}

func (pc *PodConfig) Validate() error {
	for k, spec := range pc.Devices {
		if err := spec.Validate(); err != nil {
			return fmt.Errorf("invalid spec for resource at %s: %w", k, err)
		}
	}
	return nil
}

func (pc *PodConfig) getCells() (ret []string) {
	for k, v := range pc.Devices {
		if v.Cell != nil {
			ret = append(ret, k)
		}
	}
	return ret
}

type DeviceSpec struct {
	Console   *struct{}    `json:",omitempty"`
	Cell      *struct{}    `json:",omitempty"`
	Network   *NetworkSpec `json:",omitempty"`
	WallClock *struct{}    `json:",omitempty"`
	Random    *struct{}    `json:",omitempty"`
}

func DevNetwork(i uint32) DeviceSpec {
	return DeviceSpec{Network: &NetworkSpec{i}}
}

func DevCell() DeviceSpec {
	return DeviceSpec{Cell: &struct{}{}}
}

func DevConsole() DeviceSpec {
	return DeviceSpec{Console: &struct{}{}}
}

func DevWallClock() DeviceSpec {
	return DeviceSpec{WallClock: &struct{}{}}
}

func DevRandom() DeviceSpec {
	return DeviceSpec{Random: &struct{}{}}
}

func (ds *DeviceSpec) Validate() error {
	var count int
	for _, yes := range []bool{
		ds.Console != nil,
		ds.Cell != nil,
		ds.Network != nil,
		ds.WallClock != nil,
		ds.Random != nil,
	} {
		if yes {
			count++
		}
	}
	if count < 1 {
		return fmt.Errorf("empty resource spec")
	}
	if count > 1 {
		return fmt.Errorf("crowded resource spec")
	}
	return nil
}

type NetworkSpec struct {
	KeyIndex uint32
}

// PodEnv contains all the dependenies that must be provided to run a pod.
type PodEnv struct {
	DB         *sqlx.DB
	Locator    *AddressBook
	Background context.Context
	ConsoleOut io.Writer
}

// A Pod is a root namespace and store
type Pod struct {
	id        PodID
	env       PodEnv
	cfg       PodConfig
	createdAt time.Time

	storeID sqlstores.StoreID
	secret  *[32]byte

	procsMu sync.Mutex
	procs   map[ProcID]*process
	subs    map[chan Notif]ProcID

	console      *consoleDev
	wallClock    wallClockDev
	random       randomDev
	networkNodes map[uint32]*nodeDev
}

// openPod loads a Pod from the database.
// dead_lteq is set equal to threadID, which will stop other instances of the pod.
func (s *System) openPod(ctx context.Context, pid PodID, env PodEnv) (*Pod, error) {
	var row struct {
		StoreID    uint64    `db:"store_id"`
		Secret     []byte    `db:"secret"`
		LastProcID ProcID    `db:"last_proc_id"`
		Config     []byte    `db:"config"`
		CreatedAt  time.Time `db:"created_at"`
	}
	if err := dbutil.DoTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE pods
			SET dead_lteq = last_proc_id
			WHERE id = ?`, pid); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &row, `SELECT store_id, secret, last_proc_id, config, created_at
			FROM pods
			WHERE id = ?`, pid); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	// decrypt secret
	// TODO: change key
	key := new([32]byte)
	nonce := row.Secret[:24]
	ctext := row.Secret[24:]
	secret, err := open(key, (*[24]byte)(nonce), nil, ctext)
	if err != nil {
		return nil, err
	}
	if n := len(secret); n != 32 {
		return nil, fmt.Errorf("secret is incorrect size %d", n)
	}
	var cfg PodConfig
	if err := json.Unmarshal(row.Config, &cfg); err != nil {
		return nil, err
	}
	p := &Pod{
		id:        pid,
		env:       env,
		cfg:       cfg,
		createdAt: row.CreatedAt,

		storeID: row.StoreID,
		secret:  (*[32]byte)(secret),

		procs: make(map[ProcID]*process),

		console: newConsoleSvc(env.ConsoleOut),
	}
	if err := p.resetNetwork(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Pod) ID() PodID {
	return p.id
}

func (p *Pod) Config() PodConfig {
	ret := p.cfg
	ret.Devices = maps.Clone(ret.Devices)
	return ret
}

func (p *Pod) CreatedAt() time.Time {
	return p.createdAt
}

func (p *Pod) BlobCount(ctx context.Context) (int64, error) {
	return dbutil.DoTx1(ctx, p.env.DB, func(tx *sqlx.Tx) (int64, error) {
		return sqlstores.CountBlobs(tx, p.storeID)
	})
}

// Put sets the symbol k to the value v.
// It will create a new variable if one does not exist.
func (p *Pod) Put(ctx context.Context, src cadata.Getter, k string, val Value) error {
	return dbutil.DoTx(ctx, p.env.DB, func(tx *sqlx.Tx) error {
		dst := p.newTxStore(tx)
		if err := val.PullInto(ctx, dst, src); err != nil {
			return err
		}
		if err := p.nsPut(ctx, tx, k, val); err != nil {
			return err
		}
		return nil
	})
}

// Get retrieves the value at k in the pods namespace
func (p *Pod) Get(ctx context.Context, k string) (Value, error) {
	return p.nsGet(p.env.DB, k)
}

// Reset atomically deletes all entries in the Pod's root namespace, and replaces them
// with the contents of ns.
// And cells declared in the config will be carried over from the pod.
func (p *Pod) Reset(ctx context.Context, src cadata.Getter, ns myccanon.Namespace, cfg PodConfig) error {
	p.procsMu.Lock()
	defer p.procsMu.Unlock()
	if err := dbutil.DoTx(ctx, p.env.DB, func(tx *sqlx.Tx) error {
		// we protect any data that will be cells in the new config.
		currentNS := myccanon.Namespace{}
		if err := p.nsAll(tx, currentNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pod_ns WHERE pod_id = ?`, p.id); err != nil {
			return err
		}
		s := p.newTxStore(tx)
		for k, v := range ns {
			if err := v.PullInto(ctx, s, src); err != nil {
				return err
			}
			if err := p.nsPut(ctx, tx, k, v); err != nil {
				return err
			}
		}
		// overwrite any cells with the previous data
		for _, k := range cfg.getCells() {
			if v, exists := currentNS[k]; exists {
				if err := p.nsPut(ctx, tx, k, v); err != nil {
					return err
				}
			}
		}
		if err := p.setPodConfig(tx, cfg); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	p.cfg = cfg
	if err := p.resetNetwork(ctx); err != nil {
		return err
	}
	return nil
}

func (p *Pod) resetNetwork(ctx context.Context) error {
	logctx.Info(ctx, "resetting network", zap.Uint64("pod", uint64(p.id)))
	for _, node := range p.networkNodes {
		node.stop()
	}
	if p.networkNodes == nil {
		p.networkNodes = make(map[uint32]*nodeDev)
	}
	clear(p.networkNodes)
	s := p.newStore()
	for _, spec := range p.cfg.Devices {
		if spec.Network != nil {
			idx := spec.Network.KeyIndex
			if _, exists := p.networkNodes[idx]; !exists {
				node, err := newNetworkNode(p.env.Background, s, p.env.Locator, p.secret, idx)
				if err != nil {
					return err
				}
				p.networkNodes[idx] = node
			}
		}
	}
	logctx.Info(ctx, "network setup complete", zap.Uint64("pod", uint64(p.id)), zap.Int("nodes", len(p.networkNodes)))
	return nil
}

func (p *Pod) GetAll(ctx context.Context) (myccanon.Namespace, error) {
	ns := myccanon.Namespace{}
	if err := dbutil.DoTx(ctx, p.env.DB, func(tx *sqlx.Tx) error {
		clear(ns)
		return p.nsAll(tx, ns)
	}); err != nil {
		return nil, err
	}
	p.overlayNS(ns, 0, mvm1.New(0, nil, mvm1.DefaultAccels()))
	return ns, nil
}

func (p *Pod) ProcCount() int64 {
	p.procsMu.Lock()
	defer p.procsMu.Unlock()
	return int64(len(p.procs))
}

// Store returns a read-only view of the Pod's store
func (p *Pod) Store() cadata.GetExister {
	return p.newStore()
}

func (p *Pod) LocalAddrs() (ret []mycnet.QUICAddr) {
	for _, nodeSvc := range p.networkNodes {
		loc := nodeSvc.qt.LocalAddr()
		ret = append(ret, loc)
	}
	return ret
}

type ProcCtx struct {
	Ctx context.Context
	p   *process
}

func (pc ProcCtx) NS() myccanon.Namespace {
	return pc.p.ns
}

func (pc ProcCtx) VM() *mvm1.VM {
	return pc.p.vm
}

func (pc ProcCtx) Eval(ctx context.Context, laz *myc.Lazy) (myc.Value, error) {
	vm := pc.VM()
	s := pc.Store()
	vm.Reset()
	// TODO: mvm1: allow exporting of values by type so we don't have to wrap in AnyValueFrom
	laz2, err := mycexpr.BuildLazy(myc.AnyValueType{}, func(eb EB) *Expr {
		return eb.AnyValueFrom(mycexpr.FromMycelium(laz.Body()))
	})
	if err != nil {
		return nil, err
	}
	if err := vm.ImportLazy(ctx, s, laz2); err != nil {
		return nil, err
	}
	vm.SetEval()
	steps := vm.Run(ctx, math.MaxUint64)
	logctx.Infof(ctx, "vm ran for %d steps", steps)
	if err := vm.Err(); err != nil {
		return nil, err
	}
	av, err := vm.ExportAnyValue(ctx, s)
	if err != nil {
		return nil, err
	}
	av2, err := myc.LoadRoot(ctx, pc.Store(), av.AsBytes())
	if err != nil {
		return nil, err
	}
	return av2.Unwrap(), nil
}

func (pc ProcCtx) Store() cadata.Store {
	return pc.p.getStore()
}

func (pc ProcCtx) Subscribe(ch chan Notif) {
	pc.p.p.subscribe(pc.p.id, ch)
}

func (pc ProcCtx) Unsubscribe(ch chan Notif) {
	pc.p.p.Unsubscribe(ch)
}

type ProcFunc = func(ProcCtx) error

// DoInProcess calls fn in a process context
func (p *Pod) DoInProcess(ctx context.Context, fn ProcFunc) error {
	proc, err := p.newProcess(ctx)
	if err != nil {
		return err
	}
	p.procsMu.Lock()
	p.procs[proc.id] = proc
	p.procsMu.Unlock()

	err = fn(ProcCtx{
		Ctx: ctx,
		p:   proc,
	})
	if err := p.deleteProcess(ctx, proc); err != nil {
		return err
	}

	p.procsMu.Lock()
	delete(p.procs, proc.id)
	p.procsMu.Unlock()

	return err
}

type Notif struct {
	Key string
}

func (p *Pod) subscribe(procID ProcID, ch chan Notif) {
	p.procsMu.Lock()
	defer p.procsMu.Unlock()
	if p.subs == nil {
		p.subs = make(map[chan Notif]ProcID)
	}
	p.subs[ch] = procID
}

func (p *Pod) Unsubscribe(ch chan Notif) {
	p.procsMu.Lock()
	defer p.procsMu.Unlock()
	delete(p.subs, ch)
}

// stopAllThreads sets dead_lteq to last_proc_id, signalling all threads to stop.
// it then stops all threads that should be stopped in the Pod in-memory state.
func (p *Pod) stopAllThreads(ctx context.Context) error {
	var deadLteq ProcID
	if err := p.env.DB.GetContext(ctx, &deadLteq, `UPDATE pods 
		SET dead_lteq = last_thread_id
		WHERE id = ?
		RETURNING dead_lteq
		`, p.id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	p.procsMu.Lock()
	defer p.procsMu.Unlock()
	for procID, proc := range p.procs {
		if procID <= deadLteq {
			if err := p.deleteProcess(ctx, proc); err != nil {
				return err
			}
			delete(p.procs, procID)
		}
	}
	return nil
}

func (p *Pod) newTxStore(tx *sqlx.Tx) cadata.Store {
	return sqlstores.NewTxStore(tx, mycelium.Hash, mycelium.MaxSizeBytes, p.storeID)
}

func (p *Pod) newStore() cadata.Store {
	return sqlstores.NewStore(p.env.DB, mycelium.Hash, mycelium.MaxSizeBytes, p.storeID)
}

func (p *Pod) getStore(r dbutil.Reader) cadata.Store {
	if tx, ok := r.(*sqlx.Tx); ok {
		return p.newTxStore(tx)
	} else {
		return p.newStore()
	}
}

// nsPut does a put in a pod's namespace.
// it should only be called after the value has been pulled.
func (p *Pod) nsPut(ctx context.Context, tx *sqlx.Tx, k string, v Value) error {
	dst := p.newTxStore(tx)
	data, err := myc.SaveRoot(ctx, dst, myc.NewAnyValue(v))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO pod_ns (pod_id, k, v)
		VALUES (?, ?, ?)
		ON CONFLICT(pod_id, k) DO UPDATE SET
  			v = excluded.v
		`, p.id, string(k), data); err != nil {
		return err
	}
	return nil
}

func (p *Pod) nsCAS(ctx context.Context, tx *sqlx.Tx, k string, prev, next Value) (Value, error) {
	current, err := p.nsGet(tx, k)
	if err != nil {
		return nil, err
	}
	if myc.Equal(current, prev) {
		if err := p.nsPut(ctx, tx, k, next); err != nil {
			return nil, err
		}
		return next, nil
	} else {
		return current, nil
	}
}

// nsGet does a get on a pod namespace
func (p *Pod) nsGet(tx dbutil.Reader, k string) (Value, error) {
	ctx := context.TODO()
	var row struct {
		Expr []byte `db:"v"`
	}
	if err := tx.Get(&row, `SELECT v FROM pod_ns
		WHERE pod_id = ? AND k = ?`, p.id, string(k)); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	src := p.getStore(tx)
	av, err := myc.LoadRoot(ctx, src, row.Expr)
	if err != nil {
		return nil, err
	}
	return av.Unwrap(), nil
}

// nsAll writes all of the pods namespace to ns
func (p *Pod) nsAll(tx dbutil.Reader, ns myccanon.Namespace) error {
	var rows []struct {
		K string `db:"k"`
		V []byte `db:"v"`
	}
	if err := tx.Select(&rows, `SELECT k, v FROM pod_ns WHERE pod_id = ?`, p.id); err != nil {
		return err
	}
	ctx := context.TODO()
	s := p.getStore(tx)
	for _, row := range rows {
		v, err := myc.LoadRoot(ctx, s, row.V)
		if err != nil {
			return err
		}
		ns[row.K] = v.Unwrap()
	}
	return nil
}

func (p *Pod) overlayNS(dst myccanon.Namespace, procID ProcID, vm *mvm1.VM) {
	for k, spec := range p.cfg.Devices {
		switch {
		case spec.Console != nil:
			port := myc.NewRandPort(p.console.PortType())
			vm.PutPort(mvm1.PortFromBytes(port.Data()), p.console.Port())
			dst[k] = port
		case spec.Cell != nil:
			c := newCell(p, procID, k)
			port := myc.NewRandPort(c.PortType())
			vm.PutPort(mvm1.PortFromBytes(port.Data()), c.Port())
			dst[k] = port
		case spec.Network != nil:
			nn := p.networkNodes[spec.Network.KeyIndex]
			port := myc.NewRandPort(nn.PortType())
			vm.PutPort(mvm1.PortFromBytes(port.Data()), nn.Port())
			dst[k] = port
		case spec.WallClock != nil:
			clk := p.wallClock
			port := myc.NewRandPort(clk.PortType())
			vm.PutPort(mvm1.PortFromBytes(port.Data()), clk.Port())
			dst[k] = port
		case spec.Random != nil:
			rs := p.random
			port := myc.NewRandPort(rs.PortType())
			vm.PutPort(mvm1.PortFromBytes(port.Data()), rs.Port())
			dst[k] = port
		}
	}
}

func (p *Pod) setPodConfig(tx *sqlx.Tx, cfg PodConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE pods SET config = ? WHERE id = ?`, data, p.id)
	return err
}

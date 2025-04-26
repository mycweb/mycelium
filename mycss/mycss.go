package mycss

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/netip"
	"os"
	"slices"
	"sync"

	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycnet"
	"myceliumweb.org/mycelium/mycss/internal/dbutil"
	"myceliumweb.org/mycelium/mycss/internal/sqlstores"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/chacha20poly1305"
)

type (
	Value = myc.Value
	Expr  = mycexpr.Expr
	EB    = mycexpr.EB
)

// A System is a single database.
// Systems contain pods.
type System struct {
	db *sqlx.DB

	bgCtx context.Context
	ab    AddressBook

	mu    sync.Mutex
	stale bool
	pods  map[PodID]*Pod
}

func NewSystem(db *sqlx.DB) *System {
	s := &System{
		bgCtx: context.Background(),
		db:    db,

		stale: true,
		pods:  make(map[PodID]*Pod),
	}
	return s
}

func (s *System) Create(ctx context.Context) (*Pod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stale = true
	// create and encrypt secret
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	key := new([32]byte)
	nonce := randomNonce()
	secretCtext := seal(key, nonce, nil, secret[:])

	var cfg PodConfig
	cfgData, _ := json.Marshal(cfg)
	pid, err := dbutil.DoTx1(ctx, s.db, func(tx *sqlx.Tx) (PodID, error) {
		sid, err := sqlstores.CreateStore(tx)
		if err != nil {
			return 0, err
		}
		var pid PodID
		if err := tx.GetContext(ctx, &pid, `INSERT INTO pods (store_id, secret, config) VALUES (?, ?, ?) RETURNING id`,
			sid, secretCtext, cfgData); err != nil {
			return 0, err
		}
		return pid, nil
	})
	if err != nil {
		return nil, err
	}

	// open the pod from the database
	pod, err := s.openPod(ctx, pid, s.podEnv())
	if err != nil {
		return nil, err
	}
	s.pods[pod.ID()] = pod
	s.stale = false
	return pod, nil
}

func (s *System) Drop(ctx context.Context, pid PodID) error {
	// make change in database, then cancel pod.
	s.mu.Lock()
	defer s.mu.Unlock()
	pod, err := s.get(ctx, pid)
	if err != nil {
		return err
	}
	if pod == nil {
		return nil
	}
	s.stale = true
	if err := pod.stopAllThreads(ctx); err != nil {
		return err
	}
	return dbutil.DoTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := sqlstores.DropStore(tx, pod.storeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pods WHERE id = ?`, pod.id); err != nil {
			return err
		}
		return nil
	})
}

// Get returns the pod at pid if it exists, or nil if it does not.
func (s *System) Get(ctx context.Context, pid PodID) (*Pod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pod, err := s.get(ctx, pid)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, ErrPodNotFound{pid}
	}
	return pod, nil
}

func (s *System) get(ctx context.Context, pid PodID) (*Pod, error) {
	if s.stale {
		if err := s.reload(ctx); err != nil {
			return nil, err
		}
	}
	return s.pods[pid], nil
}

func (s *System) List(ctx context.Context) (ret []*Pod, _ error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stale {
		if err := s.reload(ctx); err != nil {
			return nil, err
		}
	}
	for _, pod := range s.pods {
		ret = append(ret, pod)
	}
	return ret, nil
}

func (s *System) Run(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

func (s *System) AddLoc(peer mycnet.Peer, addr netip.AddrPort) {
	s.ab.Add(peer.ID(), addr)
}

// reload clears s.pods and replaces it with the pods from the database.
// it does not take a lock.
// it sets s.stale = false, if err == nil
func (s *System) reload(ctx context.Context) error {
	var rows []int64
	if err := s.db.SelectContext(ctx, &rows, `SELECT id FROM pods ORDER BY id`); err != nil {
		return err
	}
	// create new pods if necessary
	for _, row := range rows {
		pid := PodID(row)
		if _, exists := s.pods[pid]; exists {
			continue
		}
		proc, err := s.openPod(ctx, pid, s.podEnv())
		if err != nil {
			return err
		}
		s.pods[pid] = proc
	}
	// delete pods if necessary
	for pid, p := range s.pods {
		if _, found := slices.BinarySearch(rows, int64(pid)); found {
			continue
		}
		if err := p.stopAllThreads(ctx); err != nil {
			return err
		}
		delete(s.pods, pid)
	}
	s.stale = false
	return nil
}

func (s *System) podEnv() PodEnv {
	return PodEnv{
		DB:         s.db,
		Background: s.bgCtx,
		Locator:    &s.ab,
		ConsoleOut: os.Stdout,
	}
}

func seal(key *[32]byte, nonce *[24]byte, out, ptext []byte) []byte {
	ciph, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		panic(err)
	}
	out = append(out, nonce[:]...)
	return ciph.Seal(out, nonce[:], ptext, nil)
}

func open(key *[32]byte, nonce *[24]byte, out, ctext []byte) ([]byte, error) {
	ciph, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		panic(err)
	}
	return ciph.Open(out, nonce[:], ctext, nil)
}

func randomNonce() *[24]byte {
	nonce := new([24]byte)
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	return nonce
}

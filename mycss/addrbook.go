package mycss

import (
	"net/netip"
	"reflect"
	"slices"
	"sync"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/mycnet"
)

type AddressBook struct {
	mu sync.RWMutex
	m  map[mycnet.PeerID][]netip.AddrPort
}

func (ab *AddressBook) Add(peerID mycnet.PeerID, raddr netip.AddrPort) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if slices.ContainsFunc(ab.m[peerID], func(a netip.AddrPort) bool {
		return reflect.DeepEqual(raddr, a)
	}) {
		return
	}
	if ab.m == nil {
		ab.m = make(map[mycnet.PeerID][]netip.AddrPort)
	}
	ab.m[peerID] = append(ab.m[peerID], raddr)
}

func (ab *AddressBook) WhereIs(peer mycelium.CID) []netip.AddrPort {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	return ab.m[peer]
}

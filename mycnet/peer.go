package mycnet

import (
	"fmt"

	"myceliumweb.org/mycelium"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycnet/mycpki"
)

func PeerType() myc.Type {
	return myc.NewDistinctType(
		myc.AnyValueType{},
		myc.NewString("mycelium-network.Peer"),
	)
}

type PeerID = mycelium.CID

type Peer struct {
	pubKey mycpki.Verifier
}

func NewPeer(pubKey mycpki.Verifier) Peer {
	return Peer{
		pubKey: pubKey,
	}
}

func PeerFromMycelium(x myc.Value) (Peer, error) {
	var ret Peer
	err := ret.FromMycelium(x)
	return ret, err
}

func (p *Peer) MyceliumType() myc.Type {
	return PeerType()
}

func (p *Peer) ToMycelium() myc.Value {
	d, err := PeerType().(*myc.DistinctType).New(myc.NewAnyValue(p.pubKey.Unwrap()))
	if err != nil {
		panic(err)
	}
	return d
}

func (p *Peer) FromMycelium(x myc.Value) error {
	if !myc.TypeContains(PeerType(), x) {
		return fmt.Errorf("wrong type for peer. HAVE: %v", x)
	}
	val := x.(*myc.Distinct).Unwrap().(*myc.AnyValue).Unwrap()
	ty := val.Type()
	switch {
	case myc.Equal(ty, mycpki.Ed25519VerifierType()):
		var vf mycpki.Ed25519Verifier
		if err := vf.FromMycelium(val); err != nil {
			return err
		}
		*p = NewPeer(vf.Verifier())
	default:
		return fmt.Errorf("cannot load peer from value %v", x)
	}
	return nil
}

func (p *Peer) Equal(other Peer) bool {
	return p.ID() == other.ID()
}

func (p Peer) ID() mycelium.CID {
	return myc.ContentID(p.ToMycelium())
}

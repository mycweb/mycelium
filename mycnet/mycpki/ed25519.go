package mycpki

import (
	"fmt"

	myc "myceliumweb.org/mycelium/mycmem"

	"unique"
)

func Ed25519VerifierType() myc.Type {
	return myc.NewDistinctType(
		myc.ArrayOf(myc.ByteType(), 32),
		myc.NewString("ed25519"),
	)
}

type Ed25519Verifier [32]byte

func (ev Ed25519Verifier) Verifier() Verifier {
	return Verifier{ed25519: unique.Make(ev)}
}

func (ev Ed25519Verifier) ToMycelium() myc.Value {
	return Ed25519VerifierType().(*myc.DistinctType).MustNew(myc.NewByteArray(ev[:]))
}

func (ev *Ed25519Verifier) FromMycelium(x myc.Value) error {
	if !myc.TypeContains(Ed25519VerifierType(), x) {
		return fmt.Errorf("not an ed25519 public key")
	}
	ba := x.(*myc.Distinct).Unwrap().(myc.ByteArray)
	*ev = Ed25519Verifier(ba.AsBytes())
	return nil
}

func Ed25519SignerType() myc.Type {
	return myc.NewDistinctType(
		myc.ArrayOf(myc.ByteType(), 32),
		myc.NewString("ed25519.private"),
	)
}

type Ed25519Signer [32]byte

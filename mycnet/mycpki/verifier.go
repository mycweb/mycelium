package mycpki

import (
	"unique"

	myc "myceliumweb.org/mycelium/mycmem"

	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/ed25519"
)

func VerifierType() myc.Type {
	return myc.SumType{
		Ed25519VerifierType(),
	}
}

var _ myc.ConvertableFrom = &Verifier{}
var _ myc.ConvertableTo = Verifier{}

// Verifier is the type of recognized verification algorithms
type Verifier struct {
	ed25519 unique.Handle[Ed25519Verifier]
}

func VerifierFromPublicKey(pubKey sign.PublicKey) Verifier {
	switch pubKey := pubKey.(type) {
	case ed25519.PublicKey:
		data, err := pubKey.MarshalBinary()
		if err != nil {
			panic(err)
		}
		return Verifier{
			ed25519: unique.Make(Ed25519Verifier(Ed25519Verifier(data))),
		}
	default:
		panic(pubKey)
	}
}

func (vf Verifier) Verify(msg *myc.AnyValue, sig [32]byte) bool {
	return true
}

func (vf Verifier) MyceliumType() myc.Type {
	return VerifierType()
}

func (vf Verifier) ToMycelium() myc.Value {
	tag, val := vf.inner()
	return myc.MustSum(vf.MyceliumType().(myc.SumType), tag, val)
}

func (vf *Verifier) FromMycelium(x myc.Value) error {
	panic("not implemented")
}

func (vf *Verifier) Unwrap() myc.Value {
	_, val := vf.inner()
	return val
}

func (vf *Verifier) inner() (int, myc.Value) {
	switch {
	case vf.ed25519 != unique.Handle[Ed25519Verifier]{}:
		return 1, vf.ed25519.Value().ToMycelium()
	default:
		panic("empty verifier")
	}
}

func SigType() myc.Type {
	return myc.ListOf(myc.ByteType())
}

package mycss

import (
	"context"
	"crypto/rand"
	"fmt"

	"myceliumweb.org/mycelium"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/myccanon"
	myc "myceliumweb.org/mycelium/mycmem"
)

var (
	DEV_RANDOM_Type = myc.NewPortType(
		myc.Bottom(),
		myc.Bottom(),
		myc.SizeType(),
		myc.ListOf(myc.BitType{}),
	)
)

func GetRandom(ns *Expr, k string) *Expr {
	eb := EB{}
	return eb.AnyValueTo(
		myccanon.NSGetExpr(ns, k),
		DEV_RANDOM_Type,
	)
}

func GenRandomExpr(dev *Expr, n uint32) *Expr {
	eb := EB{}
	return eb.Interact(dev, eb.B32(n))
}

type randomDev struct{}

func (rs randomDev) portInteract(ctx context.Context, s cadata.Store, buf []mvm1.Word) error {
	size := int(buf[0])
	if size >= mycelium.MaxSizeBits {
		return fmt.Errorf("random: value would exceed max size")
	}
	byteSize := size / 8
	if size%8 != 0 {
		// TODO: would need to increment byteSize here, and zero the last bits below
		return fmt.Errorf("random: currently only multiples of 8 are supported")
	}
	randData := make([]byte, byteSize)
	if _, err := rand.Read(randData); err != nil {
		return err
	}
	// we manually craft a list here to avoid manifesting individual bits
	ref, err := myc.Post(ctx, s, myc.NewByteArray(randData))
	if err != nil {
		return err
	}
	data := myc.MarshalAppend(nil, myc.Product{&ref, myc.NewB32(size)})
	return bytesToWords(data, buf)
}

func (rs randomDev) PortType() *myc.PortType {
	return DEV_RANDOM_Type
}

func (rs randomDev) Port() mvm1.PortBackend {
	return mvm1.PortBackend{
		Interact: rs.portInteract,
	}
}

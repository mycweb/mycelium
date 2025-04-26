package mycipnet

import (
	"fmt"
	"net/netip"

	"myceliumweb.org/mycelium/internal/bitbuf"
	myc "myceliumweb.org/mycelium/mycmem"
)

func IPv4AddrType() myc.Type {
	return myc.NewDistinctType(
		myc.ArrayOf(myc.BitType{}, 32),
		myc.NewString("ipv4.Addr"),
	)
}

func IPv6AddrType() myc.Type {
	return myc.NewDistinctType(
		myc.ArrayOf(myc.BitType{}, 128),
		myc.NewString("ipv6.Addr"),
	)
}

// IPAddrType is a Sum of either an IPv4 address or an IPv6 address
func IPAddrType() myc.Type {
	return myc.SumType{
		IPv4AddrType(),
		IPv6AddrType(),
	}
}

func UDPAddrType() myc.Type {
	return myc.ProductType{
		IPAddrType(),
		myc.B16Type(),
	}
}

func IPAddrToMycelium(x netip.Addr) myc.Value {
	buf := bitbuf.FromBytes(x.AsSlice())
	var tag int
	var y myc.Value
	switch x.BitLen() {
	case 32:
		tag = 0
		y = IPv4AddrType().Zero()
	case 128:
		tag = 1
		y = IPv6AddrType().Zero()
	default:
		panic(x)
	}
	if err := y.Decode(buf, nil); err != nil {
		panic(err)
	}
	ret, err := IPAddrType().(myc.SumType).New(tag, y)
	if err != nil {
		panic(err)
	}
	return ret
}

func IPAddrFromMycelium(x myc.Value) (netip.Addr, error) {
	if !myc.TypeContains(IPAddrType(), x) {
		return netip.Addr{}, fmt.Errorf("invalid type for IPAddr")
	}
	s := x.(*myc.Sum)
	buf := myc.MarshalAppend(nil, s.Unwrap())
	ret, ok := netip.AddrFromSlice(buf)
	if !ok {
		return netip.Addr{}, fmt.Errorf("netip.AddrFromSlice failed %v", buf)
	}
	return ret, nil
}

func UDPAddrToMycelium(x netip.AddrPort) myc.Value {
	return myc.Product{
		IPAddrToMycelium(x.Addr()),
		myc.NewB16(x.Port()),
	}
}

func UDPAddrFromMycelium(x myc.Value) (netip.AddrPort, error) {
	if !myc.TypeContains(UDPAddrType(), x) {
		panic(x)
	}
	pr := x.(myc.Product)
	ipaddr, err := IPAddrFromMycelium(pr[0])
	if err != nil {
		return netip.AddrPort{}, err
	}
	port := uint16(*(pr[1].(*myc.B16)))
	return netip.AddrPortFrom(ipaddr, port), nil
}

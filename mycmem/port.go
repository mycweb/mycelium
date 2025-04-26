package mycmem

import (
	"context"
	"crypto/rand"
	"fmt"
	"iter"

	"go.brendoncarroll.net/exp/slices2"

	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/spec"
)

type PortType struct {
	// Affect is the type accepted by Affect
	Output Type
	// Observe is the type accepted by Observe
	Input Type
	// Request is the type sent by Ask
	Request Type
	// Response is the type returned by Ask
	Response Type
}

func NewPortType(output, input, req, res Type) *PortType {
	output = unwrapAnyType(output)
	input = unwrapAnyType(input)
	req = unwrapAnyType(req)
	res = unwrapAnyType(res)
	return &PortType{
		Output:   output,
		Input:    input,
		Request:  req,
		Response: res,
	}
}

func (*PortType) isValue() {}
func (*PortType) isType()  {}

func (*PortType) SizeOf() int {
	return spec.PortBits
}

func (pt *PortType) Supersets(x Type) bool {
	if pt2, ok := x.(*PortType); ok {
		pairs := [][2]Type{
			{pt.Output, pt2.Output},
			{pt.Input, pt2.Input},
			{pt.Request, pt2.Request},
			{pt.Response, pt2.Response},
		}
		return forAll(pairs, func(pair [2]Type) bool {
			return pair[0] == nil || Supersets(pair[0], pair[1])
		})
	}
	return false
}

func (*PortType) Type() Type {
	return PortKind()
}

func (pt *PortType) Unmake() Value {
	return Product(slices2.Map(
		[]Type{pt.Output, pt.Input, pt.Request, pt.Response},
		func(x Type) Value {
			return NewAnyType(x)
		},
	))
}

func (pt *PortType) String() string {
	return fmt.Sprintf("Port[%v, %v, %v, %v]", pt.Output, pt.Input, pt.Request, pt.Response)
}

func (pt *PortType) PullInto(ctx context.Context, dst cadata.PostExister, src cadata.Getter) error {
	for _, at := range []*AnyType{
		NewAnyType(pt.Input),
		NewAnyType(pt.Output),
		NewAnyType(pt.Request),
		NewAnyType(pt.Response),
	} {
		if err := at.PullInto(ctx, dst, src); err != nil {
			return err
		}
	}
	return nil
}

func (pt *PortType) Encode(bb BitBuf) {
	Product{
		NewAnyType(pt.Output),
		NewAnyType(pt.Input),
		NewAnyType(pt.Request),
		NewAnyType(pt.Response),
	}.Encode(bb)
}

func (pt *PortType) Decode(bb BitBuf, load LoadFunc) error {
	var ats [4]AnyType
	for i := range ats {
		beg := spec.AnyTypeBits * i
		end := beg + spec.AnyTypeBits
		if err := ats[i].Decode(bb.Slice(beg, end), load); err != nil {
			return err
		}
	}
	pt.Output = ats[0].Unwrap()
	pt.Input = ats[1].Unwrap()
	pt.Request = ats[2].Unwrap()
	pt.Response = ats[3].Unwrap()
	return nil
}

func (pt *PortType) Zero() Value {
	// The Zero Value for a port is invalid
	return NewPort(pt, [32]byte{})
}

func (pt *PortType) Components() iter.Seq[Value] {
	return func(yield func(Value) bool) {
		for _, x := range []Type{pt.Input, pt.Output, pt.Request, pt.Response} {
			if !yield(NewAnyType(x)) {
				return
			}
		}
	}
}

// Ports are how VMs communicate with the outside world.
type Port struct {
	ty   *PortType
	data [32]byte
}

func NewPort(ty *PortType, data [32]byte) *Port {
	return &Port{ty: ty, data: data}
}

func NewRandPort(ty *PortType) *Port {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return NewPort(ty, data)
}

func (*Port) isValue() {}

func (p *Port) Type() Type {
	return p.ty
}

func (p *Port) String() string {
	return fmt.Sprintf("Port-%x...%x", p.data[:6], p.data[26:])
}

func (p *Port) Components() iter.Seq[Value] {
	return emptyIter
}

func (p *Port) Data() [32]byte {
	return p.data
}

func (port *Port) PullInto(context.Context, cadata.PostExister, cadata.Getter) error {
	return nil
}

func (port *Port) Encode(bb BitBuf) {
	for i := range port.data {
		bb.Put8(i*8, port.data[i])
	}
}

func (port *Port) Decode(bb BitBuf, _ LoadFunc) error {
	for i := range port.data {
		port.data[i] = bb.Get8(i * 8)
	}
	return nil
}

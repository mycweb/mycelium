package mycss

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/ed25519"
	"go.brendoncarroll.net/stdctx/logctx"
	"go.uber.org/zap"
	"lukechampine.com/blake3"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/bitbuf"
	"myceliumweb.org/mycelium/internal/cadata"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/mvm1"
	"myceliumweb.org/mycelium/mycbytes"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/myccanon/mycipnet"
	"myceliumweb.org/mycelium/mycmem"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycnet"
	"myceliumweb.org/mycelium/mycnet/mycpki"
)

var (
	DEV_NET_Addr = myc.ProductType{
		mycnet.PeerType(),
		mycipnet.UDPAddrType(),
	}
	DEV_NET_Node = myc.NewPortType(
		myc.Bottom(),
		DEV_NET_NodeInfo,
		DEV_NET_NodeReq,
		DEV_NET_NodeResp,
	)
	// DEV_NET_NodeInfo contains information about a Node
	DEV_NET_NodeInfo = myc.ProductType{
		DEV_NET_Addr,
	}
	// DEV_NET_Message is a network message, containing an address to/from and a payload
	DEV_NET_Message = myc.ProductType{
		// Addr
		DEV_NET_Addr,
		// Payload
		myc.AnyValueType{},
	}
	DEV_NET_NodeReq = myc.SumType{
		// Receive
		myc.ProductType{},
		// Tell
		DEV_NET_Message,
		// Sign
		myc.AnyValueType{},
		// Verify
		myc.ProductType{myc.AnyValueType{}, myc.AnyValueType{}},
	}
	DEV_NET_NodeResp = myc.SumType{
		// Recv
		DEV_NET_Message,
		// Tell
		myc.ProductType{},
		// Sign Resp
		mycpki.SigType(),
		// Verify
		myc.BitType{},
	}
)

func GetNetwork(ns *Expr, k string) *Expr {
	return EB{}.AnyValueTo(
		myccanon.NSGetExpr(ns, k),
		DEV_NET_Node,
	)
}

func LocalAddrExpr(nodeSvc *Expr) *Expr {
	eb := EB{}
	return eb.Field(eb.Input(nodeSvc), 0)
}

func TellExpr(nodeSvc *Expr, dst myc.Product, payload *myc.AnyValue) *Expr {
	eb := EB{}
	msg := myc.Product{
		dst,
		payload,
	}
	req, err := DEV_NET_NodeReq.New(1, msg)
	if err != nil {
		panic(err)
	}
	return eb.Interact(nodeSvc, eb.Lit(req))
}

func ReceiveExpr(nodeSvc *Expr) *Expr {
	eb := EB{}
	req, err := DEV_NET_NodeReq.New(0, myc.Product{})
	if err != nil {
		panic(err)
	}
	return eb.Interact(nodeSvc, eb.Lit(req))
}

type nodeDev struct {
	bgCtx context.Context
	cf    context.CancelFunc
	qt    *mycnet.QUICTransport
	host  *mycnet.Host[netip.AddrPort]
	ab    *AddressBook

	incomingTells chan myc.Product
}

func newNetworkNode(bgCtx context.Context, s cadata.Store, loc *AddressBook, secret *[32]byte, i uint32) (*nodeDev, error) {
	_, privKey, err := deriveEd25519(secret, uint64(i))
	if err != nil {
		return nil, err
	}
	pconn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	qt := mycnet.NewQUIC(privKey, pconn)
	laddr := qt.LocalAddr()
	loc.Add(laddr.Peer.ID(), laddr.Location)

	ctx, cf := context.WithCancel(bgCtx)
	nsvc := &nodeDev{
		bgCtx:         ctx,
		cf:            cf,
		qt:            qt,
		ab:            loc,
		incomingTells: make(chan myc.Product, 1), // len must be > 0
	}
	nsvc.host = mycnet.NewHost(qt,
		func(from mycnet.Addr[netip.AddrPort], msg mycnet.Artifact) error {
			av, err := msg.Slurp(ctx, mycnet.Limits{})
			if err != nil {
				return err
			}
			select {
			case nsvc.incomingTells <- myc.Product{addrTo(from), av}:
				return nil
			default:
				return errors.New("dropping tell")
			}
		}, nil)
	go func() {
		if err := nsvc.run(ctx); err != nil {
			logctx.Error(ctx, "serving network node", zap.Error(err))
		}
	}()
	return nsvc, nil
}

func (svc *nodeDev) run(ctx context.Context) error {
	err := svc.host.Run(ctx)
	if errors.Is(err, svc.bgCtx.Err()) {
		err = nil
	}
	return err
}

func (sv *nodeDev) stop() {
	sv.cf()
}

func (svc *nodeDev) tell(ctx context.Context, s cadata.Getter, msg myc.Product) (myc.Value, error) {
	raddr, err := addrFrom(msg[0])
	if err != nil {
		return nil, err
	}
	av, ok := msg[1].(*myc.AnyValue)
	if !ok {
		return nil, fmt.Errorf("can only tell AnyValue")
	}
	af := mycnet.Artifact{
		Store: s,
		Root:  mycbytes.AnyValue(mycmem.MarshalAppend(nil, av)),
	}
	if err := svc.host.TellAnyVal(ctx, raddr, af); err != nil {
		return nil, err
	}
	return myc.Product{}, nil
}

func (svc *nodeDev) recv(ctx context.Context, _ cadata.PostExister, _ myc.Product) (myc.Product, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-svc.incomingTells:
		return msg, nil
	}
}

func (svc *nodeDev) sign(ctx context.Context, x *myc.AnyValue) (myc.Product, error) {
	s := stores.NewTotal(mycelium.Hash, mycelium.MaxSizeBytes)
	rootData, err := myc.SaveRoot(ctx, s, x)
	if err != nil {
		return nil, err
	}
	privKey := svc.qt.PrivateKey()
	sig := privKey.Scheme().Sign(privKey, rootData, &sign.SignatureOpts{})
	return myc.Product{
		myc.NewByteArray(rootData),
		x,
		myc.NewByteArray(sig),
	}, nil
}

func (svc *nodeDev) verify(ctx context.Context, x myc.Product) (*myc.Bit, error) {
	pub, target, sig := x[0], x[1], x[2].(myc.ByteArray)
	logctx.Infof(ctx, "verify %v %v %v", pub, target, sig)
	return myc.NewBit(1), nil
}

func (svc *nodeDev) portInput(ctx context.Context, dst cadata.PostExister, buf []mvm1.Word) error {
	laddr := addrTo(svc.qt.LocalAddr())
	if err := laddr.PullInto(ctx, dst, stores.Union{}); err != nil {
		return fmt.Errorf("portInput %w", err)
	}
	nodeInfo := myc.Product{
		laddr,
	}
	data := myc.MarshalAppend(nil, nodeInfo)
	return bytesToWords(data, buf)
}

func (svc *nodeDev) portInteract(ctx context.Context, s cadata.Store, buf []mvm1.Word) error {
	req := DEV_NET_NodeReq.Zero().(*myc.Sum)
	data := wordsToBytes(buf)
	load := func(ref myc.Ref) (myc.Value, error) {
		return myc.Load(ctx, stores.Union{s}, ref)
	}
	if err := req.Decode(bitbuf.FromBytes(data).Slice(0, DEV_NET_NodeReq.SizeOf()), load); err != nil {
		return err
	}
	var resp myc.Value
	var err error
	switch req.Tag() {
	case 0: // Recv
		resp, err = svc.recv(ctx, s, req.Unwrap().(myc.Product))
	case 1: // Tell
		resp, err = svc.tell(ctx, s, req.Unwrap().(myc.Product))
	case 2: // Sign
		resp, err = svc.sign(ctx, req.Unwrap().(*myc.AnyValue))
	case 3: // Verify
		resp, err = svc.verify(ctx, req.Unwrap().(myc.Product))
	default:
		panic(req)
	}
	if err != nil {
		return err
	}
	respTy := DEV_NET_NodeResp
	respSum, err := respTy.New(req.Tag(), resp)
	if err != nil {
		return err
	}
	if err := resp.PullInto(ctx, s, s); err != nil {
		return err
	}
	respData := myc.MarshalAppend(nil, respSum)
	if err := bytesToWords(respData, buf[:]); err != nil {
		return err
	}
	return nil
}

func (svc *nodeDev) PortType() *myc.PortType {
	return DEV_NET_Node
}

func (svc *nodeDev) Port() mvm1.PortBackend {
	return mvm1.PortBackend{
		Input:    svc.portInput,
		Interact: svc.portInteract,
	}
}

func deriveEd25519(secret *[32]byte, i uint64) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	h := blake3.New(-1, secret[:])
	h.Write(binary.BigEndian.AppendUint64([]byte("ed25519"), i))
	return ed25519.GenerateKey(h.XOF())
}

func addrFrom(x myc.Value) (mycnet.QUICAddr, error) {
	if !myc.TypeContains(DEV_NET_Addr, x) {
		return mycnet.QUICAddr{}, fmt.Errorf("not a valid address %v", x)
	}
	pr := x.(myc.Product)
	peer, err := mycnet.PeerFromMycelium(pr[0])
	if err != nil {
		return mycnet.QUICAddr{}, err
	}
	loc, err := mycipnet.UDPAddrFromMycelium(pr[1])
	if err != nil {
		return mycnet.QUICAddr{}, err
	}
	return mycnet.QUICAddr{
		Peer:     peer,
		Location: loc,
	}, nil
}

func addrTo(x mycnet.QUICAddr) myc.Value {
	return myc.Product{
		x.Peer.ToMycelium(),
		mycipnet.UDPAddrToMycelium(x.Location),
	}
}

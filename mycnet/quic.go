package mycnet

import (
	"context"
	"crypto"
	goed25519 "crypto/ed25519"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/ed25519"
	"github.com/quic-go/quic-go"
	"go.brendoncarroll.net/exp/singleflight"
	"go.brendoncarroll.net/p2p/s/swarmutil"
	"go.brendoncarroll.net/stdctx/logctx"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/mycnet/mycpki"
)

var _ Transport[netip.AddrPort] = &QUICTransport{}

type QUICAddr = Addr[netip.AddrPort]

type connKey struct {
	PeerID mycelium.CID
	Loc    netip.AddrPort
}

func mkConnKey(x QUICAddr) connKey {
	return connKey{
		PeerID: x.Peer.ID(),
		Loc:    x.Location,
	}
}

// QUICTransport implements the Network interface using the QUIC protocol.
type QUICTransport struct {
	privateKey sign.PrivateKey
	publicKey  sign.PublicKey
	quic       quic.Transport

	mu    sync.RWMutex
	conns map[connKey]quic.Connection

	dialSF   singleflight.Group[connKey, quic.Connection]
	toHandle chan quic.Connection
}

func NewQUIC(privateKey sign.PrivateKey, pconn net.PacketConn) *QUICTransport {
	return &QUICTransport{
		privateKey: privateKey,
		publicKey:  privateKey.Public().(sign.PublicKey),
		quic: quic.Transport{
			Conn: pconn,
		},

		conns:    make(map[connKey]quic.Connection),
		toHandle: make(chan quic.Connection),
	}
}

func (qt *QUICTransport) PrivateKey() sign.PrivateKey {
	return qt.privateKey
}

func (qt *QUICTransport) Tell(ctx context.Context, raddr QUICAddr, msg *Message) error {
	conn, err := qt.getConn(ctx, raddr)
	if err != nil {
		return err
	}
	s, err := conn.OpenUniStream()
	if err != nil {
		return err
	}
	defer s.Close()
	if _, err := msg.WriteTo(s); err != nil {
		return err
	}
	return s.Close()
}

func (qt *QUICTransport) Ask(ctx context.Context, raddr QUICAddr, req, resp *Message) error {
	conn, err := qt.getConn(ctx, raddr)
	if err != nil {
		return err
	}
	s, err := conn.OpenStream()
	if err != nil {
		return err
	}
	defer s.Close()
	if _, err := req.WriteTo(s); err != nil {
		return err
	}
	if _, err := resp.ReadFrom(s); err != nil {
		return err
	}
	return nil
}

// Send sends a datagram
func (qt *QUICTransport) Send(ctx context.Context, raddr QUICAddr, data []byte) error {
	conn, err := qt.getConn(ctx, raddr)
	if err != nil {
		return err
	}
	return conn.SendDatagram(data)
}

func (qt *QUICTransport) LocalPeer() Peer {
	return NewPeer(mycpki.VerifierFromPublicKey(qt.publicKey))
}

func (qt *QUICTransport) LocalAddr() QUICAddr {
	udpAddr := qt.quic.Conn.LocalAddr().(*net.UDPAddr)
	return QUICAddr{
		Peer:     qt.LocalPeer(),
		Location: udpAddr.AddrPort(),
	}
}

// getConn returns a connection to the specified peer
func (qt *QUICTransport) getConn(ctx context.Context, raddr QUICAddr) (quic.Connection, error) {
	k := mkConnKey(raddr)
	qt.mu.RLock()
	conn := qt.conns[k]
	qt.mu.RUnlock()
	if conn != nil {
		return conn, nil
	}
	conn, err, _ := qt.dialSF.Do(k, func() (quic.Connection, error) {
		// check if there is a conn again.
		qt.mu.RLock()
		conn := qt.conns[k]
		qt.mu.RUnlock()
		if conn != nil {
			return conn, nil
		}

		conn, err := qt.dialConn(ctx, raddr)
		if err != nil {
			return nil, err
		}

		// if there is now a conn in the map (it would have been incoming) use that instead.
		// we bias existing connections here.
		qt.mu.Lock()
		if oldConn, exists := qt.conns[k]; exists {
			if err := conn.CloseWithError(1, "found existing connection"); err != nil {
				logctx.Error(ctx, "closing connection", zap.Error(err))
			}
			conn = oldConn
		} else {
			qt.conns[k] = conn
		}
		qt.mu.Unlock()

		return conn, nil
	})
	return conn, err
}

// dialConn dials a new quic connection and returns it.
// It does not modify peers or take any locks.
func (qt *QUICTransport) dialConn(ctx context.Context, raddr QUICAddr) (quic.Connection, error) {
	conn, err := qt.quic.Dial(ctx, net.UDPAddrFromAddrPort(raddr.Location), qt.makeDialTlsConfig(raddr.Peer.ID()), qt.makeQuicConfig())
	if err != nil {
		return nil, err
	}
	// send it over to the run loop
	select {
	case <-ctx.Done():
		conn.CloseWithError(0, "could not service connection")
		return nil, fmt.Errorf("timed out while sending the connection to the run loop: %w", ctx.Err())
	case qt.toHandle <- conn:
		return conn, err
	}
}

type QUICHandler = MsgHandler[netip.AddrPort]

// Serve manages accepting new connections, and handling dialed connections in the background.
// If run exits, callers can be sure there is no background activity going on.
func (qt *QUICTransport) Serve(ctx context.Context, h QUICHandler) error {
	tlsConfig := qt.makeListenTlsConfig()
	quicConfig := qt.makeQuicConfig()
	l, err := qt.quic.Listen(tlsConfig, quicConfig)
	if err != nil {
		return err
	}
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for {
			conn, err := l.Accept(ctx)
			if err != nil {
				return err
			}
			go func() {
				if err := qt.handleConn(ctx, conn, h); err != nil {
					logctx.Error(ctx, "handling conn", zap.Error(err))
				}
			}()
		}
	})
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case conn := <-qt.toHandle:
				// this allows all the background conns to have the same context
				go func() {
					if err := qt.handleConn(ctx, conn, h); err != nil {
						logctx.Error(ctx, "handling conn", zap.Error(err))
					}
				}()
			}
		}
	})
	return eg.Wait()
}

func (qt *QUICTransport) handleConn(ctx context.Context, conn quic.Connection, h QUICHandler) error {
	defer conn.CloseWithError(0, "deferred close")
	peer, err := peerFromTLSState(conn.ConnectionState().TLS)
	if err != nil {
		return err
	}
	raddr := QUICAddr{Peer: *peer, Location: locationFromConn(conn)}
	k := mkConnKey(raddr)
	// TODO: would be better to do this before getting peerID
	defer func() {
		qt.mu.Lock()
		delete(qt.conns, k)
		qt.mu.Unlock()
	}()
	logctx.Info(ctx, "handling new conn", zap.Any("peer", peer.ID()), zap.Any("addr", conn.RemoteAddr()))
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for {
			s, err := conn.AcceptStream(ctx)
			if err != nil {
				return err
			}
			go func() {
				if err := qt.handleStream(ctx, raddr, s, h); err != nil {
					logctx.Error(ctx, "handling stream", zap.Error(err))
				}
			}()
		}
	})
	eg.Go(func() error {
		for {
			s, err := conn.AcceptUniStream(ctx)
			if err != nil {
				return err
			}
			go func() {
				if err := qt.handleUniStream(ctx, raddr, s, h); err != nil {
					logctx.Error(ctx, "handling uni-stream", zap.Error(err))
				}
			}()
		}
	})
	return eg.Wait()
}

func (qt *QUICTransport) handleStream(ctx context.Context, raddr QUICAddr, s quic.Stream, h QUICHandler) error {
	defer s.Close()
	var req Message
	if _, err := req.ReadFrom(s); err != nil {
		return err
	}
	var res Message
	if err := h(ctx, raddr, &req, &res); err != nil {
		return err
	}
	if res.Type() == MT_INVALID {
		return fmt.Errorf("handler did not set response message type")
	}
	_, err := res.WriteTo(s)
	return err
}

func (qt *QUICTransport) handleUniStream(ctx context.Context, raddr QUICAddr, s quic.ReceiveStream, h QUICHandler) error {
	var msg Message
	if _, err := msg.ReadFrom(s); err != nil {
		return err
	}
	if err := h(ctx, raddr, &msg, nil); err != nil {
		return err
	}
	return nil
}

// makeDialTlsConfig is called to create a tls.Config for outbound connections
func (qt *QUICTransport) makeDialTlsConfig(dst PeerID) *tls.Config {
	cfg := qt.makeTlsConfig()
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		peer, err := peerFromTLSState(cs)
		if err != nil {
			return err
		}
		actualID := peer.ID()
		if subtle.ConstantTimeCompare(actualID[:], dst[:]) != 1 {
			return errors.New("wrong peer")
		}
		return nil
	}
	return cfg
}

// makeListenTlsConfig is called by the server side to create new connections.
func (qt *QUICTransport) makeListenTlsConfig() *tls.Config {
	return qt.makeTlsConfig()
}

func (qt *QUICTransport) makeTlsConfig() *tls.Config {
	var privKey crypto.Signer
	switch x := qt.privateKey.(type) {
	case ed25519.PrivateKey:
		privKey = goed25519.PrivateKey(x)
	default:
		panic(qt.privateKey)
	}
	cert := swarmutil.GenerateSelfSigned(privKey)
	localID := qt.LocalPeer().ID()
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		NextProtos:         []string{"myceliumweb.org/mycelium/mycmem"},
		ClientAuth:         tls.RequireAnyClientCert,
		ServerName:         localID.String(),
		InsecureSkipVerify: true,
	}
}

func (qt *QUICTransport) makeQuicConfig() *quic.Config {
	return &quic.Config{}
}

func peerFromTLSState(tlsState tls.ConnectionState) (*Peer, error) {
	if len(tlsState.PeerCertificates) < 1 {
		return nil, errors.New("no certificates")
	}
	cert := tlsState.PeerCertificates[0]
	switch pubKey := cert.PublicKey.(type) {
	case goed25519.PublicKey:
		vf := mycpki.VerifierFromPublicKey(ed25519.PublicKey(pubKey))
		peer := NewPeer(vf)
		return &peer, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %T", cert.PublicKey)
	}
}

func locationFromConn(x quic.Connection) netip.AddrPort {
	udpAddr := x.RemoteAddr().(*net.UDPAddr)
	return udpAddr.AddrPort()
}

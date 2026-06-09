package p2p

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"
)

type Node struct {
	host   host.Host
	peerID peer.ID
	ctx    context.Context
	cancel context.CancelFunc
}

type Config struct {
	Port       int
	PrivateKey crypto.PrivKey
	ListenAddrs []string
	ConnMgrLow  int
	ConnMgrHigh int
}

type Option func(*Config)

func WithPort(port int) Option {
	return func(c *Config) {
		c.Port = port
	}
}

func WithPrivateKey(key crypto.PrivKey) Option {
	return func(c *Config) {
		c.PrivateKey = key
	}
}

func WithListenAddrs(addrs ...string) Option {
	return func(c *Config) {
		c.ListenAddrs = addrs
	}
}

func WithConnectionManager(low, high int) Option {
	return func(c *Config) {
		c.ConnMgrLow = low
		c.ConnMgrHigh = high
	}
}

func NewNode(opts ...Option) (*Node, error) {
	cfg := &Config{}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Port == 0 {
		cfg.Port = 0
	}

	ctx, cancel := context.WithCancel(context.Background())

	var libp2pOpts []libp2p.Option

	if cfg.PrivateKey != nil {
		libp2pOpts = append(libp2pOpts, libp2p.Identity(cfg.PrivateKey))
	}

	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Port)
	if len(cfg.ListenAddrs) > 0 {
		listenAddr = cfg.ListenAddrs[0]
	}
	libp2pOpts = append(libp2pOpts, libp2p.ListenAddrStrings(listenAddr))

	if cfg.ConnMgrLow > 0 && cfg.ConnMgrHigh > 0 {
		cm, err := connmgr.NewConnManager(cfg.ConnMgrLow, cfg.ConnMgrHigh)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("connmgr: %w", err)
		}
		libp2pOpts = append(libp2pOpts, libp2p.ConnectionManager(cm))
	}

	h, err := libp2p.New(libp2pOpts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("libp2p.New: %w", err)
	}

	return &Node{
		host:   h,
		peerID: h.ID(),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (n *Node) PeerID() peer.ID {
	return n.peerID
}

func (n *Node) Host() host.Host {
	return n.host
}

func (n *Node) Multiaddrs() []multiaddr.Multiaddr {
	addrs := n.host.Addrs()
	pid := n.peerID
	p2pAddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		p2pAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", addr.String(), pid.String()))
		if err != nil {
			continue
		}
		p2pAddrs = append(p2pAddrs, p2pAddr)
	}
	return p2pAddrs
}

func (n *Node) Close() error {
	n.cancel()
	if err := n.host.Close(); err != nil {
		return err
	}
	return nil
}

func (n *Node) Connect(addr multiaddr.Multiaddr) error {
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("parse addr: %w", err)
	}
	n.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	return n.host.Connect(n.ctx, *info)
}

func ListenPort(h host.Host) (int, error) {
	for _, addr := range h.Addrs() {
		_, err := multiaddr.NewMultiaddr(addr.String())
		if err != nil {
			continue
		}
		port, err := addr.ValueForProtocol(multiaddr.P_TCP)
		if err == nil {
			return net.LookupPort("tcp", port)
		}
	}
	return 0, fmt.Errorf("no TCP address found")
}

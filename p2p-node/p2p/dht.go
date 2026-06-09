package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	kaddht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
)

type DHTMode int

const (
	DHTModeClient DHTMode = iota
	DHTModeServer
)

type DHTConfig struct {
	Mode           DHTMode
	BootstrapPeers []multiaddr.Multiaddr
}

type DHTOption func(*DHTConfig)

func WithDHTMode(mode DHTMode) DHTOption {
	return func(c *DHTConfig) {
		c.Mode = mode
	}
}

func WithBootstrapPeers(addrs []multiaddr.Multiaddr) DHTOption {
	return func(c *DHTConfig) {
		c.BootstrapPeers = addrs
	}
}

type DHT struct {
	host   host.Host
	dht    *kaddht.IpfsDHT
	config DHTConfig
	once   sync.Once
	closer func()
}

func NewDHT(n *Node, opts ...DHTOption) (*DHT, error) {
	cfg := &DHTConfig{
		Mode: DHTModeServer,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx := context.Background()

	var mode kaddht.ModeOpt
	switch cfg.Mode {
	case DHTModeClient:
		mode = kaddht.ModeClient
	default:
		mode = kaddht.ModeServer
	}

	dht, err := kaddht.New(ctx, n.Host(), kaddht.Mode(mode))
	if err != nil {
		return nil, fmt.Errorf("kaddht.New: %w", err)
	}

	return &DHT{
		host:   n.Host(),
		dht:    dht,
		config: *cfg,
		closer: func() { dht.Close() },
	}, nil
}

func (d *DHT) Bootstrap(ctx context.Context) error {
	if len(d.config.BootstrapPeers) == 0 {
		return fmt.Errorf("no bootstrap peers configured")
	}

	peers := make([]peer.AddrInfo, 0, len(d.config.BootstrapPeers))
	for _, addr := range d.config.BootstrapPeers {
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			continue
		}
		peers = append(peers, *info)
	}

	for _, p := range peers {
		d.host.Peerstore().AddAddrs(p.ID, p.Addrs, peerstoreAddrTTL)
	}

	return d.dht.Bootstrap(ctx)
}

func (d *DHT) FindPeer(ctx context.Context, pid peer.ID) (peer.AddrInfo, error) {
	return d.dht.FindPeer(ctx, pid)
}

func (d *DHT) Provide(ctx context.Context, key string) error {
	c, err := cidFromString(key)
	if err != nil {
		return fmt.Errorf("cidFromString: %w", err)
	}
	return d.dht.Provide(ctx, c, true)
}

func (d *DHT) FindProviders(ctx context.Context, key string) ([]peer.AddrInfo, error) {
	c, err := cidFromString(key)
	if err != nil {
		return nil, fmt.Errorf("cidFromString: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pCh := d.dht.FindProvidersAsync(ctx, c, 0)

	var providers []peer.AddrInfo
	for p := range pCh {
		providers = append(providers, p)
	}

	return providers, nil
}

func (d *DHT) WaitForPeer(ctx context.Context, pid peer.ID) error {
	for d.dht.RoutingTable().Find(pid) == "" {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for peer %s in routing table: %w", pid, ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	return nil
}

func (d *DHT) Close() error {
	d.once.Do(func() {
		if d.closer != nil {
			d.closer()
		}
	})
	return nil
}

func cidFromString(s string) (cid.Cid, error) {
	pref := cid.Prefix{
		Version:  1,
		Codec:    cid.Raw,
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}
	return pref.Sum([]byte(s))
}

const peerstoreAddrTTL = time.Hour

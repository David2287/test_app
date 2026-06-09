package p2p_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/whyskydie/p2p-node/p2p"
)

func newTestNode(t *testing.T) *p2p.Node {
	t.Helper()
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() { n.Close() })
	return n
}

func TestDHT_New(t *testing.T) {
	n := newTestNode(t)
	dht, err := p2p.NewDHT(n, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	require.NotNil(t, dht)
	t.Cleanup(func() { dht.Close() })
}

func TestDHT_Bootstrap_NoPeers(t *testing.T) {
	n := newTestNode(t)
	dht, err := p2p.NewDHT(n, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	defer dht.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = dht.Bootstrap(ctx)
	require.Error(t, err)
}

func TestDHT_FindPeer_Unknown(t *testing.T) {
	n := newTestNode(t)
	dht, err := p2p.NewDHT(n, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	defer dht.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	unknownKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	require.NoError(t, err)
	unknownID, err := peer.IDFromPrivateKey(unknownKey)
	require.NoError(t, err)

	_, err = dht.FindPeer(ctx, unknownID)
	require.Error(t, err)
}

func TestDHT_TwoNodes_FindPeer(t *testing.T) {
	n1 := newTestNode(t)
	n2 := newTestNode(t)

	dht1, err := p2p.NewDHT(n1, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	defer dht1.Close()

	dht2, err := p2p.NewDHT(n2, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	defer dht2.Close()

	err = n1.Connect(n2.Multiaddrs()[0])
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = dht1.WaitForPeer(ctx, n2.PeerID())
	require.NoError(t, err)

	info, err := dht1.FindPeer(ctx, n2.PeerID())
	require.NoError(t, err)
	require.Equal(t, n2.PeerID(), info.ID)
}

func TestDHT_Provide_Find(t *testing.T) {
	n1 := newTestNode(t)
	n2 := newTestNode(t)

	dht1, err := p2p.NewDHT(n1, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	defer dht1.Close()

	dht2, err := p2p.NewDHT(n2, p2p.WithDHTMode(p2p.DHTModeServer))
	require.NoError(t, err)
	defer dht2.Close()

	err = n1.Connect(n2.Multiaddrs()[0])
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = dht1.WaitForPeer(ctx, n2.PeerID())
	require.NoError(t, err)
	err = dht2.WaitForPeer(ctx, n1.PeerID())
	require.NoError(t, err)

	err = dht1.Provide(ctx, "test-namespace")
	require.NoError(t, err)

	providers, err := dht2.FindProviders(ctx, "test-namespace")
	require.NoError(t, err)
	require.NotEmpty(t, providers)

	found := false
	for _, p := range providers {
		if p.ID == n1.PeerID() {
			found = true
			break
		}
	}
	require.True(t, found, "peer should be found in providers")
}

func TestDHT_WithBootstrapPeers(t *testing.T) {
	key, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	require.NoError(t, err)
	pid, err := peer.IDFromPrivateKey(key)
	require.NoError(t, err)

	maddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/1.2.3.4/tcp/4001/p2p/%s", pid.String()))
	require.NoError(t, err)

	n := newTestNode(t)
	dht, err := p2p.NewDHT(
		n,
		p2p.WithDHTMode(p2p.DHTModeServer),
		p2p.WithBootstrapPeers([]multiaddr.Multiaddr{maddr}),
	)
	require.NoError(t, err)
	require.NotNil(t, dht)
	dht.Close()
}

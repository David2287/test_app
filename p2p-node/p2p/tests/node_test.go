package p2p_test

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/whyskydie/p2p-node/p2p"
)

func TestNewNode_Default(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	require.NotNil(t, n)
	require.NotEmpty(t, n.PeerID().String())
	t.Cleanup(func() { n.Close() })
}

func TestNode_PeerID_Deterministic(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	require.NoError(t, err)

	expectedID, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)

	n, err := p2p.NewNode(p2p.WithPrivateKey(priv), p2p.WithPort(0))
	require.NoError(t, err)
	require.Equal(t, expectedID, n.PeerID())
	t.Cleanup(func() { n.Close() })
}

func TestNode_Multiaddrs(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() { n.Close() })

	addrs := n.Multiaddrs()
	require.NotEmpty(t, addrs)
	for _, addr := range addrs {
		require.Contains(t, addr.String(), n.PeerID().String())
	}
}

func TestNode_Close_Twice(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)

	require.NoError(t, n.Close())
	require.NoError(t, n.Close())
}

func TestNode_WithListenAddrs(t *testing.T) {
	n, err := p2p.NewNode(
		p2p.WithPort(0),
		p2p.WithListenAddrs("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { n.Close() })

	addrs := n.Multiaddrs()
	require.NotEmpty(t, addrs)
}

func TestNode_ConnManager(t *testing.T) {
	n, err := p2p.NewNode(
		p2p.WithPort(0),
		p2p.WithConnectionManager(100, 50),
	)
	require.NoError(t, err)
	require.NotNil(t, n)
	t.Cleanup(func() { n.Close() })
}

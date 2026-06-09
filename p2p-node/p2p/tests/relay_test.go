package p2p_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"

	"github.com/whyskydie/p2p-node/p2p"
)

const relayTestProto = protocol.ID("/p2p/test/relay/1.0.0")

func TestRelay_NewRelay(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n.Close()

	r, err := p2p.NewRelay(n)
	require.NoError(t, err)
	defer r.Close()
}

func TestRelay_ThreeNodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relayNode, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer relayNode.Close()

	rl, err := p2p.NewRelay(relayNode)
	require.NoError(t, err)
	defer rl.Close()

	clientA, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer clientA.Close()

	clientB, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer clientB.Close()

	relayAddr := relayNode.Multiaddrs()[0]

	err = clientA.Connect(relayAddr)
	require.NoError(t, err)

	err = clientB.Connect(relayAddr)
	require.NoError(t, err)

	circuitInfo, err := p2p.ReserveRelay(ctx, clientA, relayAddr)
	require.NoError(t, err)
	require.NotEmpty(t, circuitInfo.Addrs)

	clientB.Host().Peerstore().AddAddrs(circuitInfo.ID, circuitInfo.Addrs, peerstore.PermanentAddrTTL)
	err = clientB.Host().Connect(ctx, *circuitInfo)
	require.NoError(t, err)

	msg := []byte("hello from relay test!")
	clientA.Host().SetStreamHandler(relayTestProto, func(s network.Stream) {
		defer s.Close()
		_, _ = s.Write(msg)
	})

	s, err := clientB.Host().NewStream(
		network.WithAllowLimitedConn(ctx, "relay-test"),
		clientA.PeerID(),
		relayTestProto,
	)
	require.NoError(t, err)
	defer s.Close()

	buf, err := io.ReadAll(s)
	require.NoError(t, err)
	require.Equal(t, msg, buf)
}

func TestRelay_CircuitAddr(t *testing.T) {
	relayNode, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer relayNode.Close()

	clientNode, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer clientNode.Close()

	relayP2pAddr := relayNode.Multiaddrs()[0]
	circuitAddr := p2p.CircuitAddr(relayP2pAddr, clientNode.PeerID())
	require.NotNil(t, circuitAddr)

	addrStr := circuitAddr.String()
	require.Contains(t, addrStr, "/p2p-circuit/p2p/")
	require.Contains(t, addrStr, clientNode.PeerID().String())
}

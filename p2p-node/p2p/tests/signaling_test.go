package p2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/whyskydie/p2p-node/p2p"
)

func TestSignaling_DialAndExchange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodeA, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer nodeA.Close()

	nodeB, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer nodeB.Close()

	err = nodeA.Connect(nodeB.Multiaddrs()[0])
	require.NoError(t, err)

	sigA := p2p.NewSignaling(nodeA)
	sigB := p2p.NewSignaling(nodeB)

	var sessionB *p2p.SignalSession
	sigB.SetHandler(func(s *p2p.SignalSession) {
		sessionB = s
	})

	err = sigA.Start()
	require.NoError(t, err)
	err = sigB.Start()
	require.NoError(t, err)

	sessionA, err := sigA.Dial(ctx, nodeB.PeerID())
	require.NoError(t, err)
	require.NotNil(t, sessionA)
	require.Equal(t, nodeB.PeerID(), sessionA.Remote())

	require.Eventually(t, func() bool { return sessionB != nil }, 3*time.Second, 100*time.Millisecond)

	err = sessionA.Send(ctx, p2p.SignalMessage{Type: p2p.SignalOffer, Data: "test-sdp-offer"})
	require.NoError(t, err)

	offerMsg, err := sessionB.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, p2p.SignalOffer, offerMsg.Type)
	require.Equal(t, "test-sdp-offer", offerMsg.Data)

	err = sessionB.Send(ctx, p2p.SignalMessage{Type: p2p.SignalAnswer, Data: "test-sdp-answer"})
	require.NoError(t, err)

	answerMsg, err := sessionA.Receive(ctx)
	require.NoError(t, err)
	require.Equal(t, p2p.SignalAnswer, answerMsg.Type)
	require.Equal(t, "test-sdp-answer", answerMsg.Data)

	sessionA.Close()
	sessionB.Close()
}

func TestSignaling_Close(t *testing.T) {
	node, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer node.Close()

	sig := p2p.NewSignaling(node)
	err = sig.Start()
	require.NoError(t, err)

	sig.Close()

	err = sig.Start()
	require.NoError(t, err)
}

func TestSignaling_DialBeforeHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodeA, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer nodeA.Close()

	nodeB, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer nodeB.Close()

	err = nodeA.Connect(nodeB.Multiaddrs()[0])
	require.NoError(t, err)

	sigA := p2p.NewSignaling(nodeA)
	sigB := p2p.NewSignaling(nodeB)

	sigB.SetHandler(func(s *p2p.SignalSession) {})

	err = sigB.Start()
	require.NoError(t, err)
	err = sigA.Start()
	require.NoError(t, err)

	sessionA, err := sigA.Dial(ctx, nodeB.PeerID())
	require.NoError(t, err)
	require.NotNil(t, sessionA)
	sessionA.Close()
}

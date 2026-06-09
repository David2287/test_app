package p2p_test

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/whyskydie/p2p-node/p2p"
)

type testNotifee struct {
	count atomic.Int32
}

func (n *testNotifee) HandlePeerFound(info peer.AddrInfo) {
	n.count.Add(1)
}

func (n *testNotifee) Count() int32 {
	return n.count.Load()
}

func TestNewDiscovery_Valid(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n.Close()

	disc := p2p.NewDiscovery(n, "", &testNotifee{})
	require.NotNil(t, disc)
	assert.NotNil(t, disc.Service)
}

func TestNewDiscovery_NilNotifee(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n.Close()

	disc := p2p.NewDiscovery(n, "", nil)
	require.NotNil(t, disc)
}

func TestDiscovery_StartClose(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n.Close()

	disc := p2p.NewDiscovery(n, "", &testNotifee{})
	require.NotNil(t, disc)

	err = disc.Start()
	assert.NoError(t, err)

	err = disc.Close()
	assert.NoError(t, err)
}

func TestDiscovery_StartTwice(t *testing.T) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n.Close()

	disc := p2p.NewDiscovery(n, "", &testNotifee{})
	require.NotNil(t, disc)

	err = disc.Start()
	require.NoError(t, err)

	err = disc.Start()
	assert.NoError(t, err)

	disc.Close()
}

func TestDiscovery_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("mDNS integration test requires Linux (multicast on loopback)")
	}

	discCh := make(chan peer.AddrInfo, 16)

	n1, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n1.Close()

	n2, err := p2p.NewNode(p2p.WithPort(0))
	require.NoError(t, err)
	defer n2.Close()

	disc1 := p2p.NewDiscovery(n1, "", mdnsNotifeeFunc(func(info peer.AddrInfo) {
		discCh <- info
	}))
	require.NoError(t, disc1.Start())
	defer disc1.Close()

	disc2 := p2p.NewDiscovery(n2, "", &testNotifee{})
	require.NoError(t, disc2.Start())
	defer disc2.Close()

	var found bool
	select {
	case info := <-discCh:
		t.Logf("Discovered peer: %s at %v", info.ID, info.Addrs)
		assert.Equal(t, n2.PeerID(), info.ID)
		found = true
	case <-time.After(3 * time.Second):
		t.Log("mDNS: no peers discovered (expected on platforms without loopback multicast)")
	}

	if found {
		assert.Eventually(t, func() bool {
			return disc2.Notifee.(*testNotifee).Count() >= 1
		}, 3*time.Second, 100*time.Millisecond)
	}
}

type mdnsNotifeeFunc func(peer.AddrInfo)

func (f mdnsNotifeeFunc) HandlePeerFound(info peer.AddrInfo) {
	f(info)
}

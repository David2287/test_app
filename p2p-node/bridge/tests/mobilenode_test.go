package bridge_test

import (
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/whyskydie/p2p-node/bridge"
)

type testHandler struct {
	mu        sync.Mutex
	discs     []string
	sessions  []string
	messages  []msgEvent
	closed    []string
	errors    []string
}

type msgEvent struct {
	sessionID string
	msgType   string
	data      string
}

func (h *testHandler) OnPeerDiscovered(peerID string, addrs string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.discs = append(h.discs, peerID)
}

func (h *testHandler) OnSignalSession(sessionID string, peerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions = append(h.sessions, sessionID)
}

func (h *testHandler) OnSignalMessage(sessionID string, msgType string, data string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msgEvent{sessionID, msgType, data})
}

func (h *testHandler) OnSignalSessionClosed(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = append(h.closed, sessionID)
}

func (h *testHandler) OnError(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errors = append(h.errors, message)
}

func TestNewMobileNode(t *testing.T) {
	handler := &testHandler{}
	mn, err := bridge.NewMobileNode(handler)
	require.NoError(t, err)
	require.NotNil(t, mn)
	defer mn.Close()

	assert.NotEmpty(t, mn.PeerID())
}

func TestMobileNode_Addresses(t *testing.T) {
	mn, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mn.Close()

	addrs := mn.Addresses()
	assert.NotEmpty(t, addrs)
	for _, a := range addrs {
		assert.Contains(t, a.Value, "/p2p/")
	}
}

func TestMobileNode_AddressesString(t *testing.T) {
	mn, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mn.Close()

	addrs := mn.AddressesString()
	assert.NotEmpty(t, addrs)
	for _, a := range addrs {
		assert.Contains(t, a, "/p2p/")
	}
}

func TestMobileNode_ConnectDisconnect(t *testing.T) {
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	addr := mnB.AddressesString()[0]
	err = mnA.Connect(addr)
	require.NoError(t, err)

	err = mnA.Disconnect(mnB.PeerID())
	require.NoError(t, err)
}

func TestMobileNode_DHT_FindPeer(t *testing.T) {
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	err = mnA.StartDHT(nil)
	require.NoError(t, err)

	err = mnB.StartDHT(nil)
	require.NoError(t, err)

	addr := mnB.AddressesString()[0]
	err = mnA.Connect(addr)
	require.NoError(t, err)

	_, err = mnA.FindPeer(mnB.PeerID())
	require.NoError(t, err)
}

func TestMobileNode_Provide_Find(t *testing.T) {
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	err = mnA.StartDHT(nil)
	require.NoError(t, err)

	err = mnB.StartDHT(nil)
	require.NoError(t, err)

	addr := mnB.AddressesString()[0]
	err = mnA.Connect(addr)
	require.NoError(t, err)

	err = mnA.WaitForPeer(mnB.PeerID())
	require.NoError(t, err)

	err = mnA.Provide("test-key")
	require.NoError(t, err)

	providers, err := mnB.FindProviders("test-key")
	require.NoError(t, err)
	assert.NotEmpty(t, providers)
	assert.Equal(t, mnA.PeerID(), providers[0])
}

func TestMobileNode_Signaling_Exchange(t *testing.T) {
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	addr := mnB.AddressesString()[0]
	err = mnA.Connect(addr)
	require.NoError(t, err)

	err = mnA.StartSignaling()
	require.NoError(t, err)

	err = mnB.StartSignaling()
	require.NoError(t, err)

	sessionID, err := mnA.DialSignal(mnB.PeerID())
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	err = mnA.SendSignal(sessionID, "offer", "test-sdp-offer")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		handlerB.mu.Lock()
		defer handlerB.mu.Unlock()
		return len(handlerB.messages) > 0
	}, 5*time.Second, 100*time.Millisecond)

	handlerB.mu.Lock()
	msgs := handlerB.messages
	handlerB.mu.Unlock()

	require.Len(t, msgs, 1)
	assert.Equal(t, "offer", msgs[0].msgType)
	assert.Equal(t, "test-sdp-offer", msgs[0].data)

	err = mnA.CloseSignalSession(sessionID)
	require.NoError(t, err)
}

func TestMobileNode_Relay(t *testing.T) {
	handlerR := &testHandler{}
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnR, err := bridge.NewMobileNode(handlerR)
	require.NoError(t, err)
	defer mnR.Close()

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	err = mnR.StartRelay()
	require.NoError(t, err)

	err = mnA.Connect(mnR.AddressesString()[0])
	require.NoError(t, err)

	err = mnB.Connect(mnR.AddressesString()[0])
	require.NoError(t, err)

	circuitAddr, err := mnA.ReserveRelay(mnR.AddressesString()[0])
	require.NoError(t, err)
	assert.Contains(t, circuitAddr, "/p2p-circuit/")
}

func TestMobileNode_Close(t *testing.T) {
	mn, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)

	err = mn.Close()
	require.NoError(t, err)
}

func TestMobileNode_Close_Idempotent(t *testing.T) {
	mn, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)

	err = mn.Close()
	require.NoError(t, err)

	err = mn.Close()
	require.NoError(t, err)
}

func TestMobileNode_GenerateE2EEKey(t *testing.T) {
	mn, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mn.Close()

	pubKey, err := mn.GenerateE2EEKey()
	require.NoError(t, err)
	require.Len(t, pubKey, 64)
}

func TestMobileNode_SetRemoteKey(t *testing.T) {
	mnA, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mnB.Close()

	pubA, err := mnA.GenerateE2EEKey()
	require.NoError(t, err)

	pubB, err := mnB.GenerateE2EEKey()
	require.NoError(t, err)

	err = mnA.SetRemoteKey(mnB.PeerID(), pubB)
	require.NoError(t, err)

	err = mnB.SetRemoteKey(mnA.PeerID(), pubA)
	require.NoError(t, err)

	require.True(t, mnA.HasSharedSecret(mnB.PeerID()))
	require.True(t, mnB.HasSharedSecret(mnA.PeerID()))
}

func TestMobileNode_EncryptDecryptMessage(t *testing.T) {
	mnA, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mnB.Close()

	pubA, err := mnA.GenerateE2EEKey()
	require.NoError(t, err)

	pubB, err := mnB.GenerateE2EEKey()
	require.NoError(t, err)

	err = mnA.SetRemoteKey(mnB.PeerID(), pubB)
	require.NoError(t, err)

	err = mnB.SetRemoteKey(mnA.PeerID(), pubA)
	require.NoError(t, err)

	plaintext := []byte("hello E2EE world")
	enc, err := mnA.EncryptMessage(mnB.PeerID(), plaintext)
	require.NoError(t, err)
	require.NotEqual(t, hex.EncodeToString(plaintext), enc.Ciphertext)

	dec, err := mnB.DecryptMessage(mnA.PeerID(), enc.Ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, dec.Plaintext)
}

func TestMobileNode_Encrypt_WrongPeer(t *testing.T) {
	mnA, err := bridge.NewMobileNode(&testHandler{})
	require.NoError(t, err)
	defer mnA.Close()

	_, err = mnA.GenerateE2EEKey()
	require.NoError(t, err)

	_, err = mnA.EncryptMessage("unknown-peer", []byte("hello"))
	require.Error(t, err)
}

func TestMobileNode_E2EE_SignalingExchange(t *testing.T) {
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	addr := mnB.AddressesString()[0]
	err = mnA.Connect(addr)
	require.NoError(t, err)

	err = mnA.StartSignaling()
	require.NoError(t, err)

	err = mnB.StartSignaling()
	require.NoError(t, err)

	sessionID, err := mnA.DialSignal(mnB.PeerID())
	require.NoError(t, err)

	pubA, err := mnA.GenerateE2EEKey()
	require.NoError(t, err)

	err = mnA.SendSignal(sessionID, "e2ee_key", pubA)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		handlerB.mu.Lock()
		defer handlerB.mu.Unlock()
		for _, msg := range handlerB.messages {
			if msg.msgType == "e2ee_key" {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond)

	handlerB.mu.Lock()
	var receivedPub string
	for _, msg := range handlerB.messages {
		if msg.msgType == "e2ee_key" {
			receivedPub = msg.data
		}
	}
	handlerB.mu.Unlock()
	require.Equal(t, pubA, receivedPub)
}

func TestMobileNode_E2EE_EncryptViaSignaling(t *testing.T) {
	handlerA := &testHandler{}
	handlerB := &testHandler{}

	mnA, err := bridge.NewMobileNode(handlerA)
	require.NoError(t, err)
	defer mnA.Close()

	mnB, err := bridge.NewMobileNode(handlerB)
	require.NoError(t, err)
	defer mnB.Close()

	addr := mnB.AddressesString()[0]
	err = mnA.Connect(addr)
	require.NoError(t, err)

	err = mnA.StartSignaling()
	require.NoError(t, err)

	err = mnB.StartSignaling()
	require.NoError(t, err)

	sessionID, err := mnA.DialSignal(mnB.PeerID())
	require.NoError(t, err)

	pubA, err := mnA.GenerateE2EEKey()
	require.NoError(t, err)

	pubB, err := mnB.GenerateE2EEKey()
	require.NoError(t, err)

	err = mnA.SetRemoteKey(mnB.PeerID(), pubB)
	require.NoError(t, err)

	err = mnB.SetRemoteKey(mnA.PeerID(), pubA)
	require.NoError(t, err)

	plaintext := []byte("secret via signaling")
	enc, err := mnA.EncryptMessage(mnB.PeerID(), plaintext)
	require.NoError(t, err)

	err = mnA.SendSignal(sessionID, "e2ee_msg", enc.Ciphertext)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		handlerB.mu.Lock()
		defer handlerB.mu.Unlock()
		for _, msg := range handlerB.messages {
			if msg.msgType == "e2ee_msg" {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond)

	handlerB.mu.Lock()
	var receivedCiphertext string
	for _, msg := range handlerB.messages {
		if msg.msgType == "e2ee_msg" {
			receivedCiphertext = msg.data
		}
	}
	handlerB.mu.Unlock()
	require.NotEmpty(t, receivedCiphertext)

	dec, err := mnB.DecryptMessage(mnA.PeerID(), receivedCiphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, dec.Plaintext)
}

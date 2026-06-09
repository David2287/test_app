package bridge

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/whyskydie/p2p-node/crypto"
	"github.com/whyskydie/p2p-node/p2p"
)

type EventHandler interface {
	OnPeerDiscovered(peerID string, addrs string)
	OnSignalSession(sessionID string, peerID string)
	OnSignalMessage(sessionID string, msgType string, data string)
	OnSignalSessionClosed(sessionID string)
	OnError(message string)
}

type MobileNode struct {
	mu       sync.Mutex
	node     *p2p.Node
	dht      *p2p.DHT
	disc     *p2p.Discovery
	relay    *p2p.Relay
	sig      *p2p.Signaling
	handler  EventHandler
	sessions map[string]*p2p.SignalSession
	ctx      context.Context
	cancel   context.CancelFunc
	seq      int

	e2eeKey    *crypto.KeyPair
	sharedSec  map[string][]byte
}

func NewMobileNode(handler EventHandler) (*MobileNode, error) {
	n, err := p2p.NewNode(p2p.WithPort(0))
	if err != nil {
		return nil, fmt.Errorf("new node: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	mn := &MobileNode{
		node:       n,
		handler:    handler,
		sessions:   make(map[string]*p2p.SignalSession),
		sharedSec:  make(map[string][]byte),
		ctx:        ctx,
		cancel:     cancel,
	}

	return mn, nil
}

func (m *MobileNode) PeerID() string {
	return m.node.PeerID().String()
}

type Multiaddr struct {
	Value string
}

func (m *MobileNode) Addresses() []*Multiaddr {
	addrs := m.node.Multiaddrs()
	res := make([]*Multiaddr, 0, len(addrs))
	for _, a := range addrs {
		res = append(res, &Multiaddr{Value: a.String()})
	}
	return res
}

func (m *MobileNode) AddressesString() []string {
	addrs := m.node.Multiaddrs()
	res := make([]string, len(addrs))
	for i, a := range addrs {
		res[i] = a.String()
	}
	return res
}

func (m *MobileNode) Connect(addr string) error {
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return fmt.Errorf("parse addr: %w", err)
	}
	return m.node.Connect(ma)
}

func (m *MobileNode) Disconnect(peerID string) error {
	pid, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("decode peer: %w", err)
	}
	return m.node.Host().Network().ClosePeer(pid)
}

func (m *MobileNode) StartDHT(bootstrapPeers []string) error {
	var opts []p2p.DHTOption
	if len(bootstrapPeers) > 0 {
		addrs := make([]multiaddr.Multiaddr, 0, len(bootstrapPeers))
		for _, s := range bootstrapPeers {
			ma, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				continue
			}
			addrs = append(addrs, ma)
		}
		if len(addrs) > 0 {
			opts = append(opts, p2p.WithBootstrapPeers(addrs))
		}
	}

	dht, err := p2p.NewDHT(m.node, opts...)
	if err != nil {
		return fmt.Errorf("new dht: %w", err)
	}
	m.dht = dht

	if len(bootstrapPeers) > 0 {
		ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
		defer cancel()
		if err := dht.Bootstrap(ctx); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
	}

	return nil
}

func (m *MobileNode) FindPeer(peerID string) ([]string, error) {
	if m.dht == nil {
		return nil, fmt.Errorf("DHT not started")
	}
	pid, err := peer.Decode(peerID)
	if err != nil {
		return nil, fmt.Errorf("decode peer: %w", err)
	}
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()
	info, err := m.dht.FindPeer(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("find peer: %w", err)
	}
	addrs := make([]string, len(info.Addrs))
	for i, a := range info.Addrs {
		addrs[i] = a.String()
	}
	return addrs, nil
}

func (m *MobileNode) WaitForPeer(peerID string) error {
	if m.dht == nil {
		return fmt.Errorf("DHT not started")
	}
	pid, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("decode peer: %w", err)
	}
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()
	return m.dht.WaitForPeer(ctx, pid)
}

func (m *MobileNode) Provide(key string) error {
	if m.dht == nil {
		return fmt.Errorf("DHT not started")
	}
	return m.dht.Provide(m.ctx, key)
}

func (m *MobileNode) FindProviders(key string) ([]string, error) {
	if m.dht == nil {
		return nil, fmt.Errorf("DHT not started")
	}
	providers, err := m.dht.FindProviders(m.ctx, key)
	if err != nil {
		return nil, fmt.Errorf("find providers: %w", err)
	}
	res := make([]string, 0, len(providers))
	for _, p := range providers {
		res = append(res, p.ID.String())
	}
	return res, nil
}

func (m *MobileNode) StartDiscovery(serviceName string) error {
	disc := p2p.NewDiscovery(m.node, serviceName, m)
	if err := disc.Start(); err != nil {
		return fmt.Errorf("discovery start: %w", err)
	}
	m.disc = disc
	return nil
}

func (m *MobileNode) StopDiscovery() error {
	if m.disc == nil {
		return nil
	}
	return m.disc.Close()
}

func (m *MobileNode) HandlePeerFound(info peer.AddrInfo) {
	if m.handler == nil {
		return
	}
	addrs := make([]string, len(info.Addrs))
	for i, a := range info.Addrs {
		addrs[i] = a.String()
	}
	m.handler.OnPeerDiscovered(info.ID.String(), strings.Join(addrs, ","))
}

func (m *MobileNode) StartRelay() error {
	r, err := p2p.NewRelay(m.node)
	if err != nil {
		return fmt.Errorf("new relay: %w", err)
	}
	m.relay = r
	return nil
}

func (m *MobileNode) ReserveRelay(relayAddr string) (string, error) {
	ma, err := multiaddr.NewMultiaddr(relayAddr)
	if err != nil {
		return "", fmt.Errorf("parse relay addr: %w", err)
	}
	info, err := p2p.ReserveRelay(m.ctx, m.node, ma)
	if err != nil {
		return "", fmt.Errorf("reserve relay: %w", err)
	}
	if len(info.Addrs) > 0 {
		return info.Addrs[0].String(), nil
	}
	return "", fmt.Errorf("no circuit address returned")
}

func (m *MobileNode) StartSignaling() error {
	sig := p2p.NewSignaling(m.node)
	sig.SetHandler(func(s *p2p.SignalSession) {
		m.mu.Lock()
		sessionID := fmt.Sprintf("sess_%d", m.seq)
		m.seq++
		m.sessions[sessionID] = s
		m.mu.Unlock()

		if m.handler != nil {
			m.handler.OnSignalSession(sessionID, s.Remote().String())
		}

		m.startSessionReceive(sessionID, s)
	})
	if err := sig.Start(); err != nil {
		return fmt.Errorf("signaling start: %w", err)
	}
	m.sig = sig
	return nil
}

func (m *MobileNode) DialSignal(peerID string) (string, error) {
	if m.sig == nil {
		return "", fmt.Errorf("signaling not started")
	}
	pid, err := peer.Decode(peerID)
	if err != nil {
		return "", fmt.Errorf("decode peer: %w", err)
	}
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	session, err := m.sig.Dial(ctx, pid)
	if err != nil {
		return "", fmt.Errorf("dial signal: %w", err)
	}

	m.mu.Lock()
	sessionID := fmt.Sprintf("sess_%d", m.seq)
	m.seq++
	m.sessions[sessionID] = session
	m.mu.Unlock()

	m.startSessionReceive(sessionID, session)

	return sessionID, nil
}

func (m *MobileNode) SendSignal(sessionID string, msgType string, data string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return session.Send(m.ctx, p2p.SignalMessage{
		Type: p2p.SignalMessageType(msgType),
		Data: data,
	})
}

func (m *MobileNode) CloseSignalSession(sessionID string) error {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return session.Close()
}

func (m *MobileNode) startSessionReceive(sessionID string, session *p2p.SignalSession) {
	go func() {
		for {
			msg, err := session.Receive(m.ctx)
			if err != nil {
				m.mu.Lock()
				delete(m.sessions, sessionID)
				m.mu.Unlock()
				if m.handler != nil {
					m.handler.OnSignalSessionClosed(sessionID)
				}
				return
			}
			if m.handler != nil {
				m.handler.OnSignalMessage(sessionID, string(msg.Type), msg.Data)
			}
		}
	}()
}

func (m *MobileNode) Close() error {
	m.cancel()

	m.mu.Lock()
	for id, session := range m.sessions {
		session.Close()
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	if m.disc != nil {
		m.disc.Close()
	}
	if m.sig != nil {
		m.sig.Close()
	}
	if m.relay != nil {
		m.relay.Close()
	}
	if m.dht != nil {
		m.dht.Close()
	}

	return m.node.Close()
}

func (m *MobileNode) GenerateE2EEKey() (string, error) {
	kp, err := crypto.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	m.mu.Lock()
	m.e2eeKey = kp
	m.mu.Unlock()
	_, pubHex := kp.Marshal()
	return pubHex, nil
}

func (m *MobileNode) SetRemoteKey(peerID string, pubKeyHex string) error {
	pubRaw, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return fmt.Errorf("hex decode: %w", err)
	}
	var pub [32]byte
	copy(pub[:], pubRaw)

	m.mu.Lock()
	if m.e2eeKey == nil {
		m.mu.Unlock()
		return fmt.Errorf("local E2EE key not generated")
	}
	secret := crypto.SharedSecret(m.e2eeKey.PrivateKey, pub)
	m.sharedSec[peerID] = secret
	m.mu.Unlock()
	return nil
}

type EncryptResult struct {
	Ciphertext string
}

func (m *MobileNode) EncryptMessage(peerID string, plaintext []byte) (*EncryptResult, error) {
	m.mu.Lock()
	secret, ok := m.sharedSec[peerID]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no shared secret for peer: %s", peerID)
	}
	ct, err := crypto.Encrypt(secret, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	return &EncryptResult{Ciphertext: hex.EncodeToString(ct)}, nil
}

type DecryptResult struct {
	Plaintext []byte
}

func (m *MobileNode) DecryptMessage(peerID string, ciphertextHex string) (*DecryptResult, error) {
	ct, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	m.mu.Lock()
	secret, ok := m.sharedSec[peerID]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("no shared secret for peer: %s", peerID)
	}
	pt, err := crypto.Decrypt(secret, ct)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return &DecryptResult{Plaintext: pt}, nil
}

func (m *MobileNode) HasSharedSecret(peerID string) bool {
	m.mu.Lock()
	_, ok := m.sharedSec[peerID]
	m.mu.Unlock()
	return ok
}

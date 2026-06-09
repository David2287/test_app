package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const signalingProtocol = protocol.ID("/p2p/signaling/1.0.0")

type SignalMessageType string

const (
	SignalOffer        SignalMessageType = "offer"
	SignalAnswer       SignalMessageType = "answer"
	SignalICECandidate SignalMessageType = "ice-candidate"
)

type SignalMessage struct {
	Type SignalMessageType `json:"type"`
	Data string            `json:"data"`
}

type SignalSession struct {
	stream network.Stream
	enc    *json.Encoder
	dec    *json.Decoder
	remote peer.ID
	mu     sync.Mutex
}

func newSignalSession(stream network.Stream) *SignalSession {
	return &SignalSession{
		stream: stream,
		enc:    json.NewEncoder(stream),
		dec:    json.NewDecoder(stream),
		remote: stream.Conn().RemotePeer(),
	}
}

func (s *SignalSession) Remote() peer.ID {
	return s.remote
}

func (s *SignalSession) Send(ctx context.Context, msg SignalMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return s.enc.Encode(msg)
}

func (s *SignalSession) Receive(ctx context.Context) (SignalMessage, error) {
	type result struct {
		msg SignalMessage
		err error
	}

	done := make(chan result, 1)

	go func() {
		var msg SignalMessage
		err := s.dec.Decode(&msg)
		done <- result{msg, err}
	}()

	select {
	case <-ctx.Done():
		return SignalMessage{}, ctx.Err()
	case r := <-done:
		return r.msg, r.err
	}
}

func (s *SignalSession) Close() error {
	return s.stream.Close()
}

type Signaling struct {
	node    *Node
	handler func(*SignalSession)
	mu      sync.Mutex
	started bool
}

func NewSignaling(node *Node) *Signaling {
	return &Signaling{node: node}
}

func (s *Signaling) SetHandler(fn func(*SignalSession)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = fn
}

func (s *Signaling) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	s.node.host.SetStreamHandler(signalingProtocol, s.handleStream)
	s.started = true
	return nil
}

func (s *Signaling) handleStream(stream network.Stream) {
	session := newSignalSession(stream)

	s.mu.Lock()
	handler := s.handler
	s.mu.Unlock()

	if handler != nil {
		handler(session)
	}
}

func (s *Signaling) Dial(ctx context.Context, remote peer.ID) (*SignalSession, error) {
	stream, err := s.node.host.NewStream(ctx, remote, signalingProtocol)
	if err != nil {
		return nil, fmt.Errorf("signaling dial: %w", err)
	}
	return newSignalSession(stream), nil
}

func (s *Signaling) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		s.node.host.RemoveStreamHandler(signalingProtocol)
		s.started = false
	}
}

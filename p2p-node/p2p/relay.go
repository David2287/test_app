package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	circuit "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/multiformats/go-multiaddr"
)

type Relay struct {
	relay *circuit.Relay
}

func NewRelay(node *Node) (*Relay, error) {
	r, err := circuit.New(node.host)
	if err != nil {
		return nil, fmt.Errorf("circuit.NewRelay: %w", err)
	}
	return &Relay{relay: r}, nil
}

func (r *Relay) Close() error {
	return r.relay.Close()
}

func CircuitAddr(relayP2pAddr multiaddr.Multiaddr, destID peer.ID) multiaddr.Multiaddr {
	circuitPart, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p-circuit/p2p/%s", destID))
	return relayP2pAddr.Encapsulate(circuitPart)
}

func ReserveRelay(ctx context.Context, node *Node, relayP2pAddr multiaddr.Multiaddr) (*peer.AddrInfo, error) {
	info, err := peer.AddrInfoFromP2pAddr(relayP2pAddr)
	if err != nil {
		return nil, fmt.Errorf("parse relay addr: %w", err)
	}
	_, err = client.Reserve(ctx, node.host, *info)
	if err != nil {
		return nil, fmt.Errorf("client.Reserve: %w", err)
	}
	circuitAddr := CircuitAddr(relayP2pAddr, node.peerID)
	return &peer.AddrInfo{
		ID:    node.peerID,
		Addrs: []multiaddr.Multiaddr{circuitAddr},
	}, nil
}

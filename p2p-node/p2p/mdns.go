package p2p

import (
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type Notifee = mdns.Notifee

type Discovery struct {
	node    *Node
	Service mdns.Service
	Notifee Notifee
	once    sync.Once
}

func NewDiscovery(node *Node, serviceName string, notifee Notifee) *Discovery {
	if serviceName == "" {
		serviceName = mdns.ServiceName
	}
	svc := mdns.NewMdnsService(node.Host(), serviceName, notifee)
	return &Discovery{
		node:    node,
		Service: svc,
		Notifee: notifee,
	}
}

func (d *Discovery) Start() error {
	if d.Service == nil {
		return fmt.Errorf("mDNS service is nil")
	}
	return d.Service.Start()
}

func (d *Discovery) Close() error {
	var err error
	d.once.Do(func() {
		if d.Service != nil {
			err = d.Service.Close()
		}
	})
	return err
}

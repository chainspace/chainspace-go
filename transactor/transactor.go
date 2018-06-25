package transactor

import (
	"time"

	"github.com/tav/golly/log"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/network"
	"golang.org/x/net/context"
)

// Config represent the configuration required to send messages
// using the transactor
type Config struct {
	NetworkConfig config.Network
	NetworkName   string
	NodeID        uint64
}

type Client interface {
	SendTransaction(t *Transaction) error
}

type client struct {
	nodeID   uint64
	topology *network.Topology
}

func New(cfg *Config) (Client, error) {
	topology, err := network.New(cfg.NetworkName, &cfg.NetworkConfig)
	if err != nil {
		return nil, err
	}

	return &client{
		nodeID:   cfg.NodeID,
		topology: topology,
	}, nil
}

func (c *client) SendTransaction(t *Transaction) error {
	// Bootstrap using mDNS.
	if err := c.topology.BootstrapMDNS(); err != nil {
		return err
	}

	for c.topology.Lookup(3) == "" {
	}

	log.Infof("sending message to: %v", c.topology.Lookup(3))
	conn, err := c.topology.Dial(context.Background(), 3)
	if err != nil {
		return err
	}

	conn.Send([]byte("testing"))
	time.Sleep(5 * time.Second)

	return nil
}

package transactor

import (
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/network"
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
	//c.topology.Listen(context.Background(), c.nodeID)
	return nil
}

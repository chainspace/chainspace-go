package transactorclient

import (
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/transactor"
)

// Config represent the configuration required to send messages
// using the transactor
type Config struct {
	NetworkConfig config.Network
	NetworkName   string
	ShardID       uint64
}

type Client interface {
	SendTransaction(t *transactor.Transaction) error
}

type client struct {
	shardID  uint64
	topology *network.Topology
}

func New(cfg *Config) (Client, error) {
	topology, err := network.New(cfg.NetworkName, &cfg.NetworkConfig)
	if err != nil {
		return nil, err
	}
	if err := topology.BootstrapMDNS(); err != nil {
		return nil, err
	}

	return &client{
		shardID:  cfg.ShardID,
		topology: topology,
	}, nil
}

func (c *client) SendTransaction(t *transactor.Transaction) error {
	// Bootstrap using mDNS.

	time.Sleep(time.Second)
	conn, err := c.topology.DialAnyInShard(c.shardID)
	if err != nil {
		return err
	}

	conn.Write([]byte("testing"))
	time.Sleep(5 * time.Second)

	return nil
}

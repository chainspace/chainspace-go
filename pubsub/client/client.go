package client

import (
	"fmt"
	"net"
	"sync"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/pubsub/internal"
)

type Callback func(internal.Payload)

type Client struct {
	nodes       map[uint64]string
	conns       map[uint64]*internal.Conn
	mu          sync.Mutex
	networkName string
}

type Config struct {
	NetworkName string
	NodeAddrs   map[uint64]string
}

func (c *Client) cb(nodeID uint64, addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[nodeID] = addr
	fmt.Printf("nodeID: %v -> addr: %v", nodeID, addr)
}

func (c *Client) Run(cb Callback) error {
	for n, a := range c.nodes {
		conn, err := net.Dial("tcp", a)
		if err != nil {
			return err
		}
		c.conns[n] = internal.NewConn(conn)
	}

	for n, c := range c.conns {
		go func(nodeID uint64, conn *internal.Conn) {
			for {
				payload, err := conn.Read(5 * time.Second)
				if err != nil {
					log.Error("unable to read from conn", fld.NodeID(nodeID), fld.Err(err))
				}
			}
		}(n, c)
	}
	return nil
}

func New(cfg *Config) *Client {
	c := &Client{
		nodes:       cfg.NodeAddrs,
		conns:       map[uint64]*internal.Conn{},
		networkName: cfg.NetworkName,
	}
	if len(c.nodes) <= 0 {
		c.nodes = map[uint64]string{}
		BootstrapMDNS(cfg.NetworkName, c.cb)
	}
	return c
}

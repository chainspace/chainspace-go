package client

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"chainspace.io/prototype/pubsub/internal"
)

type Callback func(nodeID uint64, objectID string, success bool)

type Client struct {
	usercb      Callback
	nodes       map[uint64]string
	conns       map[uint64]*internal.Conn
	mu          sync.Mutex
	networkName string
	ctx         context.Context
}

type Config struct {
	NetworkName string
	NodeAddrs   map[uint64]string
	CB          Callback
	Ctx         context.Context
}

func (c *Client) cb(nodeID uint64, addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if naddr, ok := c.nodes[nodeID]; ok {
		if naddr == addr {
			return
		}
	}
	fmt.Printf("new node [id=%v, addr=%v]\n", nodeID, addr)
	c.nodes[nodeID] = addr
	if err := c.run(nodeID, addr); err != nil {
		fmt.Printf("error: unable to dial with node [nodeID=%v, err=%v]\n", nodeID, err)
	}
}

func (c *Client) run(nodeID uint64, addr string) error {
	fmt.Printf("running node conn [nodeID=%v]\n", nodeID)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	c.conns[nodeID] = internal.NewConn(conn)

	go func(nodeID uint64, conn *internal.Conn) {
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				payload, err := conn.Read(5 * time.Second)
				if err != nil {
					fmt.Printf("error: unable to read from conn [nodeID=%v, err=%v]\n", nodeID, err)
					continue
				}
				if payload != nil {
					c.usercb(payload.NodeID, payload.ObjectID, payload.Success)
				}
			}
		}
	}(nodeID, c.conns[nodeID])
	return nil
}

func New(cfg *Config) *Client {
	c := &Client{
		usercb:      cfg.CB,
		nodes:       cfg.NodeAddrs,
		conns:       map[uint64]*internal.Conn{},
		networkName: cfg.NetworkName,
		ctx:         cfg.Ctx,
	}
	if len(c.nodes) <= 0 {
		c.nodes = map[uint64]string{}
		BootstrapMDNS(cfg.NetworkName, c.cb)
	} else {
		for nodeID, addr := range c.nodes {
			if err := c.run(nodeID, addr); err != nil {
				fmt.Printf("error: unable to dial with node [nodeID=%v, err=%v]\n", nodeID, err)
			}
		}
	}

	return c
}

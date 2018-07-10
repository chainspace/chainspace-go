package transactor

import (
	"sync"

	"chainspace.io/prototype/network"
	"golang.org/x/net/context"
)

type ConnsCache struct {
	conns map[uint64]*network.Conn
	ctx   context.Context
	mtx   *sync.Mutex
	top   *network.Topology
}

func (c *ConnsCache) Close() error {
	for _, v := range c.conns {
		if err := v.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConnsCache) dial(nodeID uint64) (*network.Conn, error) {
	c.mtx.Lock()
	if c, ok := c.conns[nodeID]; ok {
		return c, nil
	}
	c.mtx.Unlock()
	conn, err := c.top.Dial(c.ctx, nodeID)
	if err != nil {
		return nil, err
	}
	c.mtx.Lock()
	c.conns[nodeID] = conn
	c.mtx.Unlock()
	return conn, nil
}

func NewConns(ctx context.Context, top *network.Topology) *ConnsCache {
	return &ConnsCache{
		conns: map[uint64]*network.Conn{},
		ctx:   ctx,
		mtx:   &sync.Mutex{},
		top:   top,
	}
}

package transactor

import (
	"sync"
	"time"

	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"golang.org/x/net/context"
)

type ConnsCache struct {
	conns      map[uint64]*network.Conn
	ctx        context.Context
	maxPayload int
	mtx        *sync.Mutex
	top        *network.Topology
}

func (c *ConnsCache) Close() error {
	for _, v := range c.conns {
		if err := v.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConnsCache) sendHello(conn *network.Conn) error {
	hellomsg := &service.Hello{
		Type: service.CONNECTION_TRANSACTOR,
	}
	return conn.WritePayload(hellomsg, c.maxPayload, time.Second)
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
	err = c.sendHello(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.mtx.Lock()
	c.conns[nodeID] = conn
	c.mtx.Unlock()
	return conn, nil
}

func (c *ConnsCache) WriteRequest(nodeID uint64, msg *service.Message, timeout time.Duration) (uint64, error) {
	conn, err := c.dial(nodeID)
	if err != nil {
		return 0, err
	}
	return conn.WriteRequest(msg, c.maxPayload, timeout)
}

func NewConnsCache(ctx context.Context, top *network.Topology, maxPayload int) *ConnsCache {
	return &ConnsCache{
		conns: map[uint64]*network.Conn{},
		ctx:   ctx,
		mtx:   &sync.Mutex{},
		top:   top,
	}
}

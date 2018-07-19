package transactor // import "chainspace.io/prototype/transactor"

import (
	"sync"
	"time"

	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"

	"golang.org/x/net/context"
)

type ConnsCache struct {
	conns      map[uint64]*network.Conn
	ctx        context.Context
	key        signature.KeyPair
	maxPayload int
	mu         sync.Mutex
	selfID     uint64
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

func (c *ConnsCache) sendHello(nodeID uint64, conn *network.Conn) error {
	hellomsg, err := service.SignHello(c.selfID, nodeID, c.key, service.CONNECTION_TRANSACTOR)
	if err != nil {
		return err
	}
	return conn.WritePayload(hellomsg, c.maxPayload, time.Second)
}

func (c *ConnsCache) dial(nodeID uint64) (*network.Conn, error) {
	c.mu.Lock()
	conn, ok := c.conns[nodeID]
	c.mu.Unlock()
	if ok {
		return conn, nil
	}
	conn, err := c.top.Dial(c.ctx, nodeID)
	if err != nil {
		return nil, err
	}
	err = c.sendHello(nodeID, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.mu.Lock()
	c.conns[nodeID] = conn
	c.mu.Unlock()
	return conn, nil
}

func (c *ConnsCache) release(nodeID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.conns, nodeID)
}

func (c *ConnsCache) WriteRequest(nodeID uint64, msg *service.Message, timeout time.Duration) (uint64, error) {
	conn, err := c.dial(nodeID)
	if err != nil {
		return 0, err
	}
	id, err := conn.WriteRequest(msg, c.maxPayload, timeout)
	if err != nil {
		c.release(nodeID)
		return c.WriteRequest(nodeID, msg, timeout)
	}
	return id, err
}

func NewConnsCache(ctx context.Context, nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair) *ConnsCache {
	return &ConnsCache{
		conns:      map[uint64]*network.Conn{},
		ctx:        ctx,
		maxPayload: maxPayload,
		selfID:     nodeID,
		top:        top,
		key:        key,
	}
}

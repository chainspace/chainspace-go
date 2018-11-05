package conns // import "chainspace.io/prototype/internal/conns"

import (
	"sync"

	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
)

type Pool interface {
	MessageAckPending() int
	Close()
	Borrow() Cache
}

type pool struct {
	mu    sync.Mutex
	i     int
	size  int
	conns []*cache
}

func NewPool(size int, nodeID uint64, top network.NetTopology, maxPayload int, key signature.KeyPair, connection service.CONNECTION) *pool {
	conns := make([]*cache, 0, size)
	for i := 0; i < size; i += 1 {
		cc := NewCache(
			nodeID, top, maxPayload, key, connection)
		conns = append(conns, cc)
	}
	return &pool{
		i:     0,
		size:  size,
		conns: conns,
	}
}

func (c *pool) MessageAckPending() int {
	c.mu.Lock()
	cnt := 0
	for _, v := range c.conns {
		v := v
		cnt += len(v.pendingAcks)
	}
	c.mu.Unlock()
	return cnt
}

func (c *pool) Close() {
	for _, conn := range c.conns {
		conn := conn
		conn.Close()
	}
}

func (c *pool) Borrow() Cache {
	c.mu.Lock()
	cc := c.conns[c.i]
	c.i += 1
	if c.i >= c.size {
		c.i = 0
	}
	c.mu.Unlock()
	return cc
}

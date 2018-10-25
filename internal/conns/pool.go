package conns // import "chainspace.io/prototype/internal/conns"

import (
	"sync"

	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
)

type Pool struct {
	mu    sync.Mutex
	i     int
	size  int
	conns []*Cache
}

func NewPool(size int, nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair, connection service.CONNECTION) *Pool {
	conns := make([]*Cache, 0, size)
	for i := 0; i < size; i += 1 {
		cc := NewCache(
			nodeID, top, maxPayload, key, connection)
		conns = append(conns, cc)
	}
	return &Pool{
		i:     0,
		size:  size,
		conns: conns,
	}
}

func (c *Pool) Close() {
	for _, conn := range c.conns {
		conn := conn
		conn.Close()
	}
}

func (c *Pool) Borrow() *Cache {
	c.mu.Lock()
	cc := c.conns[c.i]
	c.i += 1
	if c.i >= c.size {
		c.i = 0
	}
	c.mu.Unlock()
	return cc
}

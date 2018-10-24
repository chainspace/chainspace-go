package transactor // import "chainspace.io/prototype/transactor"

import (
	"sync"

	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/network"
)

type ConnsPool struct {
	mu    sync.Mutex
	i     int
	size  int
	conns []*ConnsCache
}

func NewConnsPool(size int, nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair) *ConnsPool {
	conns := make([]*ConnsCache, 0, size)
	for i := 0; i < size; i += 1 {
		cc := NewConnsCache(
			nodeID, top, maxPayload, key)
		conns = append(conns, cc)
	}
	return &ConnsPool{
		i:     0,
		size:  size,
		conns: conns,
	}
}

func (c *ConnsPool) Close() {
	for _, conn := range c.conns {
		conn := conn
		conn.Close()
	}
}

func (c *ConnsPool) Borrow() *ConnsCache {
	c.mu.Lock()
	cc := c.conns[c.i]
	c.i += 1
	if c.i >= c.size {
		c.i = 0
	}
	c.mu.Unlock()
	return cc
}

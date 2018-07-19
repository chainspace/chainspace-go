package broadcast

import (
	"sync"
)

type cache struct {
	data   map[uint64]*SignedData
	latest uint64
	mu     sync.Mutex
}

func (c *cache) get(round uint64) *SignedData {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[round]
}

func (c *cache) prune(size int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.data) <= size {
		return
	}
	limit := c.latest - uint64(size)
	ndata := map[uint64]*SignedData{}
	for round, block := range c.data {
		if round >= limit {
			ndata[round] = block
		}
	}
	c.data = ndata
}

func (c *cache) set(round uint64, block *SignedData) {
	c.mu.Lock()
	c.data[round] = block
	c.latest = round
	c.mu.Unlock()
}

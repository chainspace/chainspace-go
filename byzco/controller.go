package byzco

import (
	"context"
)

type controller struct {
	cancel    context.CancelFunc
	ctx       context.Context
	graph     *Graph
	instances map[uint64]map[uint64]*instance
	round     uint64
	resolved  map[uint64]string
}

func (c *controller) addEdge(from BlockID, to BlockID) {
}

func (c *controller) callback(perspective uint64, node uint64, hash string) {
	c.resolved[node] = hash
	if len(c.resolved) == len(c.graph.nodes) {
		c.cancel()
		c.graph.resolve(c.round, c.resolved)
	}
}

func (c *controller) run() {
	instances := map[uint64]map[uint64]*instance{}
	resolved := map[uint64]string{}
	c.ctx, c.cancel = context.WithCancel(c.graph.ctx)
	c.instances = instances
	c.resolved = resolved
	for _, perspective := range c.graph.nodes {
		nodes := map[uint64]*instance{}
		for _, node := range c.graph.nodes {
			nodes[node] = newInstance(c, perspective, node, c.round)
		}
		instances[perspective] = nodes
	}
}

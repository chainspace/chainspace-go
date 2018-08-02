package byzco

import (
	"context"
	"sync"
	"time"
)

type blockInfo struct {
	links []BlockID
	min   uint64
	max   uint64
}

// Graph represents the graph that is generated from the nodes in a shard
// broadcasting to each other.
type Graph struct {
	c      *controller
	cb     func(*Interpreted)
	ctx    context.Context
	data   map[BlockID]blockInfo
	mu     sync.RWMutex
	nodes  []uint64
	quorum int
	round  uint64
	rounds map[uint64]map[BlockID]struct{}
	self   uint64
	valid  int
}

func (g *Graph) interpret(round uint64) {
	c := &controller{graph: g, round: round}
	g.mu.Lock()
	g.c = c
	g.round = round
	g.mu.Unlock()
	go c.run()
}

func (g *Graph) prune() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			data := map[BlockID]blockInfo{}
			g.mu.Lock()
			round := g.round
			for block, info := range g.data {
				if info.max < round {
					continue
				}
				data[block] = info
			}
			g.data = data
			g.mu.Unlock()
		case <-g.ctx.Done():
			return
		}
	}
}

func (g *Graph) resolve(round uint64, hashes map[uint64]string) {
	var blocks []BlockID
	for node, hash := range hashes {
		if hash == "-" {
			continue
		}
		blocks = append(blocks, BlockID{Hash: hash, NodeID: node, Round: round})
	}
	g.cb(&Interpreted{
		Blocks: blocks,
		Round:  round,
	})
	g.interpret(round + 1)
}

// Add updates the graph and notifies the appropriate controllers.
func (g *Graph) Add(block BlockID, links []BlockID) {
	min := block.Round
	max := block.Round
	for _, link := range links {
		if link.Round < min {
			min = link.Round
		} else if link.Round > max {
			max = link.Round
		}
	}
	g.mu.Lock()
	g.data[block] = blockInfo{
		links: links,
		min:   min,
		max:   max,
	}
	c := g.c
	g.mu.Unlock()
	_ = c
}

// New instantiates a Graph for use by the broadcast/consensus mechanism.
func New(ctx context.Context, nodes []uint64, lastRound uint64, cb func(*Interpreted)) *Graph {
	f := (len(nodes) - 1) / 3
	g := &Graph{
		cb:     cb,
		ctx:    ctx,
		data:   map[BlockID]blockInfo{},
		nodes:  nodes,
		quorum: (2 * f) + 1,
		self:   nodes[0],
		valid:  f + 1,
	}
	g.interpret(lastRound + 1)
	go g.prune()
	return g
}

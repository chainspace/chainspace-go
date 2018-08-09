package byzco

import (
	"context"
	"sync"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

type blockInfo struct {
	block BlockID
	links []BlockID
	min   uint64
	max   uint64
}

// Config represents the configuration of a byzco Graph.
type Config struct {
	LastConsumed    uint64
	LastInterpreted uint64
	Nodes           []uint64
	SelfID          uint64
}

// Graph represents the graph that is generated from the nodes in a shard
// broadcasting to each other.
type Graph struct {
	blocks   []blockInfo
	c        *controller
	cb       func(*Interpreted)
	consumed uint64
	ctx      context.Context
	data     map[BlockID]blockInfo
	mu       sync.RWMutex
	nodes    []uint64
	quorum   int
	round    uint64
	rounds   map[uint64]map[BlockID]struct{}
	self     uint64
	valid    int
}

func (g *Graph) interpret(round uint64) {
	c := &controller{graph: g, round: round}
	c.run()
	g.mu.Lock()
	g.c = c
	g.round = round
	g.mu.Unlock()
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
func (g *Graph) Add(data *BlockGraph) {
	log.Info("Adding block to graph", fld.BlockID(data.Block))
	for _, dep := range data.Deps {
		log.Info("Dep:", fld.BlockID(dep.Block))
	}
	// min := block.Round
	// max := block.Round
	// for _, link := range links {
	// 	if link.Round < min {
	// 		min = link.Round
	// 	} else if link.Round > max {
	// 		max = link.Round
	// 	}
	// }
	// info := blockInfo{
	// 	links: links,
	// 	min:   min,
	// 	max:   max,
	// }
	// g.mu.Lock()
	// g.data[block] = info
	// c := g.c
	// g.mu.Unlock()
	// if min >= c.round {
	// 	e := event{block, links}
	// 	for _, instance := range c.instances[block.NodeID] {
	// 		instance.addEvent(e)
	// 	}
	// }
}

// New instantiates a Graph for use by the broadcast/consensus mechanism.
func New(ctx context.Context, cfg *Config, cb func(*Interpreted)) *Graph {
	f := (len(cfg.Nodes) - 1) / 3
	g := &Graph{
		cb:       cb,
		consumed: cfg.LastConsumed,
		ctx:      ctx,
		data:     map[BlockID]blockInfo{},
		nodes:    cfg.Nodes,
		quorum:   (2 * f) + 1,
		self:     cfg.SelfID,
		valid:    f + 1,
	}
	g.interpret(cfg.LastInterpreted + 1)
	return g
}

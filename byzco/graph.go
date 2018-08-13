package byzco

import (
	"context"
	"sync"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

type blockInfo struct {
	data BlockGraph
	max  uint64
}

// Config represents the configuration of a byzco Graph.
type Config struct {
	LastConsumed    uint64
	LastInterpreted uint64
	Nodes           []uint64
	SelfID          uint64
	TotalNodes      uint64
}

// Graph represents the graph that is generated from the nodes in a shard
// broadcasting to each other.
type Graph struct {
	blocks     []BlockGraph
	cb         func(*Interpreted)
	cond       *sync.Cond
	consumed   uint64
	ctx        context.Context
	data       map[BlockID]blockInfo
	entries    []*entry
	mu         sync.RWMutex
	nodes      []uint64
	quorumf1   int
	quorum2f1  int
	round      uint64
	rounds     map[uint64]map[BlockID]struct{}
	self       uint64
	states     map[BlockID]*state
	totalNodes uint64
}

func (g *Graph) interpret(round uint64) {
	// c := &controller{graph: g, round: round}
	// c.run()
	// g.mu.Lock()
	// g.c = c
	// g.round = round
	// g.mu.Unlock()
}

func (g *Graph) process(e *entry) {

	var s *state
	if e.prev.Valid() {
		s = g.states[e.prev].clone()
	} else {
		s = &state{
			data:     map[stateData]interface{}{},
			delay:    map[uint64]uint64{},
			final:    map[noderound]string{},
			timeouts: map[uint64][]timeout{},
		}
	}

	node, round, hash := e.block.Node, e.block.Round, e.block.Hash
	msgs := []message{preprepare{
		hash:  hash,
		node:  node,
		round: round,
		view:  0,
	}}
	out := []message{msgs[0]}

	for _, dep := range e.deps {
		s.delay[dep.Node] = diff(round, dep.Round)
	}

	tval := uint64(1)
	for _, val := range s.delay {
		if val > tval {
			tval = val
		}
	}
	s.timeout = tval * 10

	for _, tmout := range s.timeouts[round] {
		_, exists := s.data[final{node: tmout.node, round: tmout.round}]
		if exists {
			continue
		}
	}

	s.out = out
	g.states[e.block] = s

}

func (g *Graph) prune() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			// data := map[BlockID]blockInfo{}
			// g.mu.Lock()
			// round := g.round
			// for block, info := range g.data {
			// 	if info.max < round {
			// 		continue
			// 	}
			// 	data[block] = info
			// }
			// g.data = data
			// g.mu.Unlock()
		case <-g.ctx.Done():
			return
		}
	}
}

func (g *Graph) release() {
	<-g.ctx.Done()
	g.cond.Signal()
}

func (g *Graph) resolve(round uint64, hashes map[uint64]string) {
	var blocks []BlockID
	for node, hash := range hashes {
		if hash == "-" {
			continue
		}
		blocks = append(blocks, BlockID{Hash: hash, Node: node, Round: round})
	}
	g.cb(&Interpreted{
		Blocks: blocks,
		Round:  round,
	})
	g.interpret(round + 1)
}

func (g *Graph) run() {
	for {
		g.cond.L.Lock()
		for len(g.entries) == 0 {
			g.cond.Wait()
			select {
			case <-g.ctx.Done():
				return
			default:
			}
		}
		e := g.entries[0]
		g.entries = g.entries[1:]
		g.cond.L.Unlock()
		g.process(e)
	}
}

// Add updates the graph and notifies the appropriate controllers.
func (g *Graph) Add(data *BlockGraph) {
	log.Info("Adding block to graph", fld.BlockID(data.Block))
	for _, dep := range data.Deps {
		log.Info("Dep:", fld.BlockID(dep.Block))
	}
	self := &entry{
		block: data.Block,
		prev:  data.Prev,
	}
	self.deps = make([]BlockID, len(data.Deps)+1)
	self.deps[0] = data.Prev
	for i, dep := range data.Deps {
		self.deps[i+1] = dep.Block
	}
	deps := make([]*entry, len(data.Deps))
	for i, dep := range data.Deps {
		e := &entry{
			block: dep.Block,
			prev:  dep.Prev,
		}
		e.deps = make([]BlockID, len(dep.Deps)+1)
		e.deps[0] = dep.Prev
		copy(e.deps[1:], dep.Deps)
		deps[i] = e
	}
	g.cond.L.Lock()
	g.entries = append(g.entries, deps...)
	g.entries = append(g.entries, self)
	g.cond.L.Unlock()
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
		cb:         cb,
		cond:       sync.NewCond(&sync.Mutex{}),
		consumed:   cfg.LastConsumed,
		ctx:        ctx,
		data:       map[BlockID]blockInfo{},
		entries:    []*entry{},
		nodes:      cfg.Nodes,
		quorumf1:   f + 1,
		quorum2f1:  (2 * f) + 1,
		self:       cfg.SelfID,
		totalNodes: cfg.TotalNodes,
	}
	g.interpret(cfg.LastInterpreted + 1)
	return g
}

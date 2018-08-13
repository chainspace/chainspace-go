package byzco

import (
	"context"
	"sync"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

type blockInfo struct {
	data BlockGraph
	max  uint64
}

type minmax struct {
	min uint64
	max uint64
}

// Config represents the configuration of a byzco Graph.
type Config struct {
	LastInterpreted uint64
	Nodes           []uint64
	SelfID          uint64
	TotalNodes      uint64
}

// Graph represents the graph that is generated from the nodes in a shard
// broadcasting to each other.
type Graph struct {
	blocks     []*blockInfo
	cb         func(*Interpreted)
	cond       *sync.Cond // protects entries
	ctx        context.Context
	entries    []*entry
	minmax     map[BlockID]minmax
	mu         sync.RWMutex // protects blocks, min, round
	nodeCount  int
	nodes      []uint64
	quorumf1   int
	quorum2f1  int
	resolved   map[uint64]map[uint64]string
	round      uint64
	self       uint64
	states     map[BlockID]*state
	totalNodes uint64
}

func (g *Graph) deliver(node uint64, round uint64, hash string) {
	hashes, exists := g.resolved[round]
	if !exists {
		hashes = map[uint64]string{
			node: hash,
		}
		g.resolved[round] = hashes
	}
	if len(hashes) != g.nodeCount {
		return
	}
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
}

func (g *Graph) process(e *entry) {

	var s *state
	if e.prev.Valid() {
		s = g.states[e.prev].clone()
	} else {
		s = &state{
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

	if len(e.deps) > 0 {
		if s.delay == nil {
			s.delay = map[uint64]uint64{}
		}
		for _, dep := range e.deps {
			s.delay[dep.Node] = diff(round, dep.Round)
		}
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

func (g *Graph) release() {
	<-g.ctx.Done()
	g.cond.Signal()
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
	g.mu.RLock()
	min := data.Block.Round
	max := min
	entries := make([]*entry, len(data.Deps)+1)
	for i, dep := range data.Deps {
		log.Info("Dep:", fld.BlockID(dep.Block))
		e := &entry{
			block: dep.Block,
			prev:  dep.Prev,
		}
		e.deps = make([]BlockID, len(dep.Deps)+1)
		e.deps[0] = dep.Prev
		copy(e.deps[1:], dep.Deps)
		entries[i] = e
		round := dep.Block.Round
		if round < min {
			min = round
		} else if round > max {
			max = round
		}
		for _, link := range dep.Deps {
			dm, exists := g.minmax[link]
			if exists {
				if min > dm.max {
					min = dm.max
				} else if max < g.round {
					max = g.round
				}
			} else {
				if min > g.round {
					min = g.round
				} else if max < g.round {
					max = g.round
				}

			}
		}
	}
	g.mu.RUnlock()
	self := &entry{
		block: data.Block,
		prev:  data.Prev,
	}
	self.deps = make([]BlockID, len(data.Deps)+1)
	self.deps[0] = data.Prev
	for i, dep := range data.Deps {
		self.deps[i+1] = dep.Block
	}
	entries[len(entries)-1] = self
	g.cond.L.Lock()
	g.entries = append(g.entries, entries...)
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
}

// New instantiates a Graph for use by the broadcast/consensus mechanism.
func New(ctx context.Context, cfg *Config, cb func(*Interpreted)) *Graph {
	f := (len(cfg.Nodes) - 1) / 3
	g := &Graph{
		blocks:     []*blockInfo{},
		cb:         cb,
		cond:       sync.NewCond(&sync.Mutex{}),
		ctx:        ctx,
		entries:    []*entry{},
		minmax:     map[BlockID]minmax{},
		nodeCount:  len(cfg.Nodes),
		nodes:      cfg.Nodes,
		quorumf1:   f + 1,
		quorum2f1:  (2 * f) + 1,
		resolved:   map[uint64]map[uint64]string{},
		round:      cfg.LastInterpreted + 1,
		self:       cfg.SelfID,
		totalNodes: cfg.TotalNodes,
	}
	return g
}

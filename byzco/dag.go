package byzco

import (
	"context"
	"sync"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
)

// DAG represents the directed acyclic graph that is generated from the nodes in
// a shard broadcasting to each other.
type DAG struct {
	c      *controller
	cb     func(*Interpreted)
	ctx    context.Context
	edges  map[BlockID][]BlockID
	mu     sync.RWMutex
	nodes  []uint64
	round  uint64
	rounds map[uint64]map[BlockID]struct{}
}

func (d *DAG) interpret(round uint64) {
	c := &controller{dag: d, round: round}
	d.mu.Lock()
	d.c = c
	d.round = round
	d.mu.Unlock()
	go c.run()
}

func (d *DAG) prune() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			edges := map[BlockID][]BlockID{}
			d.mu.Lock()
			// last := d.lastRound
			for from, to := range d.edges {
				edges[from] = to
			}
			d.edges = edges
			d.mu.Unlock()
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *DAG) resolve(round uint64, hashes map[uint64]string) {
	var blocks []BlockID
	for node, hash := range hashes {
		if hash == "-" {
			continue
		}
		blocks = append(blocks, BlockID{Hash: hash, NodeID: node, Round: round})
	}
	d.cb(&Interpreted{
		Blocks: blocks,
		Round:  round,
	})
	d.interpret(round + 1)
}

// AddEdge updates the DAG with an edge between blocks.
func (d *DAG) AddEdge(from BlockID, to BlockID) {
	d.mu.Lock()
	d.edges[from] = append(d.edges[from], to)
	seen, exists := d.rounds[from.Round]
	if !exists {
		seen = map[BlockID]struct{}{}
	}
	seen[to] = struct{}{}
	d.mu.Unlock()
	log.Info("Adding DAG edge", fld.FromBlock(from), fld.ToBlock(to))
}

// NewDAG instantiates a DAG for use by the broadcast/consensus mechanism.
func NewDAG(ctx context.Context, nodes []uint64, lastRound uint64, cb func(*Interpreted)) *DAG {
	d := &DAG{
		cb:    cb,
		ctx:   ctx,
		edges: map[BlockID][]BlockID{},
		nodes: nodes,
	}
	d.interpret(lastRound + 1)
	go d.prune()
	return d
}

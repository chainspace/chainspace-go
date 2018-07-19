package byzco

import (
	"context"
	"sync"
	"time"
)

// DAG represents the directed acyclic graph that is generated from the nodes in
// a shard broadcasting to each other.
type DAG struct {
	cb    func(*Interpreted)
	ctx   context.Context
	mu    sync.RWMutex
	round uint64
	state map[BlockID][]BlockID
}

func (d *DAG) prune() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			state := map[BlockID][]BlockID{}
			d.mu.Lock()
			// last := d.lastRound
			for from, to := range d.state {
				state[from] = to
			}
			d.state = state
			d.mu.Unlock()
		case <-d.ctx.Done():
			return
		}
	}
}

// AddEdge updates the DAG with an edge between blocks.
func (d *DAG) AddEdge(from BlockID, to []BlockID) {
	d.mu.Lock()
	d.state[from] = to
	d.mu.Unlock()
}

// NewDAG instantiates a DAG for use by the broadcast/consensus mechanism.
func NewDAG(ctx context.Context, nodes []uint64, lastRound uint64, cb func(*Interpreted)) *DAG {
	d := &DAG{
		cb:    cb,
		ctx:   ctx,
		round: lastRound + 1,
		state: map[BlockID][]BlockID{},
	}
	go d.prune()
	return d
}

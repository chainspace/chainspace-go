package byzco

type BlockID struct {
	Hash   string
	NodeID uint64
	Round  uint64
}

// DAG represents the directed acyclic graph that is generated from the nodes in
// a shard broadcasting to each other.
type DAG struct {
	cb func(round uint64, blocks []BlockID)
}

// AddEdge updates the DAG with an edge between two nodes labeled with the given
// round.
func (d *DAG) AddEdge(from uint64, to uint64, round uint64) {}

// NewDAG instantiates a DAG for use by the broadcast/consensus mechanism.
func NewDAG(cb func(round uint64, blocks []BlockID)) *DAG {
	return &DAG{
		cb: cb,
	}
}

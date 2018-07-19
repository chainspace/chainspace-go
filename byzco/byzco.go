// Package byzco implements the core ByzCoChain consensus algorithm.
package byzco // import "chainspace.io/prototype/byzco"

// BlockID represents a specific block from a node in a way that can be used as
// a map key.
type BlockID struct {
	Hash   string
	NodeID uint64
	Round  uint64
}

// Interpreted represents the results of interpreting a DAG for a particular
// round.
type Interpreted struct {
	Blocks []BlockID
	Round  uint64
}

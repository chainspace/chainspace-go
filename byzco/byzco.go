// Package byzco implements the core ByzCoChain consensus algorithm.
package byzco // import "chainspace.io/prototype/byzco"

import (
	"strconv"
)

const (
	hexUpper = "0123456789ABCDEF"
)

// BlockGraph represents a block generated by a node and its dependencies.
type BlockGraph struct {
	Block BlockID
	Deps  []Dep
	Prev  BlockID
}

// BlockID represents a specific block from a node in a way that can be used as
// a map key.
type BlockID struct {
	Hash   string
	NodeID uint64
	Round  uint64
}

func (b BlockID) String() string {
	var v []byte
	v = strconv.AppendUint(v, b.NodeID, 10)
	v = append(v, ' ', '|', ' ')
	v = strconv.AppendUint(v, b.Round, 10)
	v = append(v, ' ', '|', ' ')
	for i := 6; i < 12; i++ {
		c := b.Hash[i]
		v = append(v, hexUpper[c>>4], hexUpper[c&0x0f])
	}
	return string(v)
}

// Valid returns whether the BlockID is not the zero value.
func (b BlockID) Valid() bool {
	return b.Hash != ""
}

// Dep represents a direct dependency of a block generated by a node.
type Dep struct {
	Block BlockID
	Deps  []BlockID
	Prev  BlockID
}

// Interpreted represents the results of interpreting a Graph for a particular
// round.
type Interpreted struct {
	Blocks   []BlockID
	Consumed uint64
	Round    uint64
}

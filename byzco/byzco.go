// Package byzco implements the core ByzCoChain consensus algorithm.
package byzco // import "chainspace.io/prototype/byzco"

import (
	"strconv"
)

const (
	hexUpper = "0123456789ABCDEF"
)

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

// Interpreted represents the results of interpreting a DAG for a particular
// round.
type Interpreted struct {
	Blocks []BlockID
	Round  uint64
}

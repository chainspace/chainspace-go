package messages

import "fmt"

// PrePrepare represents a pre-prepare message sent between nodes in a consensus round
type PrePrepare struct {
	Hash  string
	Node  uint64
	Round uint64
	View  uint32
}

// Kind is PrePrepareMsg
func (p PrePrepare) Kind() MessageKind {
	return PrePrepareMsg
}

// NodeRound returns the node and round that this message came from.
// Messages may come from any node participating in consensus.
func (p PrePrepare) NodeRound() (uint64, uint64) {
	return p.Node, p.Round
}

func (p PrePrepare) String() string {
	return fmt.Sprintf(
		"pre-prepare{node: %d, round: %d, view: %d, hash: '%s'}",
		p.Node, p.Round, p.View, fmtHash(p.Hash),
	)
}

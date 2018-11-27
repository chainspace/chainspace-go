package messages

import "fmt"

// ViewChange represents a view-change message sent between nodes in a consensus round
type ViewChange struct {
	Hash   string
	Node   uint64
	Round  uint64
	Sender uint64
	View   uint32
}

// Kind is ViewChangedMsg
func (v ViewChange) Kind() MessageKind {
	return ViewChangedMsg
}

// NodeRound returns the node and round that this message came from.
// Messages may come from any node participating in consensus.
func (v ViewChange) NodeRound() (uint64, uint64) {
	return v.Node, v.Round
}

func (v ViewChange) String() string {
	return fmt.Sprintf(
		"view-change{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		v.Node, v.Round, v.View, fmtHash(v.Hash), v.Sender,
	)
}

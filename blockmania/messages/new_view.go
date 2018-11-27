package messages

import "fmt"

// NewView represents a new view message sent between nodes in a consensus round
type NewView struct {
	Hash   string
	Node   uint64
	Round  uint64
	Sender uint64
	View   uint32
}

// Kind is NewViewMsg
func (n NewView) Kind() MessageKind {
	return NewViewMsg
}

// NodeRound returns the node and round that this message came from.
// Messages may come from any node participating in consensus.
func (n NewView) NodeRound() (uint64, uint64) {
	return n.Node, n.Round
}

func (n NewView) String() string {
	return fmt.Sprintf(
		"new-view{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		n.Node, n.Round, n.View, fmtHash(n.Hash), n.Sender,
	)
}

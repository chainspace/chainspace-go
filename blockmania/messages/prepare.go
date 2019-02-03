package messages

import "fmt"

// Prepare represents a prepare message sent between nodes in a consensus round
type Prepare struct {
	Hash   string
	Node   uint64
	Round  uint64
	Sender uint64
	View   uint32
}

// Kind is PrepareMsg
func (p Prepare) Kind() MessageKind {
	return PrepareMsg
}

// NodeRound returns the node and round that this message came from.
// Messages may come from any node participating in consensus.
func (p Prepare) NodeRound() (uint64, uint64) {
	return p.Node, p.Round
}

// Pre formats a PrePrepare message out of this Prepare message.
// TODO: Dave check this with George: I suspect this is used in the protocol
// when a given node has committed to deliver but no consensus has been reached.
// The Commit is turned into a PrePrepare and re-used.
func (p Prepare) Pre() PrePrepare {
	return PrePrepare{Hash: p.Hash, Node: p.Node, Round: p.Round, View: p.View}
}

func (p Prepare) String() string {
	return fmt.Sprintf(
		"prepare{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		p.Node, p.Round, p.View, fmtHash(p.Hash), p.Sender,
	)
}

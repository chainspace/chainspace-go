package messages

import "fmt"

// Commit represents a commit message sent between nodes in a consensus round
type Commit struct {
	Hash   string
	Node   uint64
	Round  uint64
	Sender uint64
	View   uint32
}

// Kind is CommitMsg
func (c Commit) Kind() MessageKind {
	return CommitMsg
}

// NodeRound returns the node and round that this message came from.
// Messages may come from any node participating in consensus.
func (c Commit) NodeRound() (uint64, uint64) {
	return c.Node, c.Round
}

// Pre formats a PrePrepare message out of this commit message.
// TODO: Dave check this with George: I suspect this is used in the protocol
// when a given node has committed to deliver but no consensus has been reached.
// The Commit is turned into a PrePrepare and re-used.
func (c Commit) Pre() PrePrepare {
	return PrePrepare{Hash: c.Hash, Node: c.Node, Round: c.Round, View: c.View}
}

func (c Commit) String() string {
	return fmt.Sprintf(
		"commit{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		c.Node, c.Round, c.View, fmtHash(c.Hash), c.Sender,
	)
}

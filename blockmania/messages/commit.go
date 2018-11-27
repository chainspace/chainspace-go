package messages

import "fmt"

type Commit struct {
	Hash   string
	Node   uint64
	Round  uint64
	Sender uint64
	View   uint32
}

func (c Commit) Kind() MessageKind {
	return CommitMsg
}

func (c Commit) NodeRound() (uint64, uint64) {
	return c.Node, c.Round
}

func (c Commit) Pre() PrePrepare {
	return PrePrepare{Hash: c.Hash, Node: c.Node, Round: c.Round, View: c.View}
}

func (c Commit) String() string {
	return fmt.Sprintf(
		"commit{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		c.Node, c.Round, c.View, fmtHash(c.Hash), c.Sender,
	)
}

package messages

import "fmt"

type Prepare struct {
	Hash   string
	Node   uint64
	Round  uint64
	Sender uint64
	View   uint32
}

func (p Prepare) Kind() MessageKind {
	return PrepareMsg
}

func (p Prepare) NodeRound() (uint64, uint64) {
	return p.Node, p.Round
}

func (p Prepare) Pre() PrePrepare {
	return PrePrepare{Hash: p.Hash, Node: p.Node, Round: p.Round, View: p.View}
}

func (p Prepare) String() string {
	return fmt.Sprintf(
		"prepare{node: %d, round: %d, view: %d, hash: '%s', sender: %d}",
		p.Node, p.Round, p.View, fmtHash(p.Hash), p.Sender,
	)
}

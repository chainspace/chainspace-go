package messages

import "fmt"

type PrePrepare struct {
	Hash  string
	Node  uint64
	Round uint64
	View  uint32
}

func (p PrePrepare) Kind() MessageKind {
	return PrePrepareMsg
}

func (p PrePrepare) NodeRound() (uint64, uint64) {
	return p.Node, p.Round
}

func (p PrePrepare) String() string {
	return fmt.Sprintf(
		"pre-prepare{node: %d, round: %d, view: %d, hash: '%s'}",
		p.Node, p.Round, p.View, fmtHash(p.Hash),
	)
}

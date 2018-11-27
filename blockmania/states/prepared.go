package states

type Prepared struct {
	Node  uint64
	Round uint64
	View  uint32
}

func (p Prepared) GetRound() uint64 {
	return p.Round
}

func (p Prepared) SdKind() StateDataKind {
	return PreparedState
}

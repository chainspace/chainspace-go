package states

type PrePrepared struct {
	Node  uint64
	Round uint64
	View  uint32
}

func (p PrePrepared) GetRound() uint64 {
	return p.Round
}

func (p PrePrepared) SdKind() StateDataKind {
	return PrePreparedState
}

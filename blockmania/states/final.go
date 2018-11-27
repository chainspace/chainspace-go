package states

type Final struct {
	Node  uint64
	Round uint64
}

func (f Final) GetRound() uint64 {
	return f.Round
}

func (f Final) SdKind() StateDataKind {
	return FinalState
}

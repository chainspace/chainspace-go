package states

type ViewChanged struct {
	Node  uint64
	Round uint64
	View  uint32
}

func (v ViewChanged) GetRound() uint64 {
	return v.Round
}

func (v ViewChanged) SdKind() StateDataKind {
	return ViewChangedState
}

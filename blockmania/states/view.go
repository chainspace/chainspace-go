package states

type View struct {
	Node  uint64
	Round uint64
}

func (v View) GetRound() uint64 {
	return v.Round
}

func (v View) SdKind() StateDataKind {
	return ViewState
}

package states

type HNV struct {
	Node  uint64
	Round uint64
	View  uint32
}

func (h HNV) GetRound() uint64 {
	return h.Round
}

func (h HNV) SdKind() StateDataKind {
	return HNVState
}

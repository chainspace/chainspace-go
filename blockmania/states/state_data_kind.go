package states

import (
	"fmt"
)

const (
	UnknownState StateDataKind = iota
	FinalState
	HNVState
	PreparedState
	PrePreparedState
	ViewState
	ViewChangedState
)

type StateData interface {
	GetRound() uint64
	SdKind() StateDataKind
}

type StateDataKind uint8

func (s StateDataKind) String() string {
	switch s {
	case FinalState:
		return "final"
	case HNVState:
		return "hnv"
	case PreparedState:
		return "prepared"
	case PrePreparedState:
		return "preprepared"
	case UnknownState:
		return "unknown"
	case ViewState:
		return "viewState"
	case ViewChangedState:
		return "viewchanged"
	default:
		panic(fmt.Errorf("blockmania: unknown status data kind: %d", s))
	}
}

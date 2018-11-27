package messages

import "fmt"

const (
	UnknownMsg MessageKind = iota
	PrePrepareMsg
	PrepareMsg
	CommitMsg
	ViewChangedMsg
	NewViewMsg
)

// Message defines an interface for all the other messages to follow
type Message interface {
	Kind() MessageKind
	NodeRound() (uint64, uint64)
	String() string
}

// MessageKind is an integer identifier for message types.
type MessageKind uint8

func (m MessageKind) String() string {
	switch m {
	case CommitMsg:
		return "commit"
	case NewViewMsg:
		return "new-view"
	case PrepareMsg:
		return "prepare"
	case PrePrepareMsg:
		return "pre-prepare"
	case UnknownMsg:
		return "unknown"
	case ViewChangedMsg:
		return "view-change"
	default:
		panic(fmt.Errorf("blockmania: unknown message kind: %d", m))
	}
}

func fmtHash(v string) string {
	if v == "" {
		return ""
	}
	return fmt.Sprintf("%X", v[6:12])
}

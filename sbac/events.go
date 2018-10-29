package sbac

type EventKind uint8

const (
	EventKindSBACMessage EventKind = iota
	EventKindConsensus
)

func (e EventKind) String() string {
	if e == EventKindSBACMessage {
		return "EventKindSBACMessage"
	} else if e == EventKindConsensus {
		return "EventKindConsensus"
	}
	return "error"
}

type EventExt interface {
	Kind() EventKind
	TxID() []byte
	PeerID() uint64
}

// sbac event

type SBACEvent struct {
	kind EventKind
	msg  *SBACMessage2
}

func (e *SBACEvent) Kind() EventKind {
	return e.kind
}

func (e *SBACEvent) TxID() []byte {
	return e.msg.TxID
}

func (e *SBACEvent) PeerID() uint64 {
	return e.msg.PeerID
}

func NewSBACEvent(data *SBACMessage2) EventExt {
	return &SBACEvent{EventKindSBACMessage, data}
}

// consensus event

type ConsensusEvent struct {
	kind EventKind
	data *ConsensusTransaction
}

func (e *ConsensusEvent) Kind() EventKind {
	return e.kind
}

func (e *ConsensusEvent) TxID() []byte {
	return e.data.TxID
}

func (e *ConsensusEvent) PeerID() uint64 {
	return e.data.Initiator
}

func NewConsensusEvent(data *ConsensusTransaction) *ConsensusEvent {
	return &ConsensusEvent{EventKindConsensus, data}
}

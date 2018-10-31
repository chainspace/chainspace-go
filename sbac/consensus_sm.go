package sbac

import (
	"fmt"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	proto "github.com/gogo/protobuf/proto"
)

type StateConsensus uint8

const (
	StateConsensusWaiting StateConsensus = iota
	StateConsensusAccepted
	StateConsensusRejected
)

func (e StateConsensus) String() string {
	switch e {
	case StateConsensusWaiting:
		return "StateConsensusWaiting"
	case StateConsensusAccepted:
		return "StateConsensusAccepted"
	case StateConsensusRejected:
		return "StateConsensusRejected"
	default:
		return "error"
	}
}

type ConsensusEventAction func(st *States, e *ConsensusEvent) (StateConsensus, error)

type ConsensusStateMachine struct {
	action ConsensusEventAction
	data   *ConsensusTransaction
	phase  ConsensusOp
	state  StateConsensus
}

func (c *ConsensusStateMachine) processEvent(st *States, e *ConsensusEvent) error {
	if e.Kind() != EventKindConsensus {
		return fmt.Errorf("ConsensusStateMachine, invalid EventKind(%v)",
			e.Kind().String())
	}

	if c.state != StateConsensusWaiting {
		return fmt.Errorf("ConsensusStateMachine already finished, state(%v)",
			c.state.String())
	}

	var err error
	c.data = e.data
	c.state, err = c.action(st, e)
	return err
}

func (c *ConsensusStateMachine) State() StateConsensus {
	return c.state
}

func (c *ConsensusStateMachine) Phase() ConsensusOp {
	return c.phase
}

func (c *ConsensusStateMachine) Data() interface{} {
	return c.data
}

func NewConsensuStateMachine(phase ConsensusOp, action ConsensusEventAction) *ConsensusStateMachine {
	return &ConsensusStateMachine{
		action: action,
		phase:  phase,
		state:  StateConsensusWaiting,
	}
}

func (s *Service) onConsensusEvent(st *States, e *ConsensusEvent) (StateConsensus, error) {
	if e.data.Op != ConsensusOp_ConsensusCommit {
		txbytes, _ := proto.Marshal(e.data.Tx)
		if !s.verifyEvidenceSignature(txbytes, e.data.Evidences) {
			log.Error("consensus missing/invalid signatures",
				fld.TxID(st.detail.HashID),
				log.String("phase", e.data.Op.String()))
			return StateConsensusRejected, nil
		}
	}
	return StateConsensusAccepted, nil
}

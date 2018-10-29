package sbac

import (
	"fmt"

	proto "github.com/gogo/protobuf/proto"
)

type StateConsensus uint8

const (
	StateConsensusWaiting StateConsensus = iota
	StateConsensusAccept
	StateConsensusReject
)

func (e StateConsensus) String() string {
	switch e {
	case StateConsensusWaiting:
		return "StateConsensusWaiting"
	case StateConsensusAccept:
		return "StateConsensusAccept"
	case StateConsensusReject:
		return "StateConsensusReject"
	default:
		return "error"
	}
}

type ConsensusEventAction func(e *ConsensusEvent) (StateConsensus, error)

type ConsensusStateMachine struct {
	action ConsensusEventAction
	data   *ConsensusTransaction
	phase  ConsensusOp
	state  StateConsensus
}

func (c *ConsensusStateMachine) processEvent(e *ConsensusEvent) error {
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
	c.state, err = c.action(e)
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

func (s *Service) onConsensusEvent(e *ConsensusEvent) (StateConsensus, error) {
	txbytes, _ := proto.Marshal(e.data.Tx)
	if !s.verifySignatures(txbytes, e.data.Evidences) {
		// log.Error("consensus1 missing/invalid signatures", fld.TxID(e.data.HashID))
		return StateConsensusReject, nil
	}
	return StateConsensusAccept, nil
}

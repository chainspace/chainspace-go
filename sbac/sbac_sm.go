package sbac

import "fmt"

type StateSBAC uint8

const (
	StateSBACWaiting StateSBAC = iota
	StateSBACAccept
	StateSBACReject
)

func (e StateSBAC) String() string {
	switch e {
	case StateSBACWaiting:
		return "StateSBACWaiting"
	case StateSBACAccept:
		return "StateSBACAccept"
	case StateSBACReject:
		return "StateSBACReject"
	default:
		return "error"
	}
}

type SBACEventAction func(e *SBACEvent) (StateSBAC, error)

type SBACStateMachine struct {
	action SBACEventAction
	msgs   map[uint64]*SBACMessage2
	phase  SBACOp
	state  StateSBAC
}

func (c *SBACStateMachine) processEvent(e *SBACEvent) error {
	if e.Kind() != EventKindSBACMessage {
		return fmt.Errorf("SBACStateMachine, invalid EventKind(%v)",
			e.Kind().String())
	}

	if c.state != StateSBACWaiting {
		return fmt.Errorf("SBACStateMachine already finished, state(%v)",
			c.state.String())
	}

	var err error
	c.msgs[e.msg.PeerID] = e.msg
	c.state, err = c.action(e)
	return err
}

func (c *SBACStateMachine) Phase() SBACOp {
	return c.phase
}

func (c *SBACStateMachine) Data() interface{} {
	return c.msgs
}

func NewSBACStateMachine(phase SBACOp, action SBACEventAction) *SBACStateMachine {
	return &SBACStateMachine{
		action: action,
		phase:  phase,
		state:  StateSBACWaiting,
	}
}

func (s *Service) onSBACEvent(e *SBACEvent) (StateSBAC, error) {
	return StateSBACReject, nil
}

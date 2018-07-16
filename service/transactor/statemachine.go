package transactor // import "chainspace.io/prototype/service/transactor"
import (
	"fmt"

	"chainspace.io/prototype/log"

	"go.uber.org/zap"
)

type SBACDecisions uint8

const (
	Accept SBACDecisions = iota

	Reject
	Abort
)

type StateTable struct {
	actions     map[State]Action
	transitions map[StateTransition]Transition
}

type StateTransition struct {
	From State
	To   State
}

type TxDetails struct {
	CheckersEvidences map[uint64][]byte
	Consensus1        *ConsensusTransaction
	Consensus2        *ConsensusTransaction
	ID                []byte
	Phase1Decisions   map[uint64]SBACDecision
	Phase2Decisions   map[uint64]SBACDecision
	Raw               []byte
	Result            chan bool
	Tx                *Transaction
}

// Action specify an action to execute when a new event is triggered.
// it returns a State, which will be either the new actual state, the next state
// which may required a transition from the current state to the new one (see the transition
// table
type Action func(tx *TxDetails, event *Event) (State, error)

// Transition are called when the state is change from a current state to a new one.
// return a State which may involved a new transition as well.
type Transition func(tx *TxDetails) (State, error)

type Event struct {
	msg    *SBACMessage
	peerID uint64
}

type StateMachine struct {
	txDetails *TxDetails
	events    chan *Event
	state     State
	table     *StateTable
}

func (sm *StateMachine) Reset() {
	sm.state = StateWaitingForConsensus1
}

func (sm *StateMachine) State() State {
	return sm.state
}

func (sm *StateMachine) onNewEvent(event *Event) error {
	action, ok := sm.table.actions[sm.state]
	if !ok {
		// this may be because we received an event not related to the current state.
		// use the generic handle for this which just store the event for now.
		// and stay in the same state
		action, ok := sm.table.actions[StateAnyEvent]
		if !ok {
			return fmt.Errorf("missing action for this state")
		}
		log.Info("apply action for any event",
			zap.Uint32("id", ID(sm.txDetails.ID)),
			zap.String("state", sm.state.String()),
		)
		action(sm.txDetails, event)
		return nil
	}
	log.Info("apply action for event",
		zap.Uint32("id", ID(sm.txDetails.ID)),
		zap.String("state", sm.state.String()),
	)
	newstate, err := action(sm.txDetails, event)
	if err != nil {
		return err
	}
	txtransition := StateTransition{sm.state, newstate}
	// if a transition exist for the new state, apply it
	if t, ok := sm.table.transitions[txtransition]; ok {
		return sm.applyTransition(newstate, t)
	}
	// else save the new state directly
	sm.state = newstate
	return nil
}

func (sm *StateMachine) applyTransition(state State, fun Transition) error {
	log.Info("applying transition",
		zap.Uint32("id", ID(sm.txDetails.ID)),
		zap.String("old_state", sm.state.String()),
		zap.String("new_state", state.String()),
	)
	newstate, err := fun(sm.txDetails)
	if err != nil {
		log.Error("unable to apply transition",
			zap.Uint32("id", ID(sm.txDetails.ID)),
			zap.String("old_state", sm.state.String()),
			zap.String("new_state", state.String()),
			zap.Error(err),
		)
		// unable to do transition to the new state, return an error
		return err
	}
	// transition succeed, we can apply the new state
	sm.state = state
	// we check if there is transition from the new current state and the next state
	// available
	txtransition := StateTransition{sm.state, newstate}
	t, ok := sm.table.transitions[txtransition]
	if !ok {
		return nil
	}
	// no more transitions to apply
	return sm.applyTransition(newstate, t)
}

func (sm *StateMachine) run() {
	for e := range sm.events {
		sm.onNewEvent(e)
	}
}

func (sm *StateMachine) OnEvent(e *Event) {
	sm.events <- e
}

func NewStateMachine(table *StateTable, txDetails *TxDetails) *StateMachine {
	sm := &StateMachine{
		events:    make(chan *Event, 100),
		state:     StateWaitingForConsensus1,
		table:     table,
		txDetails: txDetails,
	}
	go sm.run()
	return sm
}

func NewTxDetails(txID, raw []byte, tx *Transaction, evidences map[uint64][]byte) *TxDetails {
	return &TxDetails{
		CheckersEvidences: evidences,
		ID:                txID,
		Phase1Decisions:   map[uint64]SBACDecision{},
		Phase2Decisions:   map[uint64]SBACDecision{},
		Raw:               raw,
		Result:            make(chan bool),
		Tx:                tx,
	}
}

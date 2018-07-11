package transactor // import "chainspace.io/prototype/service/transactor"

import "fmt"

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
	ID                []byte
	Raw               []byte
	Result            chan bool
	Tx                *Transaction
}

// Action specify an action to execute when a new event is triggered.
// it returns a State, which will be either the new actual state, the next state
// which may required a transition from the current state to the new one (see the transition
// table
type Action func(tx *TxDetails, event interface{}) (State, error)

// Transition are called when the state is change from a current state to a new one.
// return a State which may involved a new transition as well.
type Transition func(tx *TxDetails) (State, error)

type StateMachine struct {
	txDetails *TxDetails
	events    chan interface{}
	state     State
	table     *StateTable
}

func (sm *StateMachine) Reset() {
	sm.state = StateInitial
}

func (sm *StateMachine) State() State {
	return sm.state
}

func (sm *StateMachine) onNewEvent(event interface{}) error {
	action, ok := sm.table.actions[sm.state]
	if !ok {
		return fmt.Errorf("no action for specified state %v", sm.state)
	}
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
	newstate, err := fun(sm.txDetails)
	if err != nil {
		// unable to do transition to the new state, return an error
		return err
	}
	// transition succeed, we can apply the new state
	sm.state = state
	// we check if there is transition from the new current state and the next state
	// available
	txtransition := StateTransition{sm.state, newstate}
	if t, ok := sm.table.transitions[txtransition]; ok {
		return sm.applyTransition(newstate, t)
	}
	// no more transitions to apply
	return nil
}

func (sm *StateMachine) run() {
	for e := range sm.events {
		sm.onNewEvent(e)
	}
}

func (sm *StateMachine) OnEvent(e interface{}) {
	sm.events <- e
}

func NewStateMachine(table *StateTable, txDetails *TxDetails) *StateMachine {
	sm := &StateMachine{
		events:    make(chan interface{}, 100),
		state:     StateInitial,
		table:     table,
		txDetails: txDetails,
	}
	go sm.run()
	return sm
}

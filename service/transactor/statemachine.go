package transactor // import "chainspace.io/prototype/service/transactor"

import "fmt"

type StateTransition struct {
	From State
	To   State
}

// Action specify an action to execute when a new event is triggered.
// it returns a State, which will be either the new actual state, the next state
// which may required a transition from the current state to the new one (see the transition
// table
type Action func(txID []byte, event interface{}) (State, error)

// Transition are called when the state is change from a current state to a new one.
// return a State which may involved a new transition as well.
type Transition func(txID []byte) (State, error)

type StateMachine struct {
	actionTable     map[State]Action
	currentTxID     []byte
	currentTx       *Transaction
	events          chan interface{}
	state           State
	transitionTable map[StateTransition]Transition
}

func (sm *StateMachine) Reset() {
	sm.state = StateInitial
}

func (sm *StateMachine) State() State {
	return sm.state
}

func (sm *StateMachine) onNewEvent(event interface{}) error {
	action, ok := sm.actionTable[sm.state]
	if !ok {
		return fmt.Errorf("no action for specified state %v", sm.state)
	}
	newstate, err := action(sm.currentTxID, event)
	if err != nil {
		return err
	}
	txtransition := StateTransition{sm.state, newstate}
	// if a transition exist for the new state, apply it
	if t, ok := sm.transitionTable[txtransition]; ok {
		return sm.applyTransition(newstate, t)
	}
	// else save the new state directly
	sm.state = newstate
	return nil
}

func (sm *StateMachine) applyTransition(state State, fun Transition) error {
	newstate, err := fun(sm.currentTxID)
	if err != nil {
		// unable to do transition to the new state, return an error
		return err
	}
	// transition succeed, we can apply the new state
	sm.state = state
	// we check if there is transition from the new current state and the next state
	// available
	txtransition := StateTransition{sm.state, newstate}
	if t, ok := sm.transitionTable[txtransition]; ok {
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

func NewStateMachine(actionTable map[State]Action, transitionTable map[StateTransition]Transition) (*StateMachine, chan interface{}) {
	events := make(chan interface{}, 100)
	sm := &StateMachine{
		actionTable:     actionTable,
		events:          events,
		state:           StateInitial,
		transitionTable: transitionTable,
	}
	go sm.run()
	return sm, events
}

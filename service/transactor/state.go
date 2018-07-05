package transactor // import "chainspace.io/prototype/service/transactor"

import "fmt"

type State uint8

const (
	StateInitial State = iota // Aggregate evidence
	StateObjectLocked
	StateRejectBroadcasted
	StateAcceptBroadcasted
	StateObjectSetInactive
	StateObjectsCreated
	StateSucceeded
	StateCommitBroadcasted
	// on hold waiting for commit decisions message from inter-shards
	StateWaitingForCommitDecisionFromShards
	// on hold waiting for commit decision consensus inside the shard
	StateWaitingForCommitDecisionConsensus
	StateCommitRejected
	StateAborted
)

var (
	// change state are triggered on events
	actionTable = map[State]Action{
		StateInitial:                            onEvidenceReceived,
		StateWaitingForCommitDecisionFromShards: onCommitDecisionFromShardsReceived,
		StateWaitingForCommitDecisionConsensus:  onCommitDecisionConsensusReached,
	}

	// change state are triggererd from the previous state change
	transitionTable = map[StateTransition]Transition{
		{StateInitial, StateObjectLocked}:      toObjectLocked,
		{StateInitial, StateRejectBroadcasted}: toRejectBroadcasted,
		{StateObjectLocked, StateAborted}:      toAborted,
		// object locked with success, but multiple shards involved
		{StateObjectLocked, StateAcceptBroadcasted}: toAcceptBroadcasted,
		// objects locked with success, and only 1 shard involved
		{StateObjectLocked, StateObjectSetInactive}:   toObjectSetInactive,
		{StateObjectSetInactive, StateObjectsCreated}: toObjectsCreated,
		// only one shard involved, object are created in the same shard
		{StateObjectsCreated, StateSucceeded}: toSucceeded,
		// multiple shards, broadcast commit intention
		{StateObjectsCreated, StateCommitBroadcasted}:                     toCommitBroadcasted,
		{StateCommitBroadcasted, StateWaitingForCommitDecisionFromShards}: toWaitingForCommitDecisionFromShards,
		{StateWaitingForCommitDecisionConsensus, StateObjectsCreated}:     toObjectsCreated,
		// unlock objects
		{StateWaitingForCommitDecisionConsensus, StateCommitRejected}: toCommitRejected,
		{StateCommitRejected, StateAborted}:                           toAborted,
	}
)

type StateTransition struct {
	From State
	To   State
}

func onEvidenceReceived(txID []byte, event interface{}) (State, error) {
	// new evidences are received, store / do stuff with them
	// then run the checkers
	// check objects
	var evidenceOK bool
	var notEnoughInfo bool
	if notEnoughInfo {
		return StateInitial, nil
	}
	if evidenceOK {
		return StateObjectLocked, nil
	}
	return StateRejectBroadcasted, nil
}

// can recevied decision from other nodes/shard
// but should also be able to change state in the case a node is starting to run consensus.
func onCommitDecisionFromShardsReceived(txID []byte, event interface{}) (State, error) {
	// if enough responses from shards,
	// move state
	// return StateWaitingForCommitDecisionConsensus, nil

	// else stay in this state
	return StateWaitingForCommitDecisionFromShards, nil
}

// can reached consensus about committing the transaction or rejecting it.
func onCommitDecisionConsensusReached(txID []byte, event interface{}) (State, error) {
	// if shard decide to commit the transaction, move state to create object
	// return StateObjectsCreated, nil

	// else stay in this state
	return StateCommitRejected, nil
}

// can abort, if impossible to lock object
// then check if one shard or more is involved and return StatAcceptBroadcasted
// or StateObjectSetInactive
func toObjectLocked(txID []byte) (State, error) {
	return StateInitial, nil
}

func toRejectBroadcasted(txID []byte) (State, error) {
	return StateInitial, nil
}

func toAcceptBroadcasted(txID []byte) (State, error) {
	return StateInitial, nil
}

func toObjectSetInactive(txID []byte) (State, error) {
	return StateInitial, nil
}

func toObjectsCreated(txID []byte) (State, error) {
	return StateInitial, nil
}

func toSucceeded(txID []byte) (State, error) {
	return StateSucceeded, nil
}

func toCommitBroadcasted(txID []byte) (State, error) {
	return StateInitial, nil
}

func toAborted(txID []byte) (State, error) {
	return StateAborted, nil
}

func toCommitRejected(txID []byte) (State, error) {
	return StateCommitRejected, nil
}

func toWaitingForCommitDecisionFromShards(txID []byte) (State, error) {
	return StateAborted, nil
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
	currentTxID     []byte
	currentTx       *Transaction
	events          chan interface{}
	state           State
	actionTable     map[State]Action
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

func NewStateMachine(table map[State]Action) (*StateMachine, chan interface{}) {
	events := make(chan interface{}, 100)
	sm := &StateMachine{
		state:       StateInitial,
		actionTable: table,
		events:      events,
	}
	go sm.run()
	return sm, events
}

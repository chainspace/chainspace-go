package transactor // import "chainspace.io/prototype/service/transactor"

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

func (s *Service) makeStatesMappings() (map[State]Action, map[StateTransition]Transition) {
	// change state are triggered on events
	actionTable := map[State]Action{
		StateInitial:                            s.onEvidenceReceived,
		StateWaitingForCommitDecisionFromShards: s.onCommitDecisionFromShardsReceived,
		StateWaitingForCommitDecisionConsensus:  s.onCommitDecisionConsensusReached,
	}

	// change state are triggererd from the previous state change
	transitionTable := map[StateTransition]Transition{
		{StateInitial, StateObjectLocked}:      s.toObjectLocked,
		{StateInitial, StateRejectBroadcasted}: s.toRejectBroadcasted,
		{StateObjectLocked, StateAborted}:      s.toAborted,
		// object locked with success, but multiple shards involved
		{StateObjectLocked, StateAcceptBroadcasted}: s.toAcceptBroadcasted,
		// objects locked with success, and only 1 shard involved
		{StateObjectLocked, StateObjectSetInactive}:   s.toObjectSetInactive,
		{StateObjectSetInactive, StateObjectsCreated}: s.toObjectsCreated,
		// only one shard involved, object are created in the same shard
		{StateObjectsCreated, StateSucceeded}: s.toSucceeded,
		// multiple shards, broadcast commit intention
		{StateObjectsCreated, StateCommitBroadcasted}:                     s.toCommitBroadcasted,
		{StateCommitBroadcasted, StateWaitingForCommitDecisionFromShards}: s.toWaitingForCommitDecisionFromShards,
		{StateWaitingForCommitDecisionConsensus, StateObjectsCreated}:     s.toObjectsCreated,
		// unlock objects
		{StateWaitingForCommitDecisionConsensus, StateCommitRejected}: s.toCommitRejected,
		{StateCommitRejected, StateAborted}:                           s.toAborted,
	}

	return actionTable, transitionTable
}

func (s *Service) onEvidenceReceived(txID []byte, event interface{}) (State, error) {
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
func (s *Service) onCommitDecisionFromShardsReceived(txID []byte, event interface{}) (State, error) {
	// if enough responses from shards,
	// move state
	// return StateWaitingForCommitDecisionConsensus, nil

	// else stay in this state
	return StateWaitingForCommitDecisionFromShards, nil
}

// can reached consensus about committing the transaction or rejecting it.
func (s *Service) onCommitDecisionConsensusReached(txID []byte, event interface{}) (State, error) {
	// if shard decide to commit the transaction, move state to create object
	// return StateObjectsCreated, nil

	// else stay in this state
	return StateCommitRejected, nil
}

// can abort, if impossible to lock object
// then check if one shard or more is involved and return StatAcceptBroadcasted
// or StateObjectSetInactive
func (s *Service) toObjectLocked(txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toRejectBroadcasted(txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toAcceptBroadcasted(txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toObjectSetInactive(txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toObjectsCreated(txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toSucceeded(txID []byte) (State, error) {
	return StateSucceeded, nil
}

func (s *Service) toCommitBroadcasted(txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toAborted(txID []byte) (State, error) {
	return StateAborted, nil
}

func (s *Service) toCommitRejected(txID []byte) (State, error) {
	return StateCommitRejected, nil
}

func (s *Service) toWaitingForCommitDecisionFromShards(txID []byte) (State, error) {
	return StateAborted, nil
}

package transactor // import "chainspace.io/prototype/service/transactor"
import "github.com/tav/golly/log"

type State uint8

const (
	StateInitial State = iota // Aggregate evidence
	StateObjectLocked
	StateRejectBroadcasted
	StateAcceptBroadcasted
	StateObjectsDeactivated
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
		{StateObjectLocked, StateObjectsDeactivated}:   s.toObjectDeactivated,
		{StateObjectsDeactivated, StateObjectsCreated}: s.toObjectsCreated,
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

func (s *Service) onEvidenceReceived(tx *Transaction, txID []byte, event interface{}) (State, error) {
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
func (s *Service) onCommitDecisionFromShardsReceived(tx *Transaction, txID []byte, event interface{}) (State, error) {
	// if enough responses from shards,
	// move state
	// return StateWaitingForCommitDecisionConsensus, nil

	// else stay in this state
	return StateWaitingForCommitDecisionFromShards, nil
}

// can reached consensus about committing the transaction or rejecting it.
func (s *Service) onCommitDecisionConsensusReached(tx *Transaction, txID []byte, event interface{}) (State, error) {
	// if shard decide to commit the transaction, move state to create object
	// return StateObjectsCreated, nil

	// else stay in this state
	return StateCommitRejected, nil
}

func (s *Service) inputObjectsForShard(shardID uint64, tx *Transaction) (objects [][]byte, allInShard bool) {
	// get all objects part of this current shard state
	allInShard = true
	for _, t := range tx.Traces {
		for _, o := range t.InputObjectsKeys {
			o := o
			if shardID := s.top.ShardForKey(o); shardID == s.shardID {
				objects = append(objects, o)
				continue
			}
			allInShard = false
		}
	}
	return
}

// can abort, if impossible to lock object
// then check if one shard or more is involved and return StateAcceptBroadcasted
// or StateObjectSetInactive
func (s *Service) toObjectLocked(tx *Transaction, txID []byte) (State, error) {
	objects, allInShard := s.inputObjectsForShard(s.shardID, tx)
	// lock them
	if err := LockObjects(s.store, objects); err != nil {
		log.Errorf("unable to lock all objects: %v", err)
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}
	if allInShard {

		return StateObjectsDeactivated, nil
	}
	return StateAcceptBroadcasted, nil
}

func (s *Service) toRejectBroadcasted(tx *Transaction, txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toAcceptBroadcasted(tx *Transaction, txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toObjectDeactivated(tx *Transaction, txID []byte) (State, error) {
	objects, _ := s.inputObjectsForShard(s.shardID, tx)
	// lock them
	if err := DeactivateObjects(s.store, objects); err != nil {
		log.Errorf("unable to deactivate all objects: %v", err)
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}

	return StateObjectsCreated, nil
}

func (s *Service) toObjectsCreated(tx *Transaction, txID []byte) (State, error) {
	traceIDPairs, err := MakeTraceIDs(tx.Traces)
	if err != nil {
		return StateAborted, err
	}
	traceObjectPairs, err := MakeTraceObjectPairs(traceIDPairs)
	if err != nil {
		return StateAborted, err
	}
	objects := []*Object{}
	allObjectsInCurrentShard := true
	for _, v := range traceObjectPairs {
		v := v
		for _, o := range v.OutputObjects {
			o := o
			if shardID := s.top.ShardForKey(o.Key); shardID == s.shardID {
				objects = append(objects, o)
				continue
			}
			allObjectsInCurrentShard = false
		}
	}
	err = CreateObjects(s.store, objects)
	if err != nil {
		return StateAborted, err
	}
	if allObjectsInCurrentShard {
		return StateSucceeded, nil
	}
	return StateCommitBroadcasted, nil
}

func (s *Service) toSucceeded(tx *Transaction, txID []byte) (State, error) {
	return StateSucceeded, nil
}

func (s *Service) toCommitBroadcasted(tx *Transaction, txID []byte) (State, error) {
	return StateInitial, nil
}

func (s *Service) toAborted(tx *Transaction, txID []byte) (State, error) {
	return StateAborted, nil
}

func (s *Service) toCommitRejected(tx *Transaction, txID []byte) (State, error) {
	return StateCommitRejected, nil
}

func (s *Service) toWaitingForCommitDecisionFromShards(tx *Transaction, txID []byte) (State, error) {
	return StateAborted, nil
}

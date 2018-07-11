package transactor // import "chainspace.io/prototype/service/transactor"
import (
	"hash/fnv"

	"chainspace.io/prototype/log"
)

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

func (s *Service) makeStateTable() *StateTable {
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
		{StateRejectBroadcasted, StateAborted}: s.toAborted,
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

	return &StateTable{actionTable, transitionTable}
}

func (s *Service) objectsExists(objs, refs [][]byte) ([]*Object, bool) {
	keys := [][]byte{}
	for _, v := range append(objs, refs...) {
		if s.top.ShardForKey(v) == s.shardID {
			keys = append(keys, v)
		}
	}

	objects, err := GetObjects(s.store, keys)
	if err != nil {
		return nil, false
	}
	return objects, true
}

func (s *Service) onEvidenceReceived(tx *TxDetails, _ interface{}) (State, error) {
	log.Infof("applying action onEvidenceReceived: %v", ID(tx.ID))
	// TODO: need to check evidences here, as this step should happend after the first consensus round.
	// not sure how we agregate evidence here, if we get new ones from the consensus rounds or the same that where sent with the transaction at the begining.
	if !s.verifySignatures(tx.ID, tx.CheckersEvidences) {
		log.Errorf("missing signatures: %v", ID(tx.ID))
		return StateRejectBroadcasted, nil
	}

	// check that all inputs objects and references part of the state of this node exists.
	for _, trace := range tx.Tx.Traces {
		objects, ok := s.objectsExists(trace.InputObjectsKeys, trace.InputReferencesKeys)
		if !ok {
			log.Errorf("some objects do not exists: %v", ID(tx.ID))
			return StateRejectBroadcasted, nil
		}
		for _, v := range objects {
			if v.Status == ObjectStatus_INACTIVE {
				log.Errorf("some objects are inactive: %v", ID(tx.ID))
				return StateRejectBroadcasted, nil
			}
		}
	}

	return StateObjectLocked, nil
}

// can recevied decision from other nodes/shard
// but should also be able to change state in the case a node is starting to run consensus.
func (s *Service) onCommitDecisionFromShardsReceived(tx *TxDetails, event interface{}) (State, error) {
	// if enough responses from shards,
	// move state
	// return StateWaitingForCommitDecisionConsensus, nil

	// else stay in this state
	return StateWaitingForCommitDecisionFromShards, nil
}

// can reached consensus about committing the transaction or rejecting it.
func (s *Service) onCommitDecisionConsensusReached(tx *TxDetails, event interface{}) (State, error) {
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
func (s *Service) toObjectLocked(tx *TxDetails) (State, error) {
	log.Infof("moving to state ObjectLocked: %v", ID(tx.ID))
	objects, allInShard := s.inputObjectsForShard(s.shardID, tx.Tx)
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

func (s *Service) toRejectBroadcasted(tx *TxDetails) (State, error) {
	log.Infof("moving to state RejectBroadcasted: %v", ID(tx.ID))
	if _, allInShard := s.inputObjectsForShard(s.shardID, tx.Tx); allInShard {
		log.Infof("no other shards involved, exiting now")
		return StateAborted, nil
	}
	return StateAborted, nil
}

func (s *Service) toAcceptBroadcasted(tx *TxDetails) (State, error) {
	return StateInitial, nil
}

func (s *Service) toObjectDeactivated(tx *TxDetails) (State, error) {
	log.Infof("moving to state ObjectDeactivated: %v", ID(tx.ID))
	objects, _ := s.inputObjectsForShard(s.shardID, tx.Tx)
	// lock them
	if err := DeactivateObjects(s.store, objects); err != nil {
		log.Errorf("unable to deactivate all objects: %v", err)
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}

	return StateObjectsCreated, nil
}

func (s *Service) toObjectsCreated(tx *TxDetails) (State, error) {
	log.Infof("moving to state ObjectCreated: %v", ID(tx.ID))
	traceIDPairs, err := MakeTraceIDs(tx.Tx.Traces)
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

func (s *Service) toSucceeded(tx *TxDetails) (State, error) {
	log.Infof("moving to state Succeeded: %v", ID(tx.ID))
	return StateSucceeded, nil
}

func (s *Service) toCommitBroadcasted(tx *TxDetails) (State, error) {
	return StateInitial, nil
}

func (s *Service) toAborted(tx *TxDetails) (State, error) {
	log.Infof("moving to state Aborted: %v", ID(tx.ID))
	return StateAborted, nil
}

func (s *Service) toCommitRejected(tx *TxDetails) (State, error) {
	return StateCommitRejected, nil
}

func (s *Service) toWaitingForCommitDecisionFromShards(tx *TxDetails) (State, error) {
	return StateAborted, nil
}

func ID(data []byte) uint32 {
	h := fnv.New32()
	h.Write(data)
	return h.Sum32()
}

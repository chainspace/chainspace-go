package transactor // import "chainspace.io/prototype/service/transactor"

import (
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/service"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

type State uint8

const (
	StateInitial State = iota // Aggregate evidence
	StateObjectLocked
	StateRejectAcceptTransactionBroadcasted
	StateAcceptAcceptTransactionBroadcasted
	StateWaitingForAcceptTransaction
	StateObjectsDeactivated
	StateObjectsCreated
	StateSucceeded
	StateAcceptCommitBroadcasted
	StateRejectCommitBroadcasted
	// on hold waiting for commit decisions message from inter-shards
	StateWaitingForCommitDecisionFromShards
	// on hold waiting for commit decision consensus inside the shard
	StateWaitingForCommitDecisionConsensus
	StateCommitRejected
	StateAborted
	StateAnyEvent
)

func (s State) String() string {
	switch s {
	case StateInitial:
		return "SateInitial"
	case StateObjectLocked:
		return "StateObjectLocked"
	case StateRejectAcceptTransactionBroadcasted:
		return "StateRejectAcceptTransactionBroadcasted"
	case StateAcceptAcceptTransactionBroadcasted:
		return "StateAcceptAcceptTransactionBroadcasted"
	case StateWaitingForAcceptTransaction:
		return "StateWaitingForAcceptTransaction"
	case StateObjectsDeactivated:
		return "StateObjectsDeactivated"
	case StateObjectsCreated:
		return "StateObjectsCreated"
	case StateSucceeded:
		return "StateSucceeded"
	case StateAcceptCommitBroadcasted:
		return "StateAcceptCommitBroadcasted"
	case StateRejectCommitBroadcasted:
		return "StateRejectCommitBroadcasted"
	case StateWaitingForCommitDecisionFromShards:
		return "StateWaitingForCommitDecisionFromShards"
	case StateWaitingForCommitDecisionConsensus:
		return "StateWaitingForCommitDecisionConsensus"
	case StateCommitRejected:
		return "StateCommitRejected"
	case StateAborted:
		return "StateAborted"
	case StateAnyEvent:
		return "StateAnyEvent"
	default:
		return "error"
	}
}

func (s *Service) makeStateTable() *StateTable {
	// change state are triggered on events
	actionTable := map[State]Action{
		StateInitial:                            s.onEvidenceReceived,
		StateWaitingForAcceptTransaction:        s.onAcceptTransactionReceived,
		StateWaitingForCommitDecisionFromShards: s.onCommitDecisionFromShardsReceived,
		StateWaitingForCommitDecisionConsensus:  s.onCommitDecisionConsensusReached,
		StateAnyEvent:                           s.onAnyEvent,
	}

	// change state are triggererd from the previous state change
	transitionTable := map[StateTransition]Transition{
		{StateInitial, StateObjectLocked}:                       s.toObjectLocked,
		{StateInitial, StateRejectAcceptTransactionBroadcasted}: s.toRejectAcceptTransactionBroadcasted,
		{StateRejectAcceptTransactionBroadcasted, StateAborted}: s.toAborted,

		{StateObjectLocked, StateAborted}: s.toAborted,
		// object locked with success, but multiple shards involved
		{StateObjectLocked, StateAcceptAcceptTransactionBroadcasted}:                s.toAcceptAcceptTransactionBroadcasted,
		{StateAcceptAcceptTransactionBroadcasted, StateWaitingForAcceptTransaction}: s.toWaitingForAcceptTransaction,
		{StateWaitingForAcceptTransaction, StateAborted}:                            s.toAborted,
		{StateWaitingForAcceptTransaction, StateSucceeded}:                          s.toSucceeded,
		// objects locked with success, and only 1 shard involved
		{StateObjectLocked, StateObjectsDeactivated}:   s.toObjectDeactivated,
		{StateObjectsDeactivated, StateObjectsCreated}: s.toObjectsCreated,
		// only one shard involved, object are created in the same shard
		{StateObjectsCreated, StateSucceeded}: s.toSucceeded,
		// multiple shards, broadcast commit intention
		{StateObjectsCreated, StateAcceptCommitBroadcasted}:                     s.toCommitBroadcasted,
		{StateAcceptCommitBroadcasted, StateWaitingForCommitDecisionFromShards}: s.toWaitingForCommitDecisionFromShards,
		{StateWaitingForCommitDecisionConsensus, StateObjectsCreated}:           s.toObjectsCreated,
		// unlock objects
		{StateWaitingForCommitDecisionConsensus, StateCommitRejected}: s.toCommitRejected,
		{StateCommitRejected, StateAborted}:                           s.toAborted,
	}

	return &StateTable{actionTable, transitionTable}
}

func (s *Service) onAnyEvent(tx *TxDetails, event *Event) (State, error) {
	if string(tx.ID) != string(event.msg.TransactionID) {
		log.Error("invalid transaction sent to state machine", zap.Uint32("expected", ID(tx.ID)), zap.Uint32("got", ID(event.msg.TransactionID)))
		return StateAnyEvent, errors.New("invalid transaction ID")
	}
	switch event.msg.Op {
	case SBACOpcode_ACCEPT_TRANSACTION:
		log.Info("ACCEPT_TRANSACTION received", zap.Uint32("id", ID(tx.ID)), zap.String("decision", SBACDecision_name[int32(event.msg.Decision)]), zap.Uint64("peer.id", event.peerID))
		tx.AcceptTransaction[event.peerID] = event.msg.Decision
	case SBACOpcode_COMMIT_TRANSACTION:
		log.Info("COMMIT_TRANSACTION received", zap.Uint32("id", ID(tx.ID)), zap.String("decision", SBACDecision_name[int32(event.msg.Decision)]), zap.Uint64("peer.id", event.peerID))
		tx.AcceptTransaction[event.peerID] = event.msg.Decision
	case SBACOpcode_NEW_TRANSACTION:
		log.Info("NEW_TRANSACTION received", zap.Uint32("id", ID(tx.ID)), zap.String("decision", SBACDecision_name[int32(event.msg.Decision)]), zap.Uint64("peer.id", event.peerID))
	default:
	}
	return StateAnyEvent, nil
}

// shardsInvolvedInTx return a list of IDs of all shards involved in the transaction either by
// holding state for an input object or input reference.
func (s *Service) shardsInvolvedInTx(tx *Transaction) []uint64 {
	uniqids := map[uint64]struct{}{}
	for _, trace := range tx.Traces {
		for _, obj := range trace.InputObjectsKeys {
			uniqids[s.top.ShardForKey(obj)] = struct{}{}
		}
		for _, ref := range trace.InputReferencesKeys {
			uniqids[s.top.ShardForKey(ref)] = struct{}{}
		}
	}
	ids := make([]uint64, 0, len(uniqids))
	for k, _ := range uniqids {
		ids = append(ids, k)
	}
	return ids
}

func twotplusone(shardSize uint64) uint64 {
	return (2*(shardSize/3) + 1)
}

func tplusone(shardSize uint64) uint64 {
	return shardSize/3 + 1
}

func (s *Service) onAcceptTransactionReceived(tx *TxDetails, event *Event) (State, error) {
	s.onAnyEvent(tx, event)
	shards := s.shardsInvolvedInTx(tx.Tx)
	var somePending bool
	// for each shards, get the nodes id, and checks if they answered
	for _, v := range shards {
		nodes := s.top.NodesInShard(v)
		var accepted uint64
		var rejected uint64
		for _, nodeID := range nodes {
			if d, ok := tx.AcceptTransaction[nodeID]; ok {
				if d == SBACDecision_ACCEPT {
					accepted += 1
					continue
				}
				rejected += 1
			}
		}
		if rejected >= tplusone(s.shardSize) {
			log.Info("transaction rejected", zap.Uint32("id", ID(tx.ID)))
			return StateAborted, nil
		}
		if accepted >= twotplusone(s.shardSize) {
			log.Info("transaction accepted", zap.Uint32("id", ID(tx.ID)))
			continue
		}
		somePending = true
	}

	if somePending {
		log.Info("transaction pending, not enough answers from shards", zap.Uint32("id", ID(tx.ID)))
		return StateWaitingForAcceptTransaction, nil
	}

	log.Info("transaction accepted by all shards", zap.Uint32("id", ID(tx.ID)))
	return StateSucceeded, nil
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

func (s *Service) onEvidenceReceived(tx *TxDetails, _ *Event) (State, error) {
	// TODO: need to check evidences here, as this step should happend after the first consensus round.
	// not sure how we agregate evidence here, if we get new ones from the consensus rounds or the same that where sent with the transaction at the begining.
	if !s.verifySignatures(tx.ID, tx.CheckersEvidences) {
		log.Error("missing/invalid signatures", zap.Uint32("id", ID(tx.ID)))
		return StateRejectAcceptTransactionBroadcasted, nil
	}

	// check that all inputs objects and references part of the state of this node exists.
	for _, trace := range tx.Tx.Traces {
		objects, ok := s.objectsExists(trace.InputObjectsKeys, trace.InputReferencesKeys)
		if !ok {
			log.Error("some objects do not exists", zap.Uint32("id", ID(tx.ID)))
			return StateRejectAcceptTransactionBroadcasted, nil
		}
		for _, v := range objects {
			if v.Status == ObjectStatus_INACTIVE {
				log.Error("some objects are inactive", zap.Uint32("id", ID(tx.ID)))
				return StateRejectAcceptTransactionBroadcasted, nil
			}
		}
	}

	log.Info("evidences and input objects/references checked successfully", zap.Uint32("id", ID(tx.ID)))
	return StateObjectLocked, nil
}

// can recevied decision from other nodes/shard
// but should also be able to change state in the case a node is starting to run consensus.
func (s *Service) onCommitDecisionFromShardsReceived(tx *TxDetails, event *Event) (State, error) {
	// if enough responses from shards,
	// move state
	// return StateWaitingForCommitDecisionConsensus, nil

	// else stay in this state
	return StateWaitingForCommitDecisionFromShards, nil
}

// can reached consensus about committing the transaction or rejecting it.
func (s *Service) onCommitDecisionConsensusReached(tx *TxDetails, event *Event) (State, error) {
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
		for _, ref := range t.InputReferencesKeys {
			ref := ref
			if shardID := s.top.ShardForKey(ref); shardID == s.shardID {
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
	objects, allInShard := s.inputObjectsForShard(s.shardID, tx.Tx)
	// lock them
	if err := LockObjects(s.store, objects); err != nil {
		log.Error("unable to lock all objects", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}
	if allInShard {
		return StateObjectsDeactivated, nil
	}
	return StateAcceptAcceptTransactionBroadcasted, nil
}

func makeMessage(m *SBACMessage) (*service.Message, error) {
	payload, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  uint32(Opcode_SBAC),
		Payload: payload,
	}, nil
}

func (s *Service) sendToAllShardInvolved(tx *TxDetails, msg *service.Message) error {
	shards := s.shardsInvolvedInTx(tx.Tx)
	for _, shard := range shards {
		nodes := s.top.NodesInShard(shard)
		for _, node := range nodes {
			// TODO: proper timeout ?
			_, err := s.conns.WriteRequest(node, msg, time.Second)
			if err != nil {
				log.Error("unable to connect to node", zap.Uint32("id", ID(tx.ID)), zap.Uint64("peer.id", node))
				return fmt.Errorf("unable to connect to node(%v): %v", node, err)
			}
		}
	}
	return nil
}

func (s *Service) toRejectAcceptTransactionBroadcasted(tx *TxDetails) (State, error) {
	sbacmsg := &SBACMessage{
		Op:            SBACOpcode_ACCEPT_TRANSACTION,
		Decision:      SBACDecision_REJECT,
		TransactionID: tx.ID,
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("unable to serialize message reject accept transaction", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
		return StateAborted, err
	}

	err = s.sendToAllShardInvolved(tx, msg)
	if err != nil {
		log.Error("unable to sent reject transaction to all shards", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
	}

	log.Info("reject transaction sent to all shards", zap.Uint32("id", ID(tx.ID)))
	return StateAborted, err
}

func (s *Service) toAcceptAcceptTransactionBroadcasted(tx *TxDetails) (State, error) {
	sbacmsg := &SBACMessage{
		Op:            SBACOpcode_ACCEPT_TRANSACTION,
		Decision:      SBACDecision_ACCEPT,
		TransactionID: tx.ID,
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("unable to serialize message accept transaction accept", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
		return StateAborted, err
	}

	err = s.sendToAllShardInvolved(tx, msg)
	if err != nil {
		log.Error("unable to sent accept transaction to all shards", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
		return StateAborted, err
	}

	log.Info("accept transaction sent to all shards", zap.Uint32("id", ID(tx.ID)))
	return StateWaitingForAcceptTransaction, nil
}

func (s *Service) toWaitingForAcceptTransaction(tx *TxDetails) (State, error) {
	return StateWaitingForAcceptTransaction, nil
}

func (s *Service) toObjectDeactivated(tx *TxDetails) (State, error) {
	objects, _ := s.inputObjectsForShard(s.shardID, tx.Tx)
	// lock them
	if err := DeactivateObjects(s.store, objects); err != nil {
		log.Error("unable to deactivate all objects", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}

	log.Info("all object deactivated successfully", zap.Uint32("id", ID(tx.ID)))
	return StateObjectsCreated, nil
}

func (s *Service) toObjectsCreated(tx *TxDetails) (State, error) {
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
		log.Error("unable to create objects", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
		return StateAborted, err
	}
	log.Info("all objects created successfully", zap.Uint32("id", ID(tx.ID)))
	if allObjectsInCurrentShard {
		return StateSucceeded, nil
	}
	return StateAcceptCommitBroadcasted, nil
}

func (s *Service) toSucceeded(tx *TxDetails) (State, error) {
	tx.Result <- true
	return StateSucceeded, nil
}

func (s *Service) toAborted(tx *TxDetails) (State, error) {
	// unlock any objects maybe related to this transaction.
	objects, _ := s.inputObjectsForShard(s.shardID, tx.Tx)
	if err := UnlockObjects(s.store, objects); err != nil {
		log.Error("unable to unlock objects", zap.Uint32("id", ID(tx.ID)), zap.Error(err))
	}
	tx.Result <- false
	return StateAborted, nil
}

func (s *Service) toCommitBroadcasted(tx *TxDetails) (State, error) {
	return StateInitial, nil
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

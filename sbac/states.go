package sbac // import "chainspace.io/prototype/sbac"

import (
	"fmt"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
)

type State uint8

const (
	StateWaitingForConsensus1 State = iota // waiting for the first consensus to be reached, in order to process the transaction inside the shard.

	StateObjectLocked // check if objects exists and are actives

	StateAcceptPhase1Broadcasted // 2-phase commit phase 1
	StateRejectPhase1Broadcasted
	StateWaitingForPhase1

	StateConsensus2Triggered
	StateWaitingForConsensus2 // second consensus inside the shard, in order to confirm accept of the transaction after the first phase of 2-phase commit. also should kick bad nodes in the future

	StateAcceptPhase2Broadcasted // 2-phase commit phase 2
	StateRejectPhase2Broadcasted
	StateWaitingForPhase2

	StateObjectsDeactivated
	StateObjectsCreated

	StateCommitObjectsBroadcasted // notify other shards to create the objects
	StateWaitingForCommit         // node which need to create object start it state machine with this
	StateConsensusCommitTriggered
	StateWaitingForConsensusCommit

	StateAborted
	StateSucceeded
)

func (s State) String() string {
	switch s {
	case StateWaitingForConsensus1:
		return "StateWaitingOnConsensus1"
	case StateObjectLocked:
		return "StateObjectLocked"
	case StateRejectPhase1Broadcasted:
		return "StateRejectPhase1Broadcasted"
	case StateAcceptPhase1Broadcasted:
		return "StateAcceptPhase1Broadcasted"
	case StateWaitingForPhase1:
		return "StateWaitingForPhase1"
	case StateConsensus2Triggered:
		return "StateConsensus2Triggered"
	case StateWaitingForConsensus2:
		return "StateWaitingForConsensus2"
	case StateAcceptPhase2Broadcasted:
		return "StateAcceptPhase2Broadcasted"
	case StateRejectPhase2Broadcasted:
		return "StateRejectPhase2Broadcasted"
	case StateWaitingForPhase2:
		return "StateWaitingForPhase2"
	case StateObjectsDeactivated:
		return "StateObjectsDeactivated"
	case StateObjectsCreated:
		return "StateObjectsCreated"
	case StateSucceeded:
		return "StateSucceeded"
	case StateCommitObjectsBroadcasted:
		return "StateCommitObjectsBroadcasted"
	case StateConsensusCommitTriggered:
		return "StateConsensusCommitTriggered"
	case StateWaitingForCommit:
		return "StateWaitingForCommit"
	case StateWaitingForConsensusCommit:
		return "StateWaitingForConsensusCommit"
	case StateAborted:
		return "StateAborted"
	default:
		return "error"
	}
}

func (s *Service) makeStateTable() *StateTable {
	// change state are triggered on events
	actionTable := map[State]Action{
		StateWaitingForConsensus1:      s.onWaitingForConsensus1,
		StateWaitingForConsensus2:      s.onWaitingForConsensus2,
		StateWaitingForPhase1:          s.onWaitingForPhase1,
		StateWaitingForPhase2:          s.onWaitingForPhase2,
		StateWaitingForCommit:          s.onWaitingForCommit,
		StateWaitingForConsensusCommit: s.onWaitingForConsensusCommit,
	}

	// change state are triggererd from the previous state change
	transitionTable := map[StateTransition]Transition{
		// first bad path, no initial censensu reach / missing evidence, whatever
		{StateWaitingForConsensus1, StateRejectPhase1Broadcasted}: s.toRejectPhase1Broadcasted,
		{StateRejectPhase1Broadcasted, StateAborted}:              s.toAborted,
		{StateObjectLocked, StateAborted}:                         s.toAborted,

		// objects locked with success, and only 1 shard involved
		{StateObjectLocked, StateObjectsDeactivated}:   s.toObjectDeactivated,
		{StateObjectsDeactivated, StateObjectsCreated}: s.toObjectsCreated,
		{StateObjectsCreated, StateSucceeded}:          s.toSucceeded,

		// object locked with success, but multiple shards involved
		{StateWaitingForConsensus1, StateObjectLocked}:        s.toObjectLocked,
		{StateObjectLocked, StateAcceptPhase1Broadcasted}:     s.toAcceptPhase1Broadcasted,
		{StateAcceptPhase1Broadcasted, StateWaitingForPhase1}: s.toWaitingForPhase1,
		{StateWaitingForPhase1, StateAborted}:                 s.toAborted,
		{StateWaitingForPhase1, StateConsensus2Triggered}:     s.toConsensus2Triggered,
		// consensus 2 / phase 2
		{StateConsensus2Triggered, StateWaitingForConsensus2}:     s.toWaitingForConsensus2,
		{StateWaitingForConsensus2, StateAcceptPhase2Broadcasted}: s.toAcceptPhase2Broadcasted,
		{StateWaitingForConsensus2, StateRejectPhase2Broadcasted}: s.toRejectPhase2Broadcasted,
		{StateRejectPhase2Broadcasted, StateAborted}:              s.toAborted,
		{StateAcceptPhase2Broadcasted, StateWaitingForPhase2}:     s.toWaitingForPhase2,
		{StateWaitingForPhase2, StateAborted}:                     s.toAborted,
		{StateWaitingForPhase2, StateObjectsDeactivated}:          s.toObjectDeactivated,

		{StateObjectsCreated, StateCommitObjectsBroadcasted}: s.toCommitObjectsBroadcasted,
		{StateCommitObjectsBroadcasted, StateAborted}:        s.toAborted,
		{StateCommitObjectsBroadcasted, StateSucceeded}:      s.toSucceeded,

		{StateWaitingForCommit, StateConsensusCommitTriggered}:          s.toConsensusCommitTriggered,
		{StateWaitingForCommit, StateAborted}:                           s.toAborted,
		{StateConsensusCommitTriggered, StateWaitingForConsensusCommit}: s.toWaitingForConsensusCommit,
		{StateWaitingForConsensusCommit, StateObjectsCreated}:           s.toObjectsCreated,
		{StateWaitingForConsensusCommit, StateAborted}:                  s.toAborted,
	}

	return &StateTable{actionTable, transitionTable}
}

func (s *Service) onWaitingForPhase1(st *States) (State, error) {
	switch st.sbac[SBACOp_Phase1].State() {
	case StateSBACRejected:
		return StateAborted, nil
	case StateSBACWaiting:
		return StateWaitingForPhase1, nil
	default:
		return StateConsensus2Triggered, nil
	}
}

func (s *Service) onWaitingForPhase2(st *States) (State, error) {
	switch st.sbac[SBACOp_Phase2].State() {
	case StateSBACRejected:
		return StateAborted, nil
	case StateSBACWaiting:
		return StateWaitingForPhase2, nil
	default:
		return StateObjectsDeactivated, nil
	}
}

func (s *Service) onWaitingForCommit(st *States) (State, error) {
	switch st.sbac[SBACOp_Commit].State() {
	case StateSBACRejected:
		return StateAborted, nil
	case StateSBACWaiting:
		return StateWaitingForCommit, nil
	default:
		return StateConsensusCommitTriggered, nil
	}
}

func (s *Service) onWaitingForConsensus1(st *States) (State, error) {
	state := st.consensus[ConsensusOp_Consensus1].State()
	if state == StateConsensusWaiting {
		return StateWaitingForConsensus1, nil
	} else if state == StateConsensusRejected {
		return StateRejectPhase1Broadcasted, nil
	}

	// check that all inputs objects and references part of the state of this node exists.
	for _, trace := range st.detail.Tx.Traces {
		objects, ok := s.objectsExists(
			trace.InputObjectVersionIDs, trace.InputReferenceVersionIDs)
		if !ok {
			log.Error("consensus1 some objects do not exists", fld.TxID(st.detail.HashID))
			return StateRejectPhase1Broadcasted, nil
		}
		for _, v := range objects {
			if v.Status == ObjectStatus_INACTIVE {
				log.Error("consensus1 some objects are inactive", fld.TxID(st.detail.HashID))
				return StateRejectPhase1Broadcasted, nil
			}
		}
	}

	if log.AtDebug() {
		log.Debug("consensus1 evidences and input objects/references checked successfully", fld.TxID(st.detail.HashID))
	}

	return StateObjectLocked, nil
}

// TODO(): kick bad node here ? not sure which one are bad
func (s *Service) onWaitingForConsensus2(st *States) (State, error) {
	state := st.consensus[ConsensusOp_Consensus2].State()
	if state == StateConsensusWaiting {
		return StateWaitingForConsensus2, nil
	} else if state == StateConsensusRejected {
		return StateRejectPhase2Broadcasted, nil
	}
	return StateAcceptPhase2Broadcasted, nil
}

// TODO(): verify signatures here, but need to be passed to first.
func (s *Service) onWaitingForConsensusCommit(st *States) (State, error) {
	state := st.consensus[ConsensusOp_ConsensusCommit].State()
	if state == StateConsensusWaiting {
		return StateWaitingForConsensusCommit, nil
	} else if state == StateConsensusRejected {
		return StateAborted, nil
	}
	return StateObjectsCreated, nil
}

// can abort, if impossible to lock object
// then check if one shard or more is involved and return StateAcceptBroadcasted
// or StateObjectSetInactive
func (s *Service) toObjectLocked(st *States) (State, error) {
	objects, allInShard := s.inputObjectsForShard(s.shardID, st.detail.Tx)
	// lock them
	if err := s.store.LockObjects(objects); err != nil {
		log.Error("unable to lock all objects", fld.TxID(st.detail.HashID), fld.Err(err))
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}
	if allInShard {
		return StateObjectsDeactivated, nil
	}
	//log.Error("OBJECTS IN MULTIPLE SHARDS")
	return StateAcceptPhase1Broadcasted, nil
}

func (s *Service) signSBACMessage(
	sbacphase SBACOp, decision SBACDecision, st *States) (*service.Message, error) {
	signature, err := s.signTransaction(st.detail.Tx)
	if err != nil {
		log.Error("unable to sign transaction",
			fld.Err(err), log.String("sbac.phase", sbacphase.String()))
		return nil, err
	}

	sbacmsg := &SBACMessage{
		Op:        sbacphase,
		Decision:  decision,
		TxID:      st.detail.ID,
		Tx:        st.detail.Tx,
		Evidences: st.detail.Evidences,
		Signature: signature,
		PeerID:    s.nodeID,
	}
	payload, err := proto.Marshal(sbacmsg)
	if err != nil {
		log.Error("unable to marshal message reject accept transaction",
			fld.TxID(st.detail.HashID), fld.Err(err),
			log.String("sbac.phase", sbacphase.String()))
		return nil, err
	}
	return &service.Message{Opcode: int32(Opcode_SBAC), Payload: payload}, nil
}

func (s *Service) toRejectPhase1Broadcasted(st *States) (State, error) {
	msg, err := s.signSBACMessage(SBACOp_Phase1, SBACDecision_REJECT, st)
	if err != nil {
		return StateAborted, nil
	}

	err = s.sendToAllShardInvolved(st, msg)
	if err != nil {
		log.Error("unable to sent reject transaction to all shards",
			fld.TxID(st.detail.HashID), fld.Err(err))
	}

	if log.AtDebug() {
		log.Debug("reject transaction sent to all shards", fld.TxID(st.detail.HashID))
	}
	return StateAborted, err
}

func (s *Service) toAcceptPhase1Broadcasted(st *States) (State, error) {
	msg, err := s.signSBACMessage(SBACOp_Phase1, SBACDecision_ACCEPT, st)
	if err != nil {
		return StateAborted, nil
	}

	err = s.sendToAllShardInvolved(st, msg)
	if err != nil {
		log.Error("unable to sent accept transaction to all shards", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
	}

	if log.AtDebug() {
		log.Debug("accept transaction sent to all shards", fld.TxID(st.detail.HashID))
	}
	return StateWaitingForPhase1, nil
}

func (s *Service) toRejectPhase2Broadcasted(st *States) (State, error) {
	msg, err := s.signSBACMessage(SBACOp_Phase2, SBACDecision_REJECT, st)
	if err != nil {
		return StateAborted, nil
	}

	err = s.sendToAllShardInvolved(st, msg)
	if err != nil {
		log.Error("phase2 reject unable to sent reject transaction to all shards", fld.TxID(st.detail.HashID), fld.Err(err))
	}

	if log.AtDebug() {
		log.Debug("phase2 reject transaction sent to all shards", fld.TxID(st.detail.HashID))
	}
	return StateAborted, err
}

func (s *Service) toAcceptPhase2Broadcasted(st *States) (State, error) {
	msg, err := s.signSBACMessage(SBACOp_Phase2, SBACDecision_ACCEPT, st)
	if err != nil {
		return StateAborted, nil
	}

	err = s.sendToAllShardInvolved(st, msg)
	if err != nil {
		log.Error("phase2 unable to sent accept transaction to all shards", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
	}

	if log.AtDebug() {
		log.Debug("phase2 accept transaction sent to all shards", fld.TxID(st.detail.HashID))
	}
	return StateWaitingForPhase2, nil
}

func (s *Service) toCommitObjectsBroadcasted(st *States) (State, error) {
	msg, err := s.signSBACMessage(SBACOp_Commit, SBACDecision_ACCEPT, st)
	if err != nil {
		return StateAborted, nil
	}

	// send to all shards which will hold state for new object, and was not involved before
	shards := s.shardsInvolved(st.detail.Tx)
	shardsUniq := touniqids(shards)
	ids, err := MakeIDs(st.detail.Tx)
	if err != nil {
		return StateAborted, err
	}
	shardsInvolvedUniq := map[uint64]struct{}{}
	for _, v := range ids.TraceObjectPairs {
		v := v
		for _, o := range v.OutputObjects {
			shard := s.top.ShardForVersionID(o.VersionID)
			if _, ok := shardsUniq[shard]; !ok {
				shardsInvolvedUniq[shard] = struct{}{}
			}

		}
	}

	shardsInvolved := fromuniqids(shardsInvolvedUniq)
	if len(shardsInvolved) > 0 {
		err = s.sendToShards(shardsInvolved, st, msg)
		if err != nil {
			log.Error("commit unable to sent accept transaction to all shards", fld.TxID(st.detail.HashID), fld.Err(err))
			return StateAborted, err
		}
	}

	if log.AtDebug() {
		log.Debug("commit accept transaction sent to all shards",
			fld.TxID(st.detail.HashID), log.Uint64s("shards_involved", shardsInvolved))
	}
	return StateSucceeded, nil
}

func (s *Service) toWaitingForPhase1(st *States) (State, error) {
	return StateWaitingForPhase1, nil
}

func (s *Service) toWaitingForPhase2(st *States) (State, error) {
	return StateWaitingForPhase2, nil
}

func (s *Service) toObjectDeactivated(st *States) (State, error) {
	objects, _ := s.inputObjectsForShard(s.shardID, st.detail.Tx)
	// lock them
	if err := s.store.DeactivateObjects(objects); err != nil {
		log.Error("unable to deactivate all objects", fld.TxID(st.detail.HashID), fld.Err(err))
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}

	if log.AtDebug() {
		log.Debug("all object deactivated successfully", fld.TxID(st.detail.HashID))
	}
	return StateObjectsCreated, nil
}

func (s *Service) toObjectsCreated(st *States) (State, error) {
	traceIDPairs, err := MakeTraceIDs(st.detail.Tx.Traces)
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
			if shardID := s.top.ShardForVersionID(o.VersionID); shardID == s.shardID {
				objects = append(objects, o)
				continue
			}
			allObjectsInCurrentShard = false
		}
	}
	err = s.store.CreateObjects(objects)
	if err != nil {
		log.Error("unable to create objects", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
	}
	if log.AtDebug() {
		log.Debug("all objects created successfully", fld.TxID(st.detail.HashID))
	}
	if allObjectsInCurrentShard {
		return StateSucceeded, nil
	}
	return StateCommitObjectsBroadcasted, nil
}

func (s *Service) toSucceeded(st *States) (State, error) {
	// st.detail.Result <- true
	log.Error("finishing transaction", fld.TxID(st.detail.HashID))
	err := s.store.FinishTransaction(st.detail.ID)
	if err != nil {
		log.Error("unable to finish transaction", fld.TxID(st.detail.HashID), fld.Err(err))
	}

	ids, err := MakeIDs(st.detail.Tx)
	if err != nil {
		log.Error("unable to make ids", fld.Err(err))
		return StateSucceeded, nil
	}

	s.saveLabels(ids)
	s.publishObjects(ids, true)

	return StateSucceeded, nil
}

func (s *Service) toAborted(st *States) (State, error) {
	// unlock any objects maybe related to this transaction.
	objects, _ := s.inputObjectsForShard(s.shardID, st.detail.Tx)
	err := s.store.UnlockObjects(objects)
	if err != nil {
		log.Error("unable to unlock objects", fld.TxID(st.detail.HashID), fld.Err(err))
	}
	log.Error("finishing transaction", fld.TxID(st.detail.HashID))

	err = s.store.FinishTransaction(st.detail.ID)
	if err != nil {
		log.Error("unable to finish transaction", fld.TxID(st.detail.HashID), fld.Err(err))
	}

	ids, err := MakeIDs(st.detail.Tx)
	if err != nil {
		log.Error("unable to make ids", fld.Err(err))
		return StateSucceeded, nil
	}
	s.publishObjects(ids, false)

	return StateAborted, nil
}

// TODO(): should we make our own evidences ourselves here ?
func (s *Service) toConsensus2Triggered(st *States) (State, error) {
	if s.isNodeInitiatingBroadcast(st.detail.HashID) {
		// broadcast transaction
		consensusTx := &ConsensusTransaction{
			Tx:        st.detail.Tx,
			TxID:      st.detail.ID,
			Evidences: st.detail.Evidences,
			Op:        ConsensusOp_Consensus2,
			Initiator: s.nodeID,
		}
		b, err := proto.Marshal(consensusTx)
		if err != nil {
			return StateAborted, fmt.Errorf(
				"sbac: unable to marshal consensus tx: %v", err)
		}

		s.broadcaster.AddTransaction(b, 0)
	}
	return StateWaitingForConsensus2, nil
}

func (s *Service) toConsensusCommitTriggered(st *States) (State, error) {
	if s.isNodeInitiatingBroadcast(st.detail.HashID) {
		// broadcast transaction
		consensusTx := &ConsensusTransaction{
			Tx:        st.detail.Tx,
			TxID:      st.detail.ID,
			Evidences: st.detail.Evidences,
			Op:        ConsensusOp_ConsensusCommit,
			Initiator: s.nodeID,
		}
		b, err := proto.Marshal(consensusTx)
		if err != nil {
			return StateAborted, fmt.Errorf(
				"sbac: unable to marshal consensus tx: %v", err)
		}

		s.broadcaster.AddTransaction(b, 0)
	}

	return StateWaitingForConsensusCommit, nil
}

func (s *Service) toWaitingForConsensusCommit(st *States) (State, error) {
	return StateWaitingForConsensusCommit, nil
}

func (s *Service) toWaitingForConsensus2(st *States) (State, error) {
	return StateWaitingForConsensus2, nil
}

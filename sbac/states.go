package sbac // import "chainspace.io/prototype/sbac"

import (
	"fmt"
	"hash/fnv"
	"time"

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

func (s *Service) shardsInvolvedWithoutSelf(tx *Transaction) []uint64 {
	shards := s.shardsInvolvedInTx(tx)
	for i, _ := range shards {
		if shards[i] == s.shardID {
			shards = append(shards[:i], shards[i+1:]...)
			break
		}
	}
	return shards
}

// shardsInvolvedInTx return a list of IDs of all shards involved in the transaction either by
// holding state for an input object or input reference.
func (s *Service) shardsInvolvedInTx(tx *Transaction) []uint64 {
	uniqids := map[uint64]struct{}{}
	for _, trace := range tx.Traces {
		for _, obj := range trace.InputObjectVersionIDs {
			uniqids[s.top.ShardForVersionID(obj)] = struct{}{}
		}
		for _, ref := range trace.InputReferenceVersionIDs {
			uniqids[s.top.ShardForVersionID(ref)] = struct{}{}
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

type WaitingDecisionResult uint8

const (
	WaitingDecisionResultAbort WaitingDecisionResult = iota
	WaitingDecisionResultPending
	WaitingDecisionResultAccept
)

func (s *Service) verifySignature(tx *Transaction, signature []byte, nodeID uint64) (bool, error) {
	b, err := proto.Marshal(tx)
	if err != nil {
		return false, err
	}
	keys := s.top.SeedPublicKeys()
	key := keys[nodeID]
	return key.Verify(b, signature), nil
}

/*
func (s *Service) onWaitingFor(st *States, decisions map[uint64]SignedDecision, phaseName string) WaitingDecisionResult {
	shards := s.shardsInvolvedWithoutSelf(st.detail.Tx)
	var somePending bool
	// for each shards, get the nodes id, and checks if they answered
	vtwotplusone := twotplusone(s.shardSize)
	vtplusone := tplusone(s.shardSize)
	for _, v := range shards {
		nodes := s.top.NodesInShard(v)
		var accepted uint64
		var rejected uint64
		for _, nodeID := range nodes {
			if d, ok := decisions[nodeID]; ok {
				if d.Decision == SBACDecision_ACCEPT {
					accepted += 1
					continue
				}
				rejected += 1
			}
		}
		if rejected >= vtplusone {
			if log.AtDebug() {
				log.Debug(phaseName+" transaction rejected",
					fld.TxID(st.detail.HashID),
					fld.PeerShard(v),
					log.Uint64("t+1", vtplusone),
					log.Uint64("rejected", rejected),
				)
			}
			return WaitingDecisionResultAbort
		}
		if accepted >= vtwotplusone {
			if log.AtDebug() {
				log.Debug(phaseName+" transaction accepted",
					fld.TxID(st.detail.HashID),
					fld.PeerShard(v),
					log.Uint64s("shards_involved", shards),
					log.Uint64("2t+1", vtwotplusone),
					log.Uint64("accepted", accepted),
				)
			}
			continue
		}
		somePending = true
	}

	if somePending {
		if log.AtDebug() {
			log.Debug(phaseName+" transaction pending, not enough answers from shards", fld.TxID(st.detail.HashID))
		}
		return WaitingDecisionResultPending
	}

	if log.AtDebug() {
		log.Debug(phaseName+" transaction accepted by all shards", fld.TxID(st.detail.HashID))
	}

	// verify signatures now
	for k, v := range decisions {
		// TODO(): what to do with nodes with invalid signature
		ok, err := s.verifySignature(st.detail.Tx, v.Signature, k)
		if err != nil {
			log.Error(phaseName+"unable to verify signature", fld.TxID(st.detail.HashID), fld.Err(err))
		}
		if !ok {
			log.Error(phaseName+" invalid signature for a decision", fld.TxID(st.detail.HashID), fld.PeerID(k))
		}
	}

	return WaitingDecisionResultAccept
}
*/

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

func (s *Service) objectsExists(vids, refvids [][]byte) ([]*Object, bool) {
	ownvids := [][]byte{}
	for _, v := range append(vids, refvids...) {
		if s.top.ShardForVersionID(v) == s.shardID {
			ownvids = append(ownvids, v)
		}
	}

	objects, err := GetObjects(s.store, ownvids)
	if err != nil {
		return nil, false
	}
	return objects, true
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
	state := st.consensus[ConsensusOp_Consensus2].State()
	if state == StateConsensusWaiting {
		return StateWaitingForConsensusCommit, nil
	} else if state == StateConsensusRejected {
		return StateAborted, nil
	}
	return StateObjectsCreated, nil
}

func (s *Service) inputObjectsForShard(shardID uint64, tx *Transaction) (objects [][]byte, allInShard bool) {
	// get all objects part of this current shard state
	allInShard = true
	for _, t := range tx.Traces {
		for _, o := range t.InputObjectVersionIDs {
			o := o
			if shardID := s.top.ShardForVersionID(o); shardID == s.shardID {
				objects = append(objects, o)
				continue
			}
			allInShard = false
		}
		for _, ref := range t.InputReferenceVersionIDs {
			ref := ref
			if shardID := s.top.ShardForVersionID(ref); shardID == s.shardID {
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
func (s *Service) toObjectLocked(st *States) (State, error) {
	objects, allInShard := s.inputObjectsForShard(s.shardID, st.detail.Tx)
	// lock them
	if err := LockObjects(s.store, objects); err != nil {
		log.Error("unable to lock all objects", fld.TxID(st.detail.HashID), fld.Err(err))
		// return nil from here as we can abort as a valid transition
		return StateAborted, nil
	}
	if allInShard {
		return StateObjectsDeactivated, nil
	}
	return StateAcceptPhase1Broadcasted, nil
}

func makeMessage(m *SBACMessage) (*service.Message, error) {
	payload, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  int32(Opcode_SBAC),
		Payload: payload,
	}, nil
}

func (s *Service) sendToAllShardInvolved(st *States, msg *service.Message) error {
	shards := s.shardsInvolvedWithoutSelf(st.detail.Tx)
	conns := s.conns.Borrow()
	for _, shard := range shards {
		nodes := s.top.NodesInShard(shard)
		for _, node := range nodes {
			// TODO: proper timeout ?
			_, err := conns.WriteRequest(node, msg, time.Hour, true, nil)
			if err != nil {
				log.Error("unable to connect to node", fld.TxID(st.detail.HashID), fld.PeerID(node))
				return fmt.Errorf("unable to connect to node(%v): %v", node, err)
			}
		}
	}
	return nil
}

func (s *Service) signTransaction(tx *Transaction) ([]byte, error) {
	b, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal transaction, %v", err)
	}
	return s.privkey.Sign(b), nil
}

func (s *Service) toRejectPhase1Broadcasted(st *States) (State, error) {
	signature, err := s.signTransaction(st.detail.Tx)
	if err != nil {
		log.Error("unable to sign transaction", fld.Err(err))
		return StateAborted, err
	}
	sbacmsg := &SBACMessage{
		Op:       SBACOpcode_PHASE1,
		Decision: SBACDecision_REJECT,
		PeerID:   s.nodeID,
		Tx: &SBACTransaction{
			ID:        st.detail.ID,
			Tx:        st.detail.Tx,
			Op:        SBACOpcode_PHASE1,
			Signature: signature,
		},
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("unable to serialize message reject accept transaction", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
	}

	err = s.sendToAllShardInvolved(st, msg)
	if err != nil {
		log.Error("unable to sent reject transaction to all shards", fld.TxID(st.detail.HashID), fld.Err(err))
	}

	if log.AtDebug() {
		log.Debug("reject transaction sent to all shards", fld.TxID(st.detail.HashID))
	}
	return StateAborted, err
}

func (s *Service) toAcceptPhase1Broadcasted(st *States) (State, error) {
	signature, err := s.signTransaction(st.detail.Tx)
	if err != nil {
		log.Error("unable to sign transaction", fld.Err(err))
		return StateAborted, err
	}
	sbacmsg := &SBACMessage{
		Op:       SBACOpcode_PHASE1,
		Decision: SBACDecision_ACCEPT,
		PeerID:   s.nodeID,
		Tx: &SBACTransaction{
			ID:        st.detail.ID,
			Tx:        st.detail.Tx,
			Op:        SBACOpcode_PHASE1,
			Signature: signature,
		},
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("unable to serialize message accept transaction accept", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
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
	signature, err := s.signTransaction(st.detail.Tx)
	if err != nil {
		log.Error("unable to sign transaction", fld.Err(err))
		return StateAborted, err
	}
	sbacmsg := &SBACMessage{
		Op:       SBACOpcode_PHASE2,
		Decision: SBACDecision_REJECT,
		PeerID:   s.nodeID,
		Tx: &SBACTransaction{
			ID:        st.detail.ID,
			Tx:        st.detail.Tx,
			Op:        SBACOpcode_PHASE2,
			Signature: signature,
		},
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("phase2 reject unable to serialize message", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
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
	signature, err := s.signTransaction(st.detail.Tx)
	if err != nil {
		log.Error("unable to sign transaction", fld.Err(err))
		return StateAborted, err
	}
	sbacmsg := &SBACMessage{
		Op:       SBACOpcode_PHASE2,
		Decision: SBACDecision_ACCEPT,
		PeerID:   s.nodeID,
		Tx: &SBACTransaction{
			ID:        st.detail.ID,
			Tx:        st.detail.Tx,
			Op:        SBACOpcode_PHASE2,
			Signature: signature,
		},
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("phase2 accept unable to serialize message", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
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

func (s *Service) toWaitingForPhase1(st *States) (State, error) {
	return StateWaitingForPhase1, nil
}

func (s *Service) toWaitingForPhase2(st *States) (State, error) {
	return StateWaitingForPhase2, nil
}

func (s *Service) toObjectDeactivated(st *States) (State, error) {
	objects, _ := s.inputObjectsForShard(s.shardID, st.detail.Tx)
	// lock them
	if err := DeactivateObjects(s.store, objects); err != nil {
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
	err = CreateObjects(s.store, objects)
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
	err := FinishTransaction(s.store, st.detail.ID)
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
	err := UnlockObjects(s.store, objects)
	if err != nil {
		log.Error("unable to unlock objects", fld.TxID(st.detail.HashID), fld.Err(err))
	}
	log.Error("finishing transaction", fld.TxID(st.detail.HashID))

	err = FinishTransaction(s.store, st.detail.ID)
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

func (s *Service) sendToShards(shards []uint64, st *States, msg *service.Message) error {
	conns := s.conns.Borrow()
	for _, shard := range shards {
		nodes := s.top.NodesInShard(shard)
		for _, node := range nodes {
			// TODO: proper timeout ?
			_, err := conns.WriteRequest(node, msg, 5*time.Second, true, nil)
			if err != nil {
				log.Error("unable to connect to node", fld.TxID(st.detail.HashID), fld.PeerID(node))
				return fmt.Errorf("unable to connect to node(%v): %v", node, err)
			}
		}
	}
	return nil
}

func (s *Service) toCommitObjectsBroadcasted(st *States) (State, error) {
	signature, err := s.signTransaction(st.detail.Tx)
	if err != nil {
		log.Error("unable to sign transaction", fld.Err(err))
		return StateAborted, err
	}
	sbacmsg := &SBACMessage{
		Op:       SBACOpcode_COMMIT,
		Decision: SBACDecision_ACCEPT,
		PeerID:   s.nodeID,
		Tx: &SBACTransaction{
			ID:        st.detail.ID,
			Tx:        st.detail.Tx,
			Op:        SBACOpcode_COMMIT,
			Signature: signature,
		},
	}
	msg, err := makeMessage(sbacmsg)
	if err != nil {
		log.Error("commit accept unable to serialize message", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
	}

	// send to all shards which will hold state for new object, and was not involved before
	shards := s.shardsInvolvedInTx(st.detail.Tx)
	shardsUniq := IDSlice(shards).ToMapUniq()
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
	shardsInvolved := IDSlice{}.FromMapUniq(shardsInvolvedUniq)

	err = s.sendToShards(shardsInvolved, st, msg)
	if err != nil {
		log.Error("commit unable to sent accept transaction to all shards", fld.TxID(st.detail.HashID), fld.Err(err))
		return StateAborted, err
	}

	// TODO(): this should be blocking here waiting for the shards to send us back the response so we can return an error

	if log.AtDebug() {
		log.Debug("commit accept transaction sent to all shards",
			fld.TxID(st.detail.HashID), log.Uint64s("shards_involved", shardsInvolved))
	}
	return StateSucceeded, nil
}

// TODO(): should we make our own evidences ourselves here ?
func (s *Service) toConsensus2Triggered(st *States) (State, error) {
	// broadcast transaction
	consensusTx := &SBACTransaction{
		Tx: st.detail.Tx,
		ID: st.detail.ID,
		//Evidences: st.detail.CheckersEvidences,
		Op: SBACOpcode_CONSENSUS2,
	}
	b, err := proto.Marshal(consensusTx)
	if err != nil {
		return StateAborted, fmt.Errorf("sbac: unable to marshal consensus tx: %v", err)
	}

	// choose the node to start the consensus based on the hash id of the transaction
	if s.isNodeInitiatingBroadcast(st.detail.HashID) {
		s.broadcaster.AddTransaction(b, 0)
	}
	return StateWaitingForConsensus2, nil
}

func (s *Service) toConsensusCommitTriggered(st *States) (State, error) {
	// broadcast transaction
	consensusTx := &SBACTransaction{
		Tx: st.detail.Tx,
		ID: st.detail.ID,
		//Evidences: st.detail.CheckersEvidences,
		Op: SBACOpcode_CONSENSUS_COMMIT,
	}
	b, err := proto.Marshal(consensusTx)
	if err != nil {
		return StateAborted, fmt.Errorf("sbac: unable to marshal consensus tx: %v", err)
	}

	if s.isNodeInitiatingBroadcast(st.detail.HashID) {
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

func ID(data []byte) uint32 {
	h := fnv.New32()
	h.Write(data)
	return h.Sum32()
}

type IDSlice []uint64

func (s IDSlice) ToMapUniq() map[uint64]struct{} {
	out := map[uint64]struct{}{}
	for _, v := range s {
		out[v] = struct{}{}
	}
	return out
}

func (s IDSlice) FromMapUniq(m map[uint64]struct{}) []uint64 {
	out := []uint64{}
	for k, _ := range m {
		out = append(out, k)
	}
	return out
}

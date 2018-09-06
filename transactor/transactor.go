package transactor // import "chainspace.io/prototype/transactor"

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"chainspace.io/prototype/broadcast"
	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

const (
	badgerStorePath = "/transactor/"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type Config struct {
	Broadcaster *broadcast.Service
	Directory   string
	NodeID      uint64
	Top         *network.Topology
	SigningKey  *config.Key
	Checkers    []Checker
	ShardSize   uint64
	ShardCount  uint64
	MaxPayload  int
	Key         signature.KeyPair
}

type Service struct {
	broadcaster *broadcast.Service
	checkers    CheckersMap
	conns       *ConnsPool
	nodeID      uint64
	pe          *pendingEvents
	privkey     signature.PrivateKey
	store       *badger.DB
	shardCount  uint64
	shardID     uint64
	shardSize   uint64
	table       *StateTable
	top         *network.Topology
	txstates    map[string]*StateMachine
	txstatesmu  sync.Mutex
}

func (s *Service) handleDeliver(round uint64, blocks []*broadcast.SignedData) {
	for _, signed := range blocks {
		block, err := signed.Block()
		if err != nil {
			log.Fatal("Unable to decode delivered block", fld.Round(round), fld.Err(err))
		}
		it := block.Iter()
		for it.Valid() {
			it.Next()
			// TODO(): do stuff with the fee?
			tx := SBACTransaction{}
			err := proto.Unmarshal(it.TxData, &tx)
			if err != nil {
				log.Error("Unable to unmarshal transaction data", fld.Err(err))
				continue
			}
			e := &Event{
				msg: &SBACMessage{
					Op:       tx.Op,
					Decision: SBACDecision_ACCEPT,
					Tx:       &tx,
				},
				peerID: 99,
			}
			s.pe.OnEvent(e)
		}
	}
}

func (s *Service) Handle(peerID uint64, m *service.Message) (*service.Message, error) {
	ctx := context.TODO()
	switch Opcode(m.Opcode) {
	case Opcode_CHECK_TRANSACTION:
		return s.checkTransaction(ctx, m.Payload, m.ID)
	case Opcode_ADD_TRANSACTION:
		return s.addTransaction(ctx, m.Payload, m.ID)
	case Opcode_QUERY_OBJECT:
		return s.queryObject(ctx, m.Payload, m.ID)
	case Opcode_DELETE_OBJECT:
		return s.deleteObject(ctx, m.Payload, m.ID)
	case Opcode_CREATE_OBJECT:
		return s.createObject(ctx, m.Payload, m.ID)
	case Opcode_STATES:
		return s.handleStates(ctx, m.Payload, m.ID)
	case Opcode_SBAC:
		return s.handleSBAC(ctx, m.Payload, peerID, m.ID)
	default:
		log.Error("transactor: unknown message opcode", log.Int32("opcode", m.Opcode), fld.PeerID(peerID), log.Int("len", len(m.Payload)))
		return nil, fmt.Errorf("transactor: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) consumeEvents(e *Event) bool {
	// check if statemachine is finished
	ok, err := TxnFinished(s.store, e.msg.GetTx().GetID())
	if err != nil {
		log.Error("error calling TxnFinished", fld.Err(err))
		// do nothing
		return true
	}
	if ok {
		log.Error("event for a finished transaction, skipping it", fld.TxID(ID(e.msg.GetTx().ID)), log.Uint64("PEERID", e.peerID))
		// txn already finished. move on
		return true
	}

	sm, ok := s.getSateMachine(e.msg.Tx.ID)
	if ok {
		if log.AtDebug() {
			log.Debug("sending new event to statemachine", fld.TxID(ID(e.msg.Tx.ID)), fld.PeerID(e.peerID), fld.PeerShard(s.top.ShardForNode(e.peerID)))
		}
		sm.OnEvent(e)
		return true
	}
	if log.AtDebug() {
		log.Debug("statemachine not ready", fld.TxID(ID(e.msg.Tx.ID)))
	}
	return false
}

func (s *Service) handleSBAC(ctx context.Context, payload []byte, peerID uint64, msgID uint64) (*service.Message, error) {
	req := &SBACMessage{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("transactor: sbac unmarshaling error", fld.Err(err))
		return nil, fmt.Errorf("transactor: sbac unmarshaling error: %v", err)
	}
	// if we received a COMMIT opcode, the statemachine may not exists
	// lets check and create it here.
	if req.Op == SBACOpcode_COMMIT {
		txdetails := NewTxDetails(req.Tx.ID, []byte{}, req.Tx.Tx, req.Tx.Evidences)
		_ = s.getOrCreateStateMachine(txdetails, StateWaitingForCommit)
	}
	e := &Event{msg: req, peerID: peerID}
	s.pe.OnEvent(e)
	res := SBACMessageAck{LastID: msgID}
	payloadres, err := proto.Marshal(&res)
	if err != nil {
		log.Error("unable to marshall handleSBAC response", fld.Err(err))
		return nil, err
	}
	return &service.Message{ID: msgID, Opcode: int32(Opcode_SBAC), Payload: payloadres}, nil
}

func (s *Service) checkTransaction(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &CheckTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("transactor: checkTransaction unmarshaling error", fld.Err(err))
		return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
	}

	// run the checkers
	ok, err := runCheckers(ctx, s.checkers, req.Tx)
	if err != nil {
		log.Error("transactor: errors happend while checkers", fld.Err(err))
		return nil, fmt.Errorf("transactor: errors happend while running the checkers: %v", err)
	}

	// create txID and signature then payload
	ids, err := MakeIDs(req.Tx)
	if err != nil {
		log.Error("transactor: unable to generate IDs", fld.Err(err))
		return nil, err
	}
	res := &CheckTransactionResponse{
		Ok:        ok,
		Signature: s.privkey.Sign(ids.TxID),
	}

	b, err := proto.Marshal(res)
	if err != nil {
		log.Error("unable to marshal checkTransaction response", fld.Err(err))
		return nil, fmt.Errorf("transactor: unable to marshal check_transaction response")
	}

	if log.AtDebug() {
		log.Debug("transactor: transaction checked successfully", fld.TxID(ID(ids.TxID)))
	}
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_ADD_TRANSACTION),
		Payload: b,
	}, nil
}

func (s *Service) verifySignatures(txID []byte, evidences map[uint64][]byte) bool {
	ok := true
	keys := s.top.SeedPublicKeys()
	for nodeID, sig := range evidences {
		key := keys[nodeID]
		if !key.Verify(txID, sig) {
			if log.AtDebug() {
				log.Debug("invalid signature", fld.PeerID(nodeID))
			}
			ok = false
		}
	}
	return ok
}

func (s *Service) addStateMachine(txdetails *TxDetails, initialState State) *StateMachine {
	s.txstatesmu.Lock()
	sm := NewStateMachine(s.table, txdetails, initialState)
	s.txstates[string(txdetails.ID)] = sm
	s.txstatesmu.Unlock()
	return sm
}

func (s *Service) getSateMachine(txID []byte) (*StateMachine, bool) {
	s.txstatesmu.Lock()
	sm, ok := s.txstates[string(txID)]
	s.txstatesmu.Unlock()
	return sm, ok
}

func (s *Service) getOrCreateStateMachine(txdetails *TxDetails, initialState State) *StateMachine {
	s.txstatesmu.Lock()
	defer s.txstatesmu.Unlock()
	sm, ok := s.txstates[string(txdetails.ID)]
	if ok {
		return sm
	}
	sm = NewStateMachine(s.table, txdetails, initialState)
	s.txstates[string(txdetails.ID)] = sm
	return sm
}

func (s *Service) gcStateMachines() {
	for {
		time.Sleep(1 * time.Second)
		s.txstatesmu.Lock()
		for k, v := range s.txstates {
			if v.State() == StateAborted || v.State() == StateSucceeded {
				if log.AtDebug() {
					log.Debug("removing statemachine", log.String("finale_state", v.State().String()), fld.TxID(ID([]byte(k))))
				}
				v.Close()
				delete(s.txstates, k)
			}
		}
		s.txstatesmu.Unlock()
	}
}

func (s *Service) addTransaction(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &AddTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("transactor: unable to unmarshal AddTransaction", fld.Err(err))
		return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
	}

	ids, err := MakeIDs(req.Tx)
	if err != nil {
		log.Error("unable to create IDs", fld.Err(err))
		return nil, err
	}

	if !s.verifySignatures(ids.TxID, req.Evidences) {
		log.Error("invalid evidences from nodes")
		return nil, errors.New("transactor: invalid evidences from nodes")
	}
	log.Info("transactor: all evidence verified with success")

	objects := map[string]*ObjectList{}
	for _, v := range ids.TraceObjectPairs {
		trID := base64.StdEncoding.EncodeToString(v.Trace.ID)
		objects[trID] = &ObjectList{v.OutputObjects}
	}
	rawtx, err := proto.Marshal(req.Tx)
	if err != nil {
		log.Error("unable to marshal transaction", fld.Err(err))
		return nil, fmt.Errorf("transactor: unable to marshal tx: %v", err)
	}
	txdetails := NewTxDetails(ids.TxID, rawtx, req.Tx, req.Evidences)

	s.addStateMachine(txdetails, StateWaitingForConsensus1)

	// broadcast transaction
	consensusTx := &SBACTransaction{
		Tx:        req.Tx,
		ID:        ids.TxID,
		Evidences: req.Evidences,
		Op:        SBACOpcode_CONSENSUS1,
	}
	b, err := proto.Marshal(consensusTx)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal consensus tx: %v", err)
	}
	if s.isNodeInitiatingBroadcast(ID(ids.TxID)) {
		s.broadcaster.AddTransaction(b, 0)
	}
	res := &AddTransactionResponse{
		Objects: objects,
	}

	b, err = proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal add_transaction response, %v", err)
	}
	log.Info("transactor: transaction added successfully")
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_ADD_TRANSACTION),
		Payload: b,
	}, nil
}

func queryPayload(id uint64, res *QueryObjectResponse) (*service.Message, error) {
	b, _ := proto.Marshal(res)
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_QUERY_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) queryObject(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &QueryObjectRequest{}
	err := proto.Unmarshal(payload, req)
	res := &QueryObjectResponse{}
	if err != nil {
		res.Error = fmt.Errorf("transactor: query_object unmarshaling error: %v", err).Error()
		return queryPayload(id, res)
	}

	if req.ObjectKey == nil {
		res.Error = fmt.Errorf("transactor: nil object key").Error()
		return queryPayload(id, res)
	}
	objects, err := GetObjects(s.store, [][]byte{req.ObjectKey})
	if err != nil {
		res.Error = err.Error()
	} else if len(objects) != 1 {
		res.Error = fmt.Errorf("transactor: invalid number of objects found, expected %v found %v", 1, len(objects)).Error()
	} else {
		res.Object = objects[0]
	}
	return queryPayload(id, res)
}

func (s *Service) handleStates(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	sr := []*StateReport{}
	s.txstatesmu.Lock()
	for _, v := range s.txstates {
		sr = append(sr, v.StateReport())
	}
	s.txstatesmu.Unlock()
	res := &StatesReportResponse{
		States:        sr,
		EventsInQueue: int32(s.pe.Len()),
	}
	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal states reports response")
	}
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_STATES),
		Payload: b,
	}, nil
}

func deletePayload(id uint64, res *DeleteObjectResponse) (*service.Message, error) {
	b, _ := proto.Marshal(res)
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_DELETE_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) deleteObject(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &DeleteObjectRequest{}
	res := &DeleteObjectResponse{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		res.Error = fmt.Errorf("transactor: remove_object unmarshaling error: %v", err).Error()
		return deletePayload(id, res)
	}

	if req.ObjectKey == nil {
		res.Error = fmt.Errorf("transactor: nil object key").Error()
		return deletePayload(id, res)
	}
	err = DeleteObjects(s.store, [][]byte{req.ObjectKey})
	if err != nil {
		res.Error = err.Error()
	}
	objects, err := GetObjects(s.store, [][]byte{req.ObjectKey})
	if err != nil {
		res.Error = err.Error()
	} else if len(objects) != 1 {
		res.Error = fmt.Errorf("transactor: invalid number of objects removed, expected %v found %v", 1, len(objects)).Error()
	} else {
		res.Object = objects[0]
	}
	return deletePayload(id, res)
}

func createPayload(id uint64, res *NewObjectResponse) (*service.Message, error) {
	b, _ := proto.Marshal(res)
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_CREATE_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) createObject(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &NewObjectRequest{}
	err := proto.Unmarshal(payload, req)
	res := &NewObjectResponse{}
	if err != nil {
		res.Error = fmt.Errorf("transactor: new_object unmarshaling error: %v", err).Error()
		return createPayload(id, res)
	}

	if req.Object == nil || len(req.Object) <= 0 {
		res.Error = fmt.Errorf("transactor: nil object key").Error()
		return createPayload(id, res)
	}
	ch := combihash.New()
	ch.Write([]byte(req.Object))
	key := ch.Digest()
	if log.AtDebug() {
		log.Debug("transactor: creating new object", log.String("objet", string(req.Object)), log.Uint32("object.id", ID(key)))
	}
	o, err := CreateObject(s.store, key, req.Object)
	if err != nil {
		if log.AtDebug() {
			log.Debug("transactor: unable to create object", log.String("objet", string(req.Object)), log.Uint32("object.id", ID(key)), fld.Err(err))
		}
		res.Error = err.Error()
	} else {
		res.ID = o.Key
	}
	return createPayload(id, res)
}

func (s *Service) Name() string {
	return "transactor"
}

func (s *Service) Stop() error {
	return s.store.Close()
}

func New(cfg *Config) (*Service, error) {
	checkers := map[string]map[string]Checker{}
	for _, c := range cfg.Checkers {
		if m, ok := checkers[c.ContractID()]; ok {
			m[c.Name()] = c
			continue
		}
		checkers[c.ContractID()] = map[string]Checker{c.Name(): c}
	}

	algorithm, err := signature.AlgorithmFromString(cfg.SigningKey.Type)
	if err != nil {
		return nil, err
	}
	privKeybytes, err := b32.DecodeString(cfg.SigningKey.Private)
	if err != nil {
		return nil, err
	}
	privkey, err := signature.LoadPrivateKey(algorithm, privKeybytes)
	if err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions
	badgerPath := path.Join(cfg.Directory, badgerStorePath)
	opts.Dir = badgerPath
	opts.ValueDir = badgerPath
	store, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	s := &Service{
		broadcaster: cfg.Broadcaster,
		conns:       NewConnsPool(20, cfg.NodeID, cfg.Top, cfg.MaxPayload, cfg.Key),
		checkers:    checkers,
		nodeID:      cfg.NodeID,
		privkey:     privkey,
		top:         cfg.Top,
		txstates:    map[string]*StateMachine{},
		store:       store,
		shardID:     cfg.Top.ShardForNode(cfg.NodeID),
		shardCount:  cfg.ShardCount,
		shardSize:   cfg.ShardSize,
	}
	s.pe = NewPendingEvents(s.consumeEvents)
	s.table = s.makeStateTable()
	s.broadcaster.Register(s.handleDeliver)
	go s.pe.Run()
	// go s.gcStateMachines()
	return s, nil
}

func (s *Service) isNodeInitiatingBroadcast(txID uint32) bool {
	nodesInShard := s.top.NodesInShard(s.shardID)
	n := nodesInShard[txID%(uint32(len(nodesInShard)))]
	log.Debug("consensus will be started", log.Uint64("peer", n))
	return n == s.nodeID
}

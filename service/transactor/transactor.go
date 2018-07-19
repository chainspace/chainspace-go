package transactor // import "chainspace.io/prototype/service/transactor"

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/service/broadcast"

	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
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
	broadcaster   *broadcast.Service
	checkers      CheckersMap
	conns         *ConnsCache
	nodeID        uint64
	privkey       signature.PrivateKey
	txstates      map[string]*StateMachine
	txstatesmu    sync.Mutex
	store         *badger.DB
	shardCount    uint64
	shardID       uint64
	shardSize     uint64
	table         *StateTable
	top           *network.Topology
	pendingEvents chan *Event
}

func (s *Service) DeliverStart(round uint64) {
	log.Info("DELIVER START", zap.Uint64("round", round))
}

func (s *Service) DeliverTransaction(txdata *broadcast.TransactionData) {
	// TODO(): do stuff with the fee?
	ctx := &ConsensusTransaction{}
	err := proto.Unmarshal(txdata.Data, ctx)
	if err != nil {
		log.Error("unable to unmarshal transaction data", zap.Error(err))
	}
	e := &Event{
		msg: &SBACMessage{
			Op:                   ctx.ConsensusRound,
			Decision:             SBACDecision_ACCEPT,
			TransactionID:        ctx.ID,
			ConsensusTransaction: ctx,
		},
		peerID: ctx.PeerID,
	}
	log.Info("new transaction broadcasted from consensus", zap.Uint32("tx.id", ID(ctx.ID)))
	s.pendingEvents <- e
}

func (s *Service) DeliverEnd(round uint64) {
	log.Info("DELIVER END", zap.Uint64("round", round))
}

func (s *Service) Handle(ctx context.Context, peerID uint64, m *service.Message) (*service.Message, error) {
	switch Opcode(m.Opcode) {
	case Opcode_CHECK_TRANSACTION:
		return s.checkTransaction(ctx, m.Payload)
	case Opcode_ADD_TRANSACTION:
		return s.addTransaction(ctx, m.Payload)
	case Opcode_QUERY_OBJECT:
		return s.queryObject(ctx, m.Payload)
	case Opcode_DELETE_OBJECT:
		return s.deleteObject(ctx, m.Payload)
	case Opcode_CREATE_OBJECT:
		return s.createObject(ctx, m.Payload)
	case Opcode_SBAC:
		return s.handleSBAC(ctx, m.Payload, peerID)
	default:
		log.Error("transactor: unknown message opcode", zap.Uint32("opcode", m.Opcode), zap.Uint64("peer.id", peerID))
		return nil, fmt.Errorf("transactor: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) consumeEvents() {
	for e := range s.pendingEvents {
		sm, ok := s.getSateMachine(e.msg.TransactionID)
		if ok {
			log.Info("sending new event to statemachine", zap.Uint32("id", ID(e.msg.TransactionID)), zap.Uint64("peer.id", e.peerID), zap.Uint64("peer.shard", s.top.ShardForNode(e.peerID)))
			sm.OnEvent(e)
			continue
		}
		log.Info("statemachine not ready", zap.Uint32("id", ID(e.msg.TransactionID)))
		s.pendingEvents <- e
	}
}

func (s *Service) handleSBAC(ctx context.Context, payload []byte, peerID uint64) (*service.Message, error) {
	req := &SBACMessage{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("transactor: sbac unmarshaling error", zap.Error(err))
		return nil, fmt.Errorf("transactor: sbac unmarshaling error: %v", err)
	}
	// if we received a COMMIT opcode, the statemachine may not exists
	// lets check an create it here.
	if req.Op == SBACOpcode_COMMIT {
		txdetails := NewTxDetails(req.TransactionID, []byte{}, req.ConsensusTransaction.Tx, req.ConsensusTransaction.Evidences)
		_ = s.getOrCreateStateMachine(txdetails, StateWaitingForCommit)
	}
	e := &Event{msg: req, peerID: peerID}
	s.pendingEvents <- e
	return &service.Message{}, nil
}

func (s *Service) checkTransaction(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &CheckTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("transactor: checkTransaction unmarshaling error", zap.Error(err))
		return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
	}

	// run the checkers
	ok, err := runCheckers(ctx, s.checkers, req.Tx)
	if err != nil {
		log.Error("transactor: errors happend while checkers", zap.Error(err))
		return nil, fmt.Errorf("transactor: errors happend while running the checkers: %v", err)
	}

	// create txID and signature then payload
	ids, err := MakeIDs(req.Tx)
	if err != nil {
		log.Error("transactor: unable to generate IDs", zap.Error(err))
		return nil, err
	}
	res := &CheckTransactionResponse{
		Ok:        ok,
		Signature: s.privkey.Sign(ids.TxID),
	}

	b, err := proto.Marshal(res)
	if err != nil {
		log.Error("unable to marshal checkTransaction response", zap.Error(err))
		return nil, fmt.Errorf("transactor: unable to marshal check_transaction response")
	}

	log.Info("transactor: transaction checked successfully")
	return &service.Message{
		Opcode:  uint32(Opcode_ADD_TRANSACTION),
		Payload: b,
	}, nil
}

func (s *Service) verifySignatures(txID []byte, evidences map[uint64][]byte) bool {
	ok := true
	keys := s.top.SeedPublicKeys()
	for nodeID, sig := range evidences {
		key := keys[nodeID]
		if !key.Verify(txID, sig) {
			log.Info("invalid signature", zap.Uint64("peer.id", nodeID))
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
				log.Info("removing statemachine", zap.String("finale_state", v.State().String()), zap.Uint32("tx.id", ID([]byte(k))))
			}
			delete(s.txstates, k)
		}
		s.txstatesmu.Unlock()
	}
}

func (s *Service) addTransaction(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &AddTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("transactor: unable to unmarshal AddTransaction", zap.Error(err))
		return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
	}

	ids, err := MakeIDs(req.Tx)
	if err != nil {
		log.Error("unable to create IDs", zap.Error(err))
		return nil, err
	}

	if !s.verifySignatures(ids.TxID, req.Evidences) {
		log.Error("invalid evidences from nodes")
		return nil, errors.New("transactor: invalid evidences from nodes")
	}
	log.Info("transactor: all evidence verified with success")

	objects := map[string]*ObjectList{}
	for _, v := range ids.TraceObjectPairs {
		objects[string(v.Trace.ID)] = &ObjectList{v.OutputObjects}
	}
	rawtx, err := proto.Marshal(req.Tx)
	if err != nil {
		log.Error("unable to marshal transaction", zap.Error(err))
		return nil, fmt.Errorf("transactor: unable to marshal tx: %v", err)
	}
	txdetails := NewTxDetails(ids.TxID, rawtx, req.Tx, req.Evidences)

	s.addStateMachine(txdetails, StateWaitingForConsensus1)

	// broadcast transaction
	consensusTx := &ConsensusTransaction{
		Tx:             req.Tx,
		ID:             ids.TxID,
		Evidences:      req.Evidences,
		PeerID:         s.nodeID,
		ConsensusRound: SBACOpcode_CONSENSUS1,
	}
	b, err := proto.Marshal(consensusTx)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal consensus tx: %v", err)
	}
	s.broadcaster.AddTransaction(&broadcast.TransactionData{Data: b})

	// block here while the statemachine does its job
	txres := <-txdetails.Result
	if !txres {
		return nil, errors.New("unable to execute the transaction")
	}
	res := &AddTransactionResponse{
		Objects: objects,
	}

	b, err = proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal add_transaction response")
	}
	log.Info("transactor: transaction added successfully")

	return &service.Message{
		Opcode:  uint32(Opcode_ADD_TRANSACTION),
		Payload: b,
	}, nil
}

func (s *Service) queryObject(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &QueryObjectRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		return nil, fmt.Errorf("transactor: query_object unmarshaling error: %v", err)
	}

	if req.ObjectKey == nil {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	objects, err := GetObjects(s.store, [][]byte{req.ObjectKey})
	if err != nil {
		return nil, err
	}
	if len(objects) != 1 {
		return nil, fmt.Errorf("transactor: invalid number of objects found, expected %v found %v", 1, len(objects))
	}

	if err != nil {
		return nil, err
	}
	res := &QueryObjectResponse{
		Object: objects[0],
	}
	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal query_object response")
	}
	return &service.Message{
		Opcode:  uint32(Opcode_QUERY_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) deleteObject(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &DeleteObjectRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		return nil, fmt.Errorf("transactor: remove_object unmarshaling error: %v", err)
	}

	if req.ObjectKey == nil {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	err = DeleteObjects(s.store, [][]byte{req.ObjectKey})
	if err != nil {
		return nil, err
	}
	objects, err := GetObjects(s.store, [][]byte{req.ObjectKey})
	if err != nil {
		return nil, err
	}
	if len(objects) != 1 {
		return nil, fmt.Errorf("transactor: invalid number of objects removed, expected %v found %v", 1, len(objects))
	}

	if err != nil {
		return nil, err
	}
	res := &DeleteObjectResponse{
		Object: objects[0],
	}
	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal remove_object response")
	}
	return &service.Message{
		Opcode:  uint32(Opcode_DELETE_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) createObject(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &NewObjectRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		return nil, fmt.Errorf("transactor: new_object unmarshaling error: %v", err)
	}

	if req.Object == nil || len(req.Object) <= 0 {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	ch := combihash.New()
	ch.Write([]byte(req.Object))
	key := ch.Digest()
	log.Info("transactor: creating new object", zap.String("objet", string(req.Object)), zap.Uint32("object.id", ID(key)))
	o, err := CreateObject(s.store, key, req.Object)
	if err != nil {
		log.Info("transactor: unable to create object", zap.String("objet", string(req.Object)), zap.Uint32("object.id", ID(key)), zap.Error(err))
		return nil, err
	}

	res := &NewObjectResponse{
		ID: o.Key,
	}
	b, err := proto.Marshal(res)
	if err != nil {
		log.Error("unable to marshal NewObject reponse", zap.Error(err))
		return nil, fmt.Errorf("transactor: unable to marshal new_object response")
	}
	return &service.Message{Opcode: uint32(Opcode_CREATE_OBJECT), Payload: b}, nil
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
		broadcaster:   cfg.Broadcaster,
		conns:         NewConnsCache(context.TODO(), cfg.NodeID, cfg.Top, cfg.MaxPayload, cfg.Key),
		checkers:      checkers,
		nodeID:        cfg.NodeID,
		privkey:       privkey,
		top:           cfg.Top,
		txstates:      map[string]*StateMachine{},
		store:         store,
		shardID:       cfg.Top.ShardForNode(cfg.NodeID),
		shardCount:    cfg.ShardCount,
		shardSize:     cfg.ShardSize,
		pendingEvents: make(chan *Event, 1000),
	}
	s.table = s.makeStateTable()
	s.broadcaster.Register(s)
	go s.consumeEvents()
	// go s.gcStateMachines()
	return s, nil
}

package transactor // import "chainspace.io/prototype/service/transactor"

import (
	"context"
	"encoding/base32"
	"errors"
	"fmt"
	"path"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/service/broadcast"

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
}

type Service struct {
	broadcaster *broadcast.Service
	checkers    CheckersMap
	nodeID      uint64
	privkey     signature.PrivateKey
	txstates    map[string]*StateMachine
	store       *badger.DB
	shardCount  uint64
	shardID     uint64
	shardSize   uint64
	table       *StateTable
	top         *network.Topology
}

func (s *Service) BroadcastStart(round uint64) {
	log.Infof("broadcast start: %v", round)
}

func (s *Service) BroadcastTransaction(txdata *broadcast.TransactionData) {
	log.Infof("broadcast transaction")
}

func (s *Service) BroadcastEnd(round uint64) {
	log.Infof("broadcast end: %v", round)
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
	default:
		return nil, fmt.Errorf("transactor: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) checkTransaction(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &CheckTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
	}

	// run the checkers
	ok, err := runCheckers(ctx, s.checkers, req.Tx)
	if err != nil {
		return nil, fmt.Errorf("transactor: errors happend while running the checkers: %v", err)
	}

	// create txID and signature then payload
	ids, err := MakeIDs(req.Tx)
	if err != nil {
		return nil, err
	}
	res := &CheckTransactionResponse{
		Ok:        ok,
		Signature: s.privkey.Sign(ids.TxID),
	}

	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal check_transaction response")
	}

	log.Infof("transactor: transaction checked successfully")
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
			log.Infof("invalid signature from node %v", nodeID)
			ok = false
		}
	}
	return ok
}

func (s *Service) addTransaction(ctx context.Context, payload []byte) (*service.Message, error) {
	req := &AddTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
	}

	ids, err := MakeIDs(req.Tx)
	if err != nil {
		return nil, err
	}

	if !s.verifySignatures(ids.TxID, req.Evidences) {
		return nil, errors.New("transactor: invalid evidences from nodes")
	}
	log.Infof("transactor: all evidence verified with success")

	objects := map[string]*ObjectList{}
	for _, v := range ids.TraceObjectPairs {
		objects[string(v.Trace.ID)] = &ObjectList{v.OutputObjects}
	}
	rawtx, err := proto.Marshal(req.Tx)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal tx: %v", err)
	}
	txdetails := TxDetails{
		CheckersEvidences: req.Evidences,
		ID:                ids.TxID,
		Raw:               rawtx,
		Result:            make(chan bool),
		Tx:                req.Tx,
	}

	sm := NewStateMachine(s.table, &txdetails)
	// start the statemachine
	s.txstates[string(txdetails.ID)] = sm
	// send an empty event for now in order to start the transitions
	sm.OnEvent(nil)
	txres := <-txdetails.Result

	if !txres {
		return nil, errors.New("unable to execute the transaction")
	}
	res := &AddTransactionResponse{
		Objects: objects,
	}

	// block here while the statemachine does its job

	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to marshal add_transaction response")
	}
	log.Infof("transactor: transaction added successfully")

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
	log.Infof("transactor: creating new object(%v) with id(%v)", string(req.Object), string(key))
	o, err := CreateObject(s.store, key, req.Object)
	if err != nil {
		log.Infof("transactor: unable to create object(%v) with id(%v): %v", req.Object, key, err)
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	res := &NewObjectResponse{
		ID: o.Key,
	}
	b, err := proto.Marshal(res)
	if err != nil {
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
		broadcaster: cfg.Broadcaster,
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
	s.table = s.makeStateTable()
	s.broadcaster.Register(s)

	return s, nil
}

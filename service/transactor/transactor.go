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
		req := &QueryObjectRequest{}
		err := proto.Unmarshal(m.Payload, req)
		if err != nil {
			return nil, fmt.Errorf("transactor: query_object unmarshaling error: %v", err)
		}
		obj, err := s.queryObject(ctx, req.ObjectKey)
		if err != nil {
			return nil, err
		}
		res := &QueryObjectResponse{
			Object: obj,
		}
		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal query_object response")
		}
		return &service.Message{Opcode: uint32(Opcode_QUERY_OBJECT), Payload: b}, nil
	case Opcode_DELETE_OBJECT:
		req := &DeleteObjectRequest{}
		err := proto.Unmarshal(m.Payload, req)
		if err != nil {
			return nil, fmt.Errorf("transactor: remove_object unmarshaling error: %v", err)
		}
		obj, err := s.removeObject(ctx, req.ObjectKey)
		if err != nil {
			return nil, err
		}
		res := &DeleteObjectResponse{
			Object: obj,
		}
		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal remove_object response")
		}
		return &service.Message{Opcode: uint32(Opcode_DELETE_OBJECT), Payload: b}, nil
	case Opcode_CREATE_OBJECT:
		req := &NewObjectRequest{}
		err := proto.Unmarshal(m.Payload, req)
		if err != nil {
			return nil, fmt.Errorf("transactor: new_object unmarshaling error: %v", err)
		}
		id, err := s.createObject(ctx, req.Object)
		if err != nil {
			return nil, err
		}
		res := &NewObjectResponse{
			ID: id,
		}
		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal new_object response")
		}
		return &service.Message{Opcode: uint32(Opcode_CREATE_OBJECT), Payload: b}, nil
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
		return nil, fmt.Errorf("transactor: unable to marshal add_transaction response")
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
		Tx:                req.Tx,
	}

	// start the statemachine
	s.txstates[string(txdetails.ID)] = NewStateMachine(s.table, &txdetails)
	// send an empty event for now in order to start the transitions
	sm := s.txstates[string(txdetails.ID)]
	sm.OnEvent(nil)

	res := &AddTransactionResponse{
		Objects: objects,
	}

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

func (s *Service) queryObject(ctx context.Context, objectKey []byte) (*Object, error) {
	if objectKey == nil {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	objects, err := GetObjects(s.store, [][]byte{objectKey})
	if err != nil {
		return nil, err
	}
	if len(objects) != 1 {
		return nil, fmt.Errorf("transactor: invalid number of objects found, expected %v found %v", 1, len(objects))
	}

	return objects[0], nil
}

func (s *Service) removeObject(ctx context.Context, objectKey []byte) (*Object, error) {
	if objectKey == nil {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	err := DeleteObjects(s.store, [][]byte{objectKey})
	if err != nil {
		return nil, err
	}
	objects, err := GetObjects(s.store, [][]byte{objectKey})
	if err != nil {
		return nil, err
	}
	if len(objects) != 1 {
		return nil, fmt.Errorf("transactor: invalid number of objects removed, expected %v found %v", 1, len(objects))
	}

	return objects[0], nil
}

func (s *Service) createObject(ctx context.Context, object []byte) ([]byte, error) {
	if object == nil || len(object) <= 0 {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	ch := combihash.New()
	ch.Write([]byte(object))
	key := ch.Digest()
	log.Infof("transactor: creating new object(%v) with id(%v)", string(object), string(key))
	o, err := CreateObject(s.store, key, object)
	if err != nil {
		log.Infof("transactor: unable to create object(%v) with id(%v): %v", object, key, err)
		return nil, err
	}
	return o.Key, nil
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

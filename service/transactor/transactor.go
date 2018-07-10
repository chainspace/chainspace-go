package transactor // import "chainspace.io/prototype/service/transactor"

import (
	"context"
	"encoding/base32"
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
	state       map[string]*StateMachine
	store       *badger.DB
	shardCount  uint64
	shardID     uint64
	shardSize   uint64
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
		req := &AddTransactionRequest{}
		err := proto.Unmarshal(m.Payload, req)
		if err != nil {
			return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
		}
		objs, err := s.addTransaction(ctx, req.Transaction, m.Payload)
		if err != nil {
			log.Errorf("transactor: failed to add transaction: %v", err)
		}
		res := &AddTransactionResponse{
			Pairs: objs,
		}

		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal add_transaction response")
		}
		log.Infof("transactor: transaction added successfully")
		return &service.Message{Opcode: uint32(Opcode_ADD_TRANSACTION), Payload: b}, nil
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
		id, err := s.newObject(ctx, req.Object)
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

func (s *Service) addTransaction(ctx context.Context, tx *Transaction, rawtx []byte) ([]*ObjectTraceIDPair, error) {
	tracesidpairs, err := MakeTraceIDs(tx.Traces)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to create traces ids: %v", err)
	}
	objects, err := MakeTraceObjectPairs(tracesidpairs)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to create objects keys: %v", err)
	}

	result := make([]*ObjectTraceIDPair, 0, len(objects))
	for _, v := range objects {
		r := &ObjectTraceIDPair{
			TraceID: v.Trace.ID,
			Objects: v.OutputObjects,
		}
		result = append(result, r)
	}

	ch := combihash.New()
	if _, err := ch.Write(rawtx); err != nil {
		return nil, fmt.Errorf("transactor: unable to hash transaction: %v", err)
	}
	txhash := ch.Digest()
	txsignature := s.privkey.Sign(txhash)
	_ = txsignature

	// Start sending the tx to other nodes / shards in order to do the consensus thingy
	// then return the output objects to the users.

	return result, nil
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

func (s *Service) newObject(ctx context.Context, object []byte) ([]byte, error) {
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
		state:       map[string]*StateMachine{},
		store:       store,
		shardID:     cfg.Top.ShardForNode(cfg.NodeID),
		shardCount:  cfg.ShardCount,
		shardSize:   cfg.ShardSize,
	}

	s.broadcaster.Register(s)

	return s, nil
}

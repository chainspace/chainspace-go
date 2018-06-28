package transactor // import "chainspace.io/prototype/service/transactor"

import (
	"context"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/gogo/protobuf/proto"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/storage"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// CheckersMap map contractID to map check procedure name to checker
type CheckersMap map[string]map[string]Checker

type Config struct {
	NodeID     uint64
	Top        *network.Topology
	SigningKey *config.Key
	Store      storage.DB
	Checkers   []Checker
}

// a pair of a trace and it's associated checker to be store in a slice
type checkerTracePair struct {
	Checker Checker
	Trace   *Trace
}

type Service struct {
	checkers CheckersMap
	nodeID   uint64
	privkey  signature.PrivateKey
	top      *network.Topology
	store    storage.DB
}

func (s *Service) Handle(ctx context.Context, m *service.Message) (*service.Message, error) {
	switch Opcode(m.Opcode) {
	case Opcode_ADD_TRANSACTION:
		req := &AddTransactionRequest{}
		err := proto.Unmarshal(m.Payload, req)
		if err != nil {
			return nil, fmt.Errorf("transactor: add_transaction unmarshaling error: %v", err)
		}
		objs, err := s.addTransaction(ctx, req.Transaction)
		res := &AddTransactionResponse{
			Objects: objs,
		}

		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal add_transaction response")
		}
		return &service.Message{uint32(Opcode_ADD_TRANSACTION), b}, nil
	case Opcode_QUERY_OBJECT:
		req := &QueryObjectRequest{}
		err := proto.Unmarshal(m.Payload, req)
		if err != nil {
			return nil, fmt.Errorf("transactor: query_object unmarshaling error: %v", err)
		}
		obj, err := s.queryObject(ctx, req.ObjectKey)
		res := &QueryObjectResponse{
			Object: obj,
		}
		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal query_object response")
		}
		return &service.Message{uint32(Opcode_QUERY_OBJECT), b}, nil

	default:
		return nil, fmt.Errorf("transactor: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) addTransaction(ctx context.Context, tx *Transaction) ([]*Object, error) {
	ok, err := runCheckers(ctx, s.checkers, tx)
	if err != nil {
		return nil, fmt.Errorf("transactor: errors happend while running the checkers: %v", err)
	}
	if !ok {
		return nil, errors.New("transactor: some checks were not successful")
	}

	objects, err := MakeObjectKeys(ctx, tx.Traces)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to create objects keys: %v", err)
	}

	return objects, nil
}

// runCheckers will run concurently all the checkers for this transaction
func runCheckers(ctx context.Context, checkers CheckersMap, tx *Transaction) (bool, error) {
	ctpairs, err := aggregateCheckers(checkers, tx.Traces)
	if err != nil {
		return false, err
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, v := range ctpairs {
		v := v
		t := v.Trace
		c := v.Checker
		g.Go(func() error {
			result := c.Check(t.InputObjectsKeys, t.InputReferencesKeys, t.Params,
				t.OutputObjects, t.Returns, t.Dependencies)
			if !result {
				return errors.New("check failed")
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return false, nil
	}
	return true, nil
}

// aggregateCheckers first ensure that all the contracts and procedures used in the transaction
// exists then map each transaction to the associated contract.
func aggregateCheckers(checkers CheckersMap, traces []*Trace) ([]checkerTracePair, error) {
	var ok bool
	var m map[string]Checker
	var checker Checker
	ctpairs := []checkerTracePair{}
	for _, t := range traces {
		m, ok = checkers[t.ContractID]
		if !ok {
			return nil, fmt.Errorf("transactor: unknown contract with ID: %v", t.ContractID)
		}
		checker, ok = m[t.Procedure]
		if !ok {
			return nil, fmt.Errorf("transactor: unknown procedure %v for contract with ID: %v", t.Procedure, t.ContractID)
		}
		newpairs, err := aggregateCheckers(checkers, t.Dependencies)
		if err != nil {
			return nil, err
		}
		newpair := checkerTracePair{Checker: checker, Trace: t}
		ctpairs = append(ctpairs, newpair)
		ctpairs = append(ctpairs, newpairs...)
	}
	return ctpairs, nil
}

func (s *Service) queryObject(ctx context.Context, objectKey []byte) (*Object, error) {
	if objectKey == nil {
		return nil, fmt.Errorf("transactor: nil object key")
	}
	var obj *Object
	shardID := s.top.ShardForKey(objectKey)
	if shardID == s.top.ShardForNode(s.nodeID) {
		var err error
		// obj, err := s.store.GetObjectByKey(objectKey)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to get object by key from local store: %v", err)
		}
	} else {
		// call a node from another shard to get the object
		return nil, fmt.Errorf("not implemented")
	}

	return obj, nil
}

func (s *Service) Name() string {
	return "transactor"
}

func MakeObjectKeys(ctx context.Context, traces []*Trace) ([]*Object, error) {
	ch := combihash.New()
	out := []*Object{}
	for _, trace := range traces {
		baseKey := []byte{}
		for _, inobj := range trace.InputObjectsKeys {
			baseKey = append(baseKey, inobj...)
		}
		for i, outobj := range trace.OutputObjects {
			ch.Reset()
			key := make([]byte, len(baseKey))
			copy(key, baseKey)
			key = append(key, outobj...)
			index := make([]byte, 4)
			binary.LittleEndian.PutUint32(index, uint32(i))
			key = append(key, index...)
			_, err := ch.Write(key)
			if err != nil {
				return nil, fmt.Errorf("transactor: unable to create hash: %v", err)
			}
			o := &Object{
				Value:  outobj,
				Key:    ch.Digest(),
				Status: ObjectStatus_ACTIVE,
			}
			out = append(out, o)
		}
	}
	return out, nil
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

	return &Service{
		checkers: checkers,
		nodeID:   cfg.NodeID,
		privkey:  privkey,
		top:      cfg.Top,
		store:    cfg.Store,
	}, nil
}

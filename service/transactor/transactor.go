package transactor // import "chainspace.io/prototype/service/transactor"

import (
	"context"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"path"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"

	"chainspace.io/prototype/log"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/sync/errgroup"
)

const (
	badgerStorePath = "/transactor/"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// CheckersMap map contractID to map check procedure name to checker
type CheckersMap map[string]map[string]Checker

type Config struct {
	Directory  string
	NodeID     uint64
	Top        *network.Topology
	SigningKey *config.Key
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
	store    *badger.DB
}

func (s *Service) Handle(ctx context.Context, peerID uint64, m *service.Message) (*service.Message, error) {
	switch Opcode(m.Opcode) {
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
		res := &QueryObjectResponse{
			Object: obj,
		}
		b, err := proto.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("transactor: unable to marshal query_object response")
		}
		return &service.Message{Opcode: uint32(Opcode_QUERY_OBJECT), Payload: b}, nil

	default:
		return nil, fmt.Errorf("transactor: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) addTransaction(ctx context.Context, tx *Transaction, rawtx []byte) ([]*ObjectTraceIDPair, error) {
	ok, err := runCheckers(ctx, s.checkers, tx)
	if err != nil {
		return nil, fmt.Errorf("transactor: errors happend while running the checkers: %v", err)
	}
	if !ok {
		return nil, errors.New("transactor: some checks were not successful")
	}

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
			result := c.Check(t.InputObjectsKeys, t.InputReferencesKeys, t.Parameters, t.OutputObjects, t.Returns, t.Dependencies)
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
		/*		var err error
				value, status, err := s.store.GetByKey(objectKey)
				if err != nil {
					return nil, fmt.Errorf("transactor: unable to get object by key from local store: %v", err)
				}

				obj = &Object{
					Value:  value,
					Key:    objectKey,
					Status: ObjectStatus(status),
				} */
	} else {
		// call a node from another shard to get the object
		return nil, fmt.Errorf("not implemented")
	}

	return obj, nil
}

func (s *Service) Name() string {
	return "transactor"
}

func (s *Service) Stop() error {
	return s.store.Close()
}

// TraceIdentifierPair is a pair of a trace and it's identifier
type TraceIdentifierPair struct {
	ID    []byte
	Trace *Trace
}

// TraceOutputObjectIDPair is composed of a trace and the list of output object IDs it create
// ordered in the same order than the orginal output objects
type TraceObjectPair struct {
	OutputObjects []*Object
	Trace         TraceIdentifierPair
}

// MakeTraceIDs generate trace IDs for all traces in the given list
func MakeTraceIDs(traces []*Trace) ([]TraceIdentifierPair, error) {
	out := make([]TraceIdentifierPair, 0, len(traces))
	for _, trace := range traces {
		trace := trace
		id, err := MakeTraceID(trace)
		if err != nil {
			return nil, err
		}
		p := TraceIdentifierPair{
			ID:    id,
			Trace: trace,
		}
		out = append(out, p)
	}

	return out, nil
}

// MakeTraceID generate an identifier for the given trace
// the ID is composed of: the contract ID, the procedure, input objects keys, input
// reference keys, trace ID of the dependencies
func MakeTraceID(trace *Trace) ([]byte, error) {
	ch := combihash.New()
	data := []byte{}
	data = append(data, []byte(trace.ContractID)...)
	data = append(data, []byte(trace.Procedure)...)
	for _, v := range trace.InputObjectsKeys {
		data = append(data, v...)
	}
	for _, v := range trace.InputReferencesKeys {
		data = append(data, v...)
	}
	for _, v := range trace.Dependencies {
		v := v
		id, err := MakeTraceID(v)
		if err != nil {
			return nil, err
		}
		data = append(data, id...)
	}
	_, err := ch.Write(data)
	if err != nil {
		return nil, fmt.Errorf("transactor: unable to create hash: %v", err)
	}

	return ch.Digest(), nil
}

// MakeTraceObjectIDs create a list of Objects based on the Trace / Trace ID input
// Objects are ordered the same as the output objects of the trace
func MakeObjectIDs(pair *TraceIdentifierPair) ([]*Object, error) {
	ch := combihash.New()
	out := []*Object{}
	for i, outobj := range pair.Trace.OutputObjects {
		ch.Reset()
		id := make([]byte, len(pair.ID))
		copy(id, pair.ID)
		id = append(id, outobj...)
		index := make([]byte, 4)
		binary.LittleEndian.PutUint32(index, uint32(i))
		id = append(id, index...)
		_, err := ch.Write(id)
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
	return out, nil
}

// MakeObjectID create a list of Object based on the traces / traces identifier
func MakeTraceObjectPairs(traces []TraceIdentifierPair) ([]TraceObjectPair, error) {
	out := []TraceObjectPair{}
	for _, trace := range traces {
		objs, err := MakeObjectIDs(&trace)
		if err != nil {
			return nil, err
		}
		pair := TraceObjectPair{
			OutputObjects: objs,
			Trace:         trace,
		}
		out = append(out, pair)
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

	opts := badger.DefaultOptions
	badgerPath := path.Join(cfg.Directory, badgerStorePath)
	opts.Dir = badgerPath
	opts.ValueDir = badgerPath
	store, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &Service{
		checkers: checkers,
		nodeID:   cfg.NodeID,
		privkey:  privkey,
		top:      cfg.Top,
		store:    store,
	}, nil
}

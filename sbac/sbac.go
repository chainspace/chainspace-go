package sbac // import "chainspace.io/prototype/sbac"

import (
	"context"
	"crypto/sha512"
	"encoding/base32"
	"errors"
	"fmt"

	"chainspace.io/prototype/broadcast"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/conns"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/pubsub"
	"chainspace.io/prototype/service"

	"github.com/gogo/protobuf/proto"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type Config struct {
	Broadcaster broadcast.Broadcaster
	Directory   string
	KVStore     kv.Service
	NodeID      uint64
	Top         network.NetTopology
	SigningKey  *config.Key
	Pubsub      pubsub.Server
	ShardSize   uint64
	ShardCount  uint64
	MaxPayload  int
	Key         signature.KeyPair
}

type Service struct {
	broadcaster broadcast.Broadcaster
	conns       conns.Pool
	kvstore     kv.Service
	nodeID      uint64
	pe          *pendingEvents
	privkey     signature.PrivateKey
	store       Store
	ps          pubsub.Server
	shardCount  uint64
	shardID     uint64
	shardSize   uint64
	table       *StateTable
	top         network.NetTopology
	txstates    *StateMachineScheduler
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
			tx := ConsensusTransaction{}
			err := proto.Unmarshal(it.TxData, &tx)
			if err != nil {
				log.Error("Unable to unmarshal transaction data", fld.Err(err))
				continue
			}
			e := NewConsensusEvent(&tx)
			s.checkEvent(e)
			s.pe.OnEvent(e)
		}
	}
}

// checkEvent check the event Op and create a new StateMachine if
// the OpCode is Consensus1 or ConsensusCommit
func (s *Service) checkEvent(e *ConsensusEvent) {
	if e.data.Op == ConsensusOp_Consensus1 {
		txbytes, _ := proto.Marshal(e.data.Tx)
		detail := DetailTx{
			ID:        e.data.TxID,
			RawTx:     txbytes,
			Tx:        e.data.Tx,
			Evidences: e.data.Evidences,
			HashID:    ID(e.data.TxID),
		}
		s.txstates.GetOrCreate(&detail, StateWaitingForConsensus1)
	}
}

func (s *Service) Handle(peerID uint64, m *service.Message) (*service.Message, error) {
	ctx := context.TODO()
	switch Opcode(m.Opcode) {
	case Opcode_ADD_TRANSACTION:
		return s.handleAddTransaction(ctx, m.Payload, m.ID)
	case Opcode_QUERY_OBJECT:
		return s.handleQueryObject(ctx, m.Payload, m.ID)
	case Opcode_CREATE_OBJECT:
		return s.handleCreateObject(ctx, m.Payload, m.ID)
	case Opcode_STATES:
		return s.handleStates(ctx, m.Payload, m.ID)
	case Opcode_SBAC:
		return s.handleSBAC(ctx, m.Payload, peerID, m.ID)
	case Opcode_CREATE_OBJECTS:
		return s.handleCreateObjects(ctx, m.Payload, m.ID)
	default:
		log.Error("sbac: unknown message opcode", log.Int32("opcode", m.Opcode), fld.PeerID(peerID), log.Int("len", len(m.Payload)))
		return nil, fmt.Errorf("sbac: unknown message opcode: %v", m.Opcode)
	}
}

func (s *Service) handleSBAC(
	ctx context.Context, payload []byte, peerID uint64, msgID uint64,
) (*service.Message, error) {
	req := &SBACMessage{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("sbac: sbac unmarshaling error", fld.Err(err))
		return nil, fmt.Errorf("sbac: sbac unmarshaling error: %v", err)
	}

	// if we received a COMMIT opcode, the statemachine may not exists
	// lets check and create it here.
	if req.Op == SBACOp_Commit {
		txbytes, _ := proto.Marshal(req.Tx)
		detail := DetailTx{
			ID:        req.TxID,
			RawTx:     txbytes,
			Tx:        req.Tx,
			Evidences: req.Evidences,
			HashID:    ID(req.TxID),
		}
		_ = s.txstates.GetOrCreate(&detail, StateWaitingForCommit)
	}

	e := NewSBACEvent(req)
	s.pe.OnEvent(e)
	res := SBACMessageAck{LastID: msgID}
	payloadres, err := proto.Marshal(&res)
	if err != nil {
		log.Error("unable to marshall handleSBAC response", fld.Err(err))
		return nil, err
	}
	return &service.Message{ID: msgID, Opcode: int32(Opcode_SBAC), Payload: payloadres}, nil
}

func (s *Service) consumeEvents(e Event) bool {
	// check if statemachine is finished
	ok, err := s.store.TxnFinished(e.TxID())
	if err != nil {
		log.Error("error calling TxnFinished", fld.Err(err))
		// do nothing
		return true
	}

	/*if ok {
		log.Error("event for a finished transaction, skipping it",
			fld.TxID(ID(e.TxID())), log.Uint64("peer.id", e.PeerID()))
		// txn already finished. move on
		return true
	}*/

	sm, ok := s.txstates.Get(e.TxID())
	if ok {
		if log.AtDebug() {
			log.Debug("sending new event to statemachine", fld.TxID(ID(e.TxID())), fld.PeerID(e.PeerID()), fld.PeerShard(s.top.ShardForNode(e.PeerID())))
		}
		sm.OnEvent(e)
		return true
	}
	if log.AtDebug() {
		log.Debug("statemachine not ready", fld.TxID(ID(e.TxID())))
	}
	return false
}

// handlers

func (s *Service) handleAddTransaction(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &AddTransactionRequest{}
	err := proto.Unmarshal(payload, req)
	if err != nil {
		log.Error("sbac: unable to unmarshal AddTransaction", fld.Err(err))
		return nil, fmt.Errorf("sbac: add_transaction unmarshaling error: %v", err)
	}

	objects, err := s.AddTransaction(ctx, req.Tx, req.Evidences)
	if err != nil {
		return nil, err
	}

	res := &AddTransactionResponse{
		Objects: objects,
	}

	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("sbac: unable to marshal add_transaction response, %v", err)
	}
	log.Info("sbac: transaction added successfully")
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

func (s *Service) handleQueryObject(
	ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &QueryObjectRequest{}
	err := proto.Unmarshal(payload, req)
	res := &QueryObjectResponse{}
	if err != nil {
		res.Error = fmt.Errorf("sbac: query_object unmarshaling error: %v", err).Error()
		return queryPayload(id, res)
	}

	if req.VersionID == nil {
		res.Error = fmt.Errorf("sbac: nil versionid").Error()
		return queryPayload(id, res)
	}
	objects, err := s.store.GetObjects([][]byte{req.VersionID})
	if err != nil {
		res.Error = err.Error()
	} else if len(objects) != 1 {
		res.Error = fmt.Errorf("sbac: invalid number of objects found, expected %v found %v", 1, len(objects)).Error()
	} else {
		res.Object = objects[0]
	}
	return queryPayload(id, res)
}

func (s *Service) StatesReport(ctx context.Context) *StatesReportResponse {
	return &StatesReportResponse{
		States:        s.txstates.StatesReport(),
		EventsInQueue: int32(s.pe.Len()),
	}
}

func (s *Service) handleStates(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	sr := s.txstates.StatesReport()
	res := &StatesReportResponse{
		States:        sr,
		EventsInQueue: int32(s.pe.Len()),
	}
	b, err := proto.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("sbac: unable to marshal states reports response")
	}
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_STATES),
		Payload: b,
	}, nil
}

func createPayload(id uint64, res *CreateObjectResponse) (*service.Message, error) {
	b, _ := proto.Marshal(res)
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_CREATE_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) handleCreateObject(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &CreateObjectRequest{}
	err := proto.Unmarshal(payload, req)
	res := &CreateObjectResponse{}
	if err != nil {
		res.Error = fmt.Errorf("sbac: new_object unmarshaling error: %v", err).Error()
		return createPayload(id, res)
	}

	if req.Object == nil || len(req.Object) <= 0 {
		res.Error = fmt.Errorf("sbac: nil object").Error()
		return createPayload(id, res)
	}
	hasher := sha512.New512_256()
	hasher.Write(req.Object)
	versionid := hasher.Sum(nil)
	if log.AtDebug() {
		log.Debug("sbac: creating new object", log.String("objet", string(req.Object)), log.Uint32("object.id", ID(versionid)))
	}
	o, err := s.store.CreateObject(versionid, req.Object)
	if err != nil {
		if log.AtDebug() {
			log.Debug("sbac: unable to create object", log.String("objet", string(req.Object)), log.Uint32("object.id", ID(versionid)), fld.Err(err))
		}
		res.Error = err.Error()
	} else {
		res.ID = o.VersionID
	}
	return createPayload(id, res)
}

func createObjectsPayload(id uint64, res *CreateObjectsResponse) (*service.Message, error) {
	b, _ := proto.Marshal(res)
	return &service.Message{
		ID:      id,
		Opcode:  int32(Opcode_CREATE_OBJECT),
		Payload: b,
	}, nil
}

func (s *Service) handleCreateObjects(ctx context.Context, payload []byte, id uint64) (*service.Message, error) {
	req := &CreateObjectsRequest{}
	err := proto.Unmarshal(payload, req)
	res := &CreateObjectsResponse{}
	if err != nil {
		res.Error = fmt.Errorf("sbac: new_object unmarshaling error: %v", err).Error()
		return createObjectsPayload(id, res)
	}

	if req.Objects == nil || len(req.Objects) <= 0 {
		res.Error = fmt.Errorf("sbac: nil object").Error()
		return createObjectsPayload(id, res)
	}

	hasher := sha512.New512_256()
	out := make([][]byte, 0, len(req.Objects))
	for _, object := range req.Objects {
		hasher.Reset()
		hasher.Write(object)
		versionid := hasher.Sum(nil)
		o, err := s.store.CreateObject(versionid, object)
		if err != nil {
			res.Error = err.Error()
			break
		}
		out = append(out, o.VersionID)
	}
	if len(res.Error) <= 0 {
		res.IDs = out
	}
	return createObjectsPayload(id, res)
}

// direct methods

func (s *Service) AddTransaction(
	ctx context.Context, tx *Transaction, evidences map[uint64][]byte,
) ([]*Object, error) {
	ids, err := MakeIDs(tx)
	if err != nil {
		log.Error("unable to create IDs", fld.Err(err))
		return nil, err
	}

	txbytes, _ := proto.Marshal(tx)
	if !s.verifyEvidenceSignature(txbytes, evidences) {
		log.Error("invalid evidences from nodes")
		return nil, errors.New("sbac: invalid evidences from nodes")
	}
	log.Info("sbac: all evidence verified with success")

	objects := []*Object{}
	for _, v := range ids.TraceObjectPairs {
		objects = append(objects, v.OutputObjects...)
	}

	detail := DetailTx{
		ID:        ids.TxID,
		RawTx:     txbytes,
		Tx:        tx,
		Evidences: evidences,
		HashID:    ID(ids.TxID),
	}
	s.txstates.Add(&detail, StateWaitingForConsensus1)

	// broadcast transaction
	consensusTx := &ConsensusTransaction{
		TxID:      ids.TxID,
		Tx:        tx,
		Evidences: evidences,
		Op:        ConsensusOp_Consensus1,
		Initiator: s.nodeID,
	}
	b, err := proto.Marshal(consensusTx)
	if err != nil {
		return nil, fmt.Errorf("sbac: unable to marshal consensus tx: %v", err)
	}
	s.broadcaster.AddTransaction(b, 0)
	log.Info("sbac: transaction added successfully")
	return objects, nil
}

func (s *Service) QueryObjectByVersionID(versionid []byte) ([]byte, error) {
	objects, err := s.store.GetObjects([][]byte{versionid})
	if err != nil {
		return nil, err
	} else if len(objects) != 1 {
		return nil, fmt.Errorf("invalid number of objects found, expected %v found %v", 1, len(objects))
	}
	return objects[0].Value, nil
}

func (s *Service) Name() string {
	return "sbac"
}

func (s *Service) Stop() error {
	return s.store.Close()
}

func New(cfg *Config) (*Service, error) {
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

	bstore, err := NewBadgerStore(cfg.Directory)
	if err != nil {
		return nil, err
	}

	s := &Service{
		broadcaster: cfg.Broadcaster,
		conns:       conns.NewPool(20, cfg.NodeID, cfg.Top, cfg.MaxPayload, cfg.Key, service.CONNECTION_SBAC),
		kvstore:     cfg.KVStore,
		nodeID:      cfg.NodeID,
		privkey:     privkey,
		ps:          cfg.Pubsub,
		top:         cfg.Top,
		store:       bstore,
		shardID:     cfg.Top.ShardForNode(cfg.NodeID),
		shardCount:  cfg.ShardCount,
		shardSize:   cfg.ShardSize,
	}
	s.pe = NewPendingEvents(s.consumeEvents)
	s.table = s.makeStateTable()
	s.broadcaster.Register(s.handleDeliver)
	s.txstates = NewStateMachineScheduler(s.onConsensusEvent, s.onSBACEvent, s.table)

	go s.pe.Run()
	// go s.txstates.RunGC()
	return s, nil
}

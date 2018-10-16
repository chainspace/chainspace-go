package kv

import (
	"fmt"
	"path"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/service"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

const (
	badgerStorePath = "/kvstore/"
)

type Config struct {
	RuntimeDir string
}

type Service struct {
	store *badger.DB
}

func (s *Service) Handle(peerID uint64, m *service.Message) (*service.Message, error) {
	switch Opcode(m.Opcode) {
	case Opcode_GET:
		return s.handleGet(m)
	default:
		log.Error("kvstore: unknown opcode",
			log.Int32("OP", m.Opcode), fld.PeerID(peerID))
		return nil, fmt.Errorf("kvstore: unknown opcode: %v", m.Opcode)
	}
}

func (s *Service) handleGet(m *service.Message) (*service.Message, error) {
	req := &GetRequest{}
	err := proto.Unmarshal(m.Payload, req)
	if err != nil {
		log.Error("KV::handleGet, unable to unmarshal GetRequest", fld.Err(err))
		return nil, err
	}

	if len(req.Key) <= 0 {
		log.Error("KV::handleGet, empty key")
		return nil, fmt.Errorf("empty key")
	}

	value, err := s.Get(req.Key)
	if err != nil {
		return nil, err
	}

	res := &GetResponse{ObjectID: value}
	b, err := proto.Marshal(res)
	if err != nil {
		log.Error("KV::handleGet, unable to marshal response", fld.Err(err))
		return nil, err
	}
	return &service.Message{
		ID:      m.ID,
		Opcode:  int32(Opcode_GET),
		Payload: b,
	}, nil
}

func (s *Service) Get(key []byte) ([]byte, error) {
	var valueout []byte
	err := s.store.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		value, err := item.Value()
		if err != nil {
			return err
		}

		valueout = make([]byte, len(value))
		copy(valueout, value)
		return nil
	})

	if err != nil {
		log.Error("unable to get objectID from key", fld.Err(err))
		return nil, err
	}

	return valueout, nil
}

func (s *Service) Set(key, value []byte) error {
	return s.store.Update(func(txn *badger.Txn) error {
		log.Error("adding new value for key", log.String("key", string(key)))
		return txn.Set(key, value)
	})
}

func New(cfg *Config) (*Service, error) {
	p := path.Join(cfg.RuntimeDir, badgerStorePath)
	opts := badger.DefaultOptions
	opts.Dir, opts.ValueDir = p, p
	store, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &Service{
		store: store,
	}, nil
}

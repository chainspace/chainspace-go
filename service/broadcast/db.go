package broadcast

import (
	"encoding/binary"
	"fmt"

	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

const (
	blockPrefix byte = iota + 1
	currentHashPrefix
	currentRoundPrefix
	ownBlockPrefix
	ownHashPrefix
)

type store struct {
	db *badger.DB
}

func (s *store) getOwnBlock(round uint64) (*SignedData, error) {
	block := &SignedData{}
	key := ownBlockKey(round)
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		if err = proto.Unmarshal(val, block); err != nil {
			return err
		}
		return err
	})
	return block, err
}

func (s *store) getBlock(nodeID uint64, round uint64, hash []byte) (*SignedData, error) {
	block := &SignedData{}
	key := blockKey(nodeID, round, hash)
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		if err = proto.Unmarshal(val, block); err != nil {
			return err
		}
		return err
	})
	return block, err
}

func (s *store) getOwnHash(round uint64) ([]byte, error) {
	var hash []byte
	key := ownHashKey(round)
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		hash = make([]byte, len(val))
		copy(hash, val)
		return nil
	})
	return hash, err
}

func (s *store) getCurrentRoundAndHash() (uint64, []byte, error) {
	var (
		round uint64
		hash  []byte
	)
	hkey := []byte{currentHashPrefix}
	rkey := []byte{currentRoundPrefix}
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(hkey)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		hash = make([]byte, len(val))
		copy(hash, val)
		item, err = txn.Get(rkey)
		if err != nil {
			return err
		}
		val, err = item.Value()
		if err != nil {
			return err
		}
		round = binary.LittleEndian.Uint64(val)
		return nil
	})
	return round, hash, err
}

func (s *store) setBlock(nodeID uint64, round uint64, hash []byte, block *SignedData) error {
	key := blockKey(nodeID, round, hash)
	val, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (s *store) setOwnBlock(nodeID uint64, round uint64, hash []byte, block *SignedData) error {
	bkey := ownBlockKey(round)
	hkey := ownHashKey(round)
	rkey := blockKey(nodeID, round, hash)
	val, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(bkey, val); err != nil {
			return err
		}
		if err := txn.Set(rkey, val); err != nil {
			return err
		}
		return txn.Set(hkey, hash)
	})
}

func blockKey(nodeID uint64, round uint64, hash []byte) []byte {
	key := []byte{blockPrefix}
	key = append(key, fmt.Sprintf("%20d", nodeID)...)
	key = append(key, fmt.Sprintf("%20d", round)...)
	return append(key, hash...)
}

func ownBlockKey(round uint64) []byte {
	key := []byte{ownBlockPrefix}
	return append(key, fmt.Sprintf("%20d", round)...)
}

func ownHashKey(round uint64) []byte {
	key := []byte{ownHashPrefix}
	return append(key, fmt.Sprintf("%20d", round)...)
}

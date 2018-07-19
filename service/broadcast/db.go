package broadcast

import (
	"encoding/binary"
	"fmt"

	"chainspace.io/prototype/byzco"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

const (
	blockPrefix byte = iota + 1
	currentHashPrefix
	currentRoundPrefix
	lastInterpretedPrefix
	ownBlockPrefix
	ownHashPrefix
	roundAcknowledgedPrefix
	roundBlocksPrefix
	sentMapPrefix
)

type store struct {
	db *badger.DB
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
		return proto.Unmarshal(val, block)
	})
	return block, err
}

func (s *store) getBlocks(refs []byzco.BlockID) ([]*SignedData, error) {
	keys := make([][]byte, len(refs))
	for idx, ref := range refs {
		keys[idx] = blockKey(ref.NodeID, ref.Round, []byte(ref.Hash))
	}
	blocks := make([]*SignedData, len(refs))
	err := s.db.View(func(txn *badger.Txn) error {
		for idx, key := range keys {
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			val, err := item.Value()
			if err != nil {
				return err
			}
			block := &SignedData{}
			if err = proto.Unmarshal(val, block); err != nil {
				return err
			}
			blocks[idx] = block
		}
		return nil
	})
	return blocks, err
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

func (s *store) getLastInterpreted() (uint64, error) {
	var round uint64
	key := []byte{lastInterpretedPrefix}
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		round = binary.LittleEndian.Uint64(val)
		return nil
	})
	return round, err
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

func (s *store) getRoundAcknowledged() (uint64, error) {
	var round uint64
	key := []byte{roundAcknowledgedPrefix}
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		round = binary.LittleEndian.Uint64(val)
		return nil
	})
	return round, err
}

func (s *store) getRoundBlocks(round uint64) ([]*SignedData, error) {
	var blocks []*SignedData
	key := roundBlocksKey(round)
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		size := int(binary.LittleEndian.Uint64(val[:8]))
		idx := 8
		for i := 0; i < size; i++ {
			ksize := int(binary.LittleEndian.Uint16(val[idx : idx+2]))
			key = val[idx+2 : idx+2+ksize]
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			val, err := item.Value()
			if err != nil {
				return err
			}
			block := &SignedData{}
			if err = proto.Unmarshal(val, block); err != nil {
				return err
			}
			blocks = append(blocks, block)
			idx += 2 + ksize
		}
		return nil
	})
	return blocks, err
}

func (s *store) getSentMap() (map[uint64]uint64, error) {
	data := map[uint64]uint64{}
	key := []byte{sentMapPrefix}
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		n := int(binary.LittleEndian.Uint64(val[:8]))
		idx := 8
		for i := 0; i < n; i++ {
			k := binary.LittleEndian.Uint64(val[idx : idx+8])
			v := binary.LittleEndian.Uint64(val[idx+8 : idx+16])
			data[k] = v
			idx += 16
		}
		return nil
	})
	return data, err
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

func (s *store) setCurrentRoundAndHash(round uint64, hash []byte) error {
	hkey := []byte{currentHashPrefix}
	rkey := []byte{currentRoundPrefix}
	val := make([]byte, 8)
	binary.LittleEndian.PutUint64(val, round)
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(hkey, hash); err != nil {
			return err
		}
		return txn.Set(rkey, val)
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

func (s *store) setRoundAcknowledged(round uint64) error {
	key := []byte{roundAcknowledgedPrefix}
	val := make([]byte, 8)
	binary.LittleEndian.PutUint64(val, round)
	return s.db.Update(func(txn *badger.Txn) error {
		// TODO(tav): Should check-and-set to not overwrite later rounds?
		return txn.Set(key, val)
	})
}

func (s *store) setRoundBlocks(round uint64, blocks []byzco.BlockID) error {
	size := 8
	keys := make([][]byte, len(blocks))
	for i, b := range blocks {
		key := blockKey(b.NodeID, b.Round, []byte(b.Hash))
		size += 2 + len(key)
		keys[i] = key
	}
	enc := make([]byte, size)
	idx := 8
	binary.LittleEndian.PutUint64(enc[:8], uint64(len(blocks)))
	for _, key := range keys {
		// Assume the length of the block key will fit into a uint16.
		binary.LittleEndian.PutUint16(enc[idx:idx+2], uint16(len(key)))
		copy(enc[idx+2:], key)
		idx += 2 + len(key)
	}
	rkey := roundBlocksKey(round)
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(rkey, enc)
	})
}

func (s *store) setSentMap(data map[uint64]uint64) error {
	enc := make([]byte, 8+(len(data)*16))
	binary.LittleEndian.PutUint64(enc[:8], uint64(len(data)))
	idx := 8
	for k, v := range data {
		binary.LittleEndian.PutUint64(enc[idx:idx+8], k)
		binary.LittleEndian.PutUint64(enc[idx+8:idx+16], v)
		idx += 16
	}
	key := []byte{sentMapPrefix}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, enc)
	})
}

func blockKey(nodeID uint64, round uint64, hash []byte) []byte {
	key := []byte{blockPrefix}
	key = append(key, sortableUint64(nodeID)...)
	key = append(key, sortableUint64(round)...)
	return append(key, hash...)
}

func ownBlockKey(round uint64) []byte {
	key := []byte{ownBlockPrefix}
	return append(key, sortableUint64(round)...)
}

func ownHashKey(round uint64) []byte {
	key := []byte{ownHashPrefix}
	return append(key, sortableUint64(round)...)
}

func roundBlocksKey(round uint64) []byte {
	key := []byte{roundBlocksPrefix}
	return append(key, sortableUint64(round)...)
}

func sortableUint64(v uint64) []byte {
	return []byte(fmt.Sprintf("%20d", v))
}

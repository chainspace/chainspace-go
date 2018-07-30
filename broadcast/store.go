package broadcast

import (
	"encoding/binary"

	"chainspace.io/prototype/byzco"
	"chainspace.io/prototype/lexinum"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

const (
	blockPrefix byte = iota + 1
	currentHashPrefix
	currentRoundPrefix
	includedPrefix
	interpretedPrefix
	lastInterpretedPrefix
	notIncludedPrefix
	ownBlockPrefix
	receivedMapPrefix
	roundAcknowledgedPrefix
	sentMapPrefix
)

type store struct {
	db    *badger.DB
	nodes int
}

func (s *store) getBlock(id byzco.BlockID) (*SignedData, error) {
	block := &SignedData{}
	key := blockKey(id)
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

func (s *store) getBlocks(ids []byzco.BlockID) ([]*SignedData, error) {
	keys := make([][]byte, len(ids))
	for i, id := range ids {
		keys[i] = blockKey(id)
	}
	blocks := make([]*SignedData, len(ids))
	err := s.db.View(func(txn *badger.Txn) error {
		for i, key := range keys {
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
			blocks[i] = block
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

func (s *store) getDeliverAcknowledged() (uint64, error) {
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

func (s *store) getInterpreted(round uint64) ([]*SignedData, error) {
	var blocks []*SignedData
	key := interpretedKey(round)
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

func (s *store) getMissing(nodeID uint64, since uint64, limit int) ([]uint64, uint64, error) {
	var (
		rounds []uint64
	)
	added := 0
	latest := since
	prefix := append([]byte{blockPrefix}, lexinum.Encode(nodeID)...)
	prefix = append(prefix, 0x00)
	start := make([]byte, len(prefix))
	copy(start, prefix)
	start = append(start, lexinum.Encode(since+1)...)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 20
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(start); it.ValidForPrefix(prefix); it.Next() {
			next, err := decodeBlockRound(it.Item().Key())
			if err != nil {
				return err
			}
			for i := latest + 1; i < next; i++ {
				latest = i
				rounds = append(rounds, i)
				if added == limit {
					return nil
				}
				added++
			}
			latest = next
		}
		return nil
	})
	return rounds, latest, err
}

func (s *store) getNotIncluded() ([]*blockData, error) {
	return nil, nil
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

// getRoundBlocks returns all seen blocks for a given round.
func (s *store) getRoundBlocks(round uint64) (map[byzco.BlockID]*Block, error) {
	data := map[byzco.BlockID]*Block{}
	prefix := append([]byte{blockPrefix}, lexinum.Encode(round)...)
	err := s.db.View(func(txn *badger.Txn) error {
		_ = prefix
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = s.nodes
		it := txn.NewIterator(opts)
		defer it.Close()
		signed := &SignedData{}
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			val, err := item.Value()
			if err != nil {
				return err
			}
			nodeID, err := lexinum.Decode(key[21:])
			if err != nil {
				return err
			}
			ref := byzco.BlockID{
				Hash:   string(key[41:]),
				NodeID: nodeID,
				Round:  round,
			}
			if err := proto.Unmarshal(val, signed); err != nil {
				return err
			}
			block := &Block{}
			if err := proto.Unmarshal(signed.Data, block); err != nil {
				return err
			}
			data[ref] = block
		}
		return nil
	})
	return data, err
}

func (s *store) getReceivedMap() (map[uint64]receivedInfo, error) {
	data := map[uint64]receivedInfo{}
	key := []byte{receivedMapPrefix}
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
			latest := binary.LittleEndian.Uint64(val[idx+8 : idx+16])
			sequence := binary.LittleEndian.Uint64(val[idx+16 : idx+24])
			data[k] = receivedInfo{latest, sequence}
			idx += 24
		}
		return nil
	})
	return data, err
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

func (s *store) isIncluded(id byzco.BlockID) (bool, error) {
	key := includedKey(id)
	key[0] = includedPrefix
	var inc bool
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		if val[0] == 1 {
			inc = true
		}
		return nil
	})
	return inc, err
}

func (s *store) setBlock(id byzco.BlockID, block *SignedData) error {
	key := blockKey(id)
	val, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	ikey := includedKey(id)
	nkey := notIncludedKey(id)
	return s.db.Update(func(txn *badger.Txn) error {
		// Since we go no error on getting the included key, the block has been
		// set already.
		if _, err := txn.Get(ikey); err == nil {
			return nil
		}
		if err != badger.ErrKeyNotFound {
			return err
		}
		if err = txn.Set(ikey, []byte{0}); err != nil {
			return err
		}
		if err = txn.Set(nkey, []byte{}); err != nil {
			return err
		}
		return txn.Set(key, val)
	})
}

func (s *store) setDeliverAcknowledged(round uint64) error {
	key := []byte{roundAcknowledgedPrefix}
	val := make([]byte, 8)
	binary.LittleEndian.PutUint64(val, round)
	return s.db.Update(func(txn *badger.Txn) error {
		// TODO(tav): Should check-and-set to not overwrite later rounds?
		return txn.Set(key, val)
	})
}

func (s *store) setInterpreted(data *byzco.Interpreted) error {
	size := 8
	keys := make([][]byte, len(data.Blocks))
	for i, id := range data.Blocks {
		key := blockKey(id)
		size += 2 + len(key)
		keys[i] = key
	}
	enc := make([]byte, size)
	idx := 8
	binary.LittleEndian.PutUint64(enc[:8], uint64(len(data.Blocks)))
	for _, key := range keys {
		// Assume the length of the block key will fit into a uint16.
		binary.LittleEndian.PutUint16(enc[idx:idx+2], uint16(len(key)))
		copy(enc[idx+2:], key)
		idx += 2 + len(key)
	}
	lkey := []byte{lastInterpretedPrefix}
	lval := make([]byte, 8)
	rkey := interpretedKey(data.Round)
	binary.LittleEndian.PutUint64(lval, data.Round)
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(rkey, enc); err != nil {
			return err
		}
		return txn.Set(lkey, lval)
	})
}

func (s *store) setOwnBlock(id byzco.BlockID, block *SignedData, refs []*blockData) error {
	key := ownBlockKey(id.Round)
	val, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	bkey := blockKey(id)
	hkey := []byte{currentHashPrefix}
	rkey := []byte{currentRoundPrefix}
	rval := make([]byte, 8)
	binary.LittleEndian.PutUint64(rval, id.Round)
	var (
		delkeys [][]byte
		refkeys [][]byte
	)
	if len(refs) > 0 {
		delkeys = make([][]byte, len(refs))
		refkeys = make([][]byte, len(refs))
		for i, ref := range refs {
			delkeys[i] = notIncludedKey(ref.id)
			refkeys[i] = includedKey(ref.id)
		}
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(bkey, val); err != nil {
			return err
		}
		if err := txn.Set(hkey, []byte(id.Hash)); err != nil {
			return err
		}
		if err := txn.Set(rkey, rval); err != nil {
			return err
		}
		for _, refkey := range refkeys {
			if err := txn.Set(refkey, []byte{1}); err != nil {
				return err
			}
		}
		for _, delkey := range delkeys {
			if err := txn.Delete(delkey); err != nil {
				return err
			}
		}
		return txn.Set(key, val)
	})
}

func (s *store) setReceivedMap(data map[uint64]receivedInfo) error {
	enc := make([]byte, 8+(len(data)*24))
	binary.LittleEndian.PutUint64(enc[:8], uint64(len(data)))
	idx := 8
	for k, v := range data {
		binary.LittleEndian.PutUint64(enc[idx:idx+8], k)
		binary.LittleEndian.PutUint64(enc[idx+8:idx+16], v.latest)
		binary.LittleEndian.PutUint64(enc[idx+16:idx+24], v.sequence)
		idx += 24
	}
	key := []byte{receivedMapPrefix}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, enc)
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

func blockKey(id byzco.BlockID) []byte {
	key := []byte{blockPrefix}
	key = append(key, lexinum.Encode(id.NodeID)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func decodeBlockRound(d []byte) (uint64, error) {
	rstart, rend := 0, 0
	for i, char := range d[1:] {
		if char == 0x00 {
			if rstart == 0 {
				rstart = i + 2
			} else {
				rend = i + 1
				break
			}
		}
	}
	return lexinum.Decode(d[rstart:rend])
}

func includedKey(id byzco.BlockID) []byte {
	key := []byte{includedPrefix}
	key = append(key, lexinum.Encode(id.NodeID)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func interpretedKey(round uint64) []byte {
	key := []byte{interpretedPrefix}
	return append(key, lexinum.Encode(round)...)
}

func notIncludedKey(id byzco.BlockID) []byte {
	key := []byte{includedPrefix}
	key = append(key, lexinum.Encode(id.NodeID)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func ownBlockKey(round uint64) []byte {
	key := []byte{ownBlockPrefix}
	return append(key, lexinum.Encode(round)...)
}

package broadcast

import (
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"chainspace.io/prototype/blockmania"
	"chainspace.io/prototype/internal/lexinum"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
)

const (
	blockPrefix byte = iota + 1
	blockGraphPrefix
	includedPrefix
	interpretedPrefix
	lastBlockRefPrefix
	lastHashPrefix
	lastInterpretedPrefix
	lastRoundPrefix
	notIncludedPrefix
	ownBlockPrefix
	receivedMapPrefix
	roundAcknowledgedPrefix
	sentMapPrefix
)

var errDBClosed = errors.New("broadcast: DB has been closed")

type store struct {
	closed    bool
	db        *badger.DB
	mu        sync.RWMutex
	nodeCount int
}

func (s *store) getBlock(id blockmania.BlockID) (*SignedData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
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

func (s *store) getBlockData(id blockmania.BlockID) (*blockData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
	signed, err := s.getBlock(id)
	if err != nil {
		return nil, err
	}
	if signed == nil {
		return nil, nil
	}
	return getBlockData(signed), nil
}

func (s *store) getBlockGraphs(nodeID uint64, from uint64, limit int) ([]*blockmania.BlockGraph, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
	var blocks []*blockmania.BlockGraph
	prefix := append([]byte{blockGraphPrefix}, lexinum.Encode(nodeID)...)
	prefix = append(prefix, 0x00)
	start := make([]byte, len(prefix))
	copy(start, prefix)
	start = append(start, lexinum.Encode(from)...)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 20
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(start); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			id, err := decodeBlockID(item.Key()[1:])
			if err != nil {
				return err
			}
			block := &blockmania.BlockGraph{
				Block: id,
			}
			val, err := item.Value()
			if err != nil {
				return err
			}
			ksize := int(binary.LittleEndian.Uint16(val))
			idx := 2
			if ksize > 0 {
				id, err := decodeBlockID(val[idx : idx+ksize])
				if err != nil {
					return err
				}
				block.Prev = id
				idx += ksize
			}
			size := int(binary.LittleEndian.Uint32(val[idx:]))
			idx += 4
			deps := make([]blockmania.Dep, size)
			for i := 0; i < size; i++ {
				ksize := int(binary.LittleEndian.Uint16(val[idx:]))
				idx += 2
				id, err := decodeBlockID(val[idx : idx+ksize])
				if err != nil {
					return err
				}
				idx += ksize
				ksize = int(binary.LittleEndian.Uint16(val[idx:]))
				idx += 2
				var prev blockmania.BlockID
				if ksize > 0 {
					prev, err = decodeBlockID(val[idx : idx+ksize])
					if err != nil {
						return err
					}
					idx += ksize
				}
				lsize := int(binary.LittleEndian.Uint32(val[idx:]))
				idx += 4
				links := make([]blockmania.BlockID, lsize)
				for j := 0; j < lsize; j++ {
					ksize := int(binary.LittleEndian.Uint16(val[idx:]))
					idx += 2
					link, err := decodeBlockID(val[idx : idx+ksize])
					if err != nil {
						return err
					}
					idx += ksize
					links[j] = link
				}
				deps[i] = blockmania.Dep{
					Block: id,
					Deps:  links,
					Prev:  prev,
				}
			}
			block.Deps = deps
			blocks = append(blocks, block)
			if len(blocks) == limit {
				break
			}
		}
		return nil
	})
	return blocks, err
}

func (s *store) getBlocks(ids []blockmania.BlockID) ([]*SignedData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
	keys := make([][]byte, len(ids))
	for i, id := range ids {
		keys[i] = blockKey(id)
	}
	blocks := make([]*SignedData, len(ids))
	err := s.db.View(func(txn *badger.Txn) error {
		for i, key := range keys {
			item, err := txn.Get(key)
			if err != nil {
				if log.AtError() {
					log.Error("Couldn't fetch block", fld.BlockID(ids[i]),
						log.String("fullhash", fmt.Sprintf("%X", ids[i].Hash)), fld.Err(err))
				}
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

func (s *store) getDeliverAcknowledged() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0, errDBClosed
	}
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
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

func (s *store) getLastInterpreted() (round uint64, consumed uint64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0, 0, errDBClosed
	}
	key := []byte{lastInterpretedPrefix}
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.Value()
		if err != nil {
			return err
		}
		round = binary.LittleEndian.Uint64(val[:8])
		consumed = binary.LittleEndian.Uint64(val[8:16])
		return nil
	})
	return round, consumed, err
}

func (s *store) getLastRoundData() (round uint64, hash []byte, blockRef *SignedData, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0, nil, nil, errDBClosed
	}
	bkey := []byte{lastBlockRefPrefix}
	hkey := []byte{lastHashPrefix}
	rkey := []byte{lastRoundPrefix}
	err = s.db.View(func(txn *badger.Txn) error {
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
		item, err = txn.Get(bkey)
		if err != nil {
			return err
		}
		val, err = item.Value()
		if err != nil {
			return err
		}
		blockRef = &SignedData{}
		if err = proto.Unmarshal(val, blockRef); err != nil {
			return err
		}
		return nil
	})
	return round, hash, blockRef, err
}

func (s *store) getMissing(nodeID uint64, since uint64, limit int) ([]uint64, uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, 0, errDBClosed
	}
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
	var blocks []*SignedData
	start := []byte{notIncludedPrefix}
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(start); it.ValidForPrefix(start); it.Next() {
			key := append([]byte{blockPrefix}, it.Item().Key()[1:]...)
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
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	data := make([]*blockData, len(blocks))
	for i, block := range blocks {
		data[i] = getBlockData(block)
	}
	return data, nil
}

func (s *store) getOwnBlock(round uint64) (*SignedData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
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

func (s *store) getReceivedMap() (map[uint64]receivedInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
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

// getRoundBlocks returns all seen blocks for a given round.
func (s *store) getRoundBlocks(round uint64) (map[blockmania.BlockID]*Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
	data := map[blockmania.BlockID]*Block{}
	prefix := append([]byte{blockPrefix}, lexinum.Encode(round)...)
	err := s.db.View(func(txn *badger.Txn) error {
		_ = prefix
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = s.nodeCount
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
			ref := blockmania.BlockID{
				Hash:  string(key[41:]),
				Node:  nodeID,
				Round: round,
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

func (s *store) getSentMap() (map[uint64]uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errDBClosed
	}
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

func (s *store) isIncluded(id blockmania.BlockID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return false, errDBClosed
	}
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

func (s *store) setBlock(id blockmania.BlockID, block *SignedData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDBClosed
	}
	key := blockKey(id)
	val, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	ikey := includedKey(id)
	nkey := notIncludedKey(id)
	err = s.db.Update(func(txn *badger.Txn) error {
		_, err := txn.Get(ikey)
		if err == nil {
			// Since we got no error on getting the included key, the block has
			// been set already.
			if log.AtDebug() {
				log.Debug("Block has seemingly already been set", fld.BlockID(id))
			}
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
		if log.AtDebug() {
			log.Debug("Writing block to DB", fld.BlockID(id),
				log.String("fullhash", fmt.Sprintf("%X", id.Hash)))
		}
		return txn.Set(key, val)
	})
	return err
}

func (s *store) setDeliverAcknowledged(round uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDBClosed
	}
	key := []byte{roundAcknowledgedPrefix}
	val := make([]byte, 8)
	binary.LittleEndian.PutUint64(val, round)
	return s.db.Update(func(txn *badger.Txn) error {
		// TODO(tav): Should check-and-set to not overwrite later rounds?
		return txn.Set(key, val)
	})
}

func (s *store) setInterpreted(data *blockmania.Interpreted) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDBClosed
	}
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
	lastKey := []byte{lastInterpretedPrefix}
	lastVal := make([]byte, 16)
	binary.LittleEndian.PutUint64(lastVal, data.Round)
	binary.LittleEndian.PutUint64(lastVal[8:], data.Consumed)
	roundKey := interpretedKey(data.Round)
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(roundKey, enc); err != nil {
			return err
		}
		return txn.Set(lastKey, lastVal)
	})
}

func (s *store) setOwnBlock(block *SignedData, blockRef *SignedData, graph *blockmania.BlockGraph) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDBClosed
	}
	key := ownBlockKey(graph.Block.Round)
	val, err := proto.Marshal(block)
	if err != nil {
		return err
	}
	blokKey := blockKey(graph.Block)
	blockRefKey := []byte{lastBlockRefPrefix}
	blockRefVal, err := proto.Marshal(blockRef)
	if err != nil {
		return err
	}
	graphKey := blockGraphKey(graph.Block)
	graphVal := encodeBlockGraph(graph)
	hashKey := []byte{lastHashPrefix}
	roundKey := []byte{lastRoundPrefix}
	roundVal := make([]byte, 8)
	binary.LittleEndian.PutUint64(roundVal, graph.Block.Round)
	var (
		delkeys [][]byte
		refkeys [][]byte
	)
	if len(graph.Deps) > 0 {
		delkeys = make([][]byte, len(graph.Deps))
		refkeys = make([][]byte, len(graph.Deps))
		for i, ref := range graph.Deps {
			delkeys[i] = notIncludedKey(ref.Block)
			refkeys[i] = includedKey(ref.Block)
		}
	}
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(blokKey, val); err != nil {
			return err
		}
		if err := txn.Set(blockRefKey, blockRefVal); err != nil {
			return err
		}
		if err := txn.Set(graphKey, graphVal); err != nil {
			return err
		}
		if err := txn.Set(hashKey, []byte(graph.Block.Hash)); err != nil {
			return err
		}
		if err := txn.Set(roundKey, roundVal); err != nil {
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
		if log.AtDebug() {
			log.Debug("Writing own block to DB", fld.BlockID(graph.Block),
				log.String("fullhash", fmt.Sprintf("%X", graph.Block.Hash)))
		}
		return txn.Set(key, val)
	})
}

func (s *store) setReceivedMap(data map[uint64]receivedInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDBClosed
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errDBClosed
	}
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

func blockGraphKey(id blockmania.BlockID) []byte {
	key := []byte{blockGraphPrefix}
	key = append(key, lexinum.Encode(id.Node)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func blockKey(id blockmania.BlockID) []byte {
	key := []byte{blockPrefix}
	key = append(key, lexinum.Encode(id.Node)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func decodeBlockID(d []byte) (blockmania.BlockID, error) {
	var (
		err    error
		hash   string
		nodeID uint64
		round  uint64
		rstart int
		stage  int
	)
outer:
	for i, char := range d {
		if char == 0x00 {
			switch stage {
			case 0:
				nodeID, err = lexinum.Decode(d[:i])
				if err != nil {
					return blockmania.BlockID{}, err
				}
				rstart = i + 1
				stage++
			case 1:
				round, err = lexinum.Decode(d[rstart:i])
				if err != nil {
					return blockmania.BlockID{}, err
				}
				hash = string(d[i+1 : len(d)])
				break outer
			}
		}
	}
	return blockmania.BlockID{
		Hash:  hash,
		Node:  nodeID,
		Round: round,
	}, nil
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

func encodeBlockGraph(block *blockmania.BlockGraph) []byte {
	kbuf := make([]byte, 2)
	sbuf := make([]byte, 4)
	var out []byte
	if block.Prev.Valid() {
		id := encodeBlockID(block.Prev)
		binary.LittleEndian.PutUint16(kbuf, uint16(len(id)))
		out = append(out, kbuf...)
		out = append(out, id...)
	} else {
		out = append(out, 0x00, 0x00)
	}
	size := len(block.Deps)
	binary.LittleEndian.PutUint32(sbuf, uint32(size))
	out = append(out, sbuf...)
	for i := 0; i < size; i++ {
		dep := block.Deps[i]
		id := encodeBlockID(dep.Block)
		binary.LittleEndian.PutUint16(kbuf, uint16(len(id)))
		out = append(out, kbuf...)
		out = append(out, id...)
		if dep.Prev.Valid() {
			id := encodeBlockID(dep.Prev)
			binary.LittleEndian.PutUint16(kbuf, uint16(len(id)))
			out = append(out, kbuf...)
			out = append(out, id...)
		} else {
			out = append(out, 0x00, 0x00)
		}
		lsize := len(dep.Deps)
		binary.LittleEndian.PutUint32(sbuf, uint32(lsize))
		out = append(out, sbuf...)
		for j := 0; j < lsize; j++ {
			id := encodeBlockID(dep.Deps[j])
			binary.LittleEndian.PutUint16(kbuf, uint16(len(id)))
			out = append(out, kbuf...)
			out = append(out, id...)
		}
	}
	return out
}

func encodeBlockID(id blockmania.BlockID) []byte {
	var out []byte
	out = append(out, lexinum.Encode(id.Node)...)
	out = append(out, 0x00)
	out = append(out, lexinum.Encode(id.Round)...)
	out = append(out, 0x00)
	return append(out, id.Hash...)

}

func getBlockData(signed *SignedData) *blockData {
	block := &Block{}
	if err := proto.Unmarshal(signed.Data, block); err != nil {
		log.Fatal("Unable to decode signed block", fld.Err(err))
	}
	hasher := sha512.New512_256()
	if _, err := hasher.Write(signed.Data); err != nil {
		log.Fatal("Unable to hash signed block data", fld.Err(err))
	}
	hash := hasher.Sum(nil)
	ref := &BlockReference{
		Hash:  hash,
		Node:  block.Node,
		Round: block.Round,
	}
	id := blockmania.BlockID{
		Hash:  string(hash),
		Node:  block.Node,
		Round: block.Round,
	}
	enc, err := proto.Marshal(ref)
	if err != nil {
		log.Fatal("Unable to encode block reference", fld.Err(err))
	}
	var prev blockmania.BlockID
	ref = &BlockReference{}
	if block.Round != 1 {
		if err = proto.Unmarshal(block.Previous.Data, ref); err != nil {
			log.Fatal("Unable to decode block's previous reference", fld.Err(err))
		}
		prev = blockmania.BlockID{
			Hash:  string(ref.Hash),
			Node:  ref.Node,
			Round: ref.Round,
		}
	}
	var deps []blockmania.BlockID
	for _, sref := range block.References {
		ref = &BlockReference{}
		if err := proto.Unmarshal(sref.Data, ref); err != nil {
			log.Fatal("Unable to decode block reference", fld.Err(err))
		}
		deps = append(deps, blockmania.BlockID{
			Hash:  string(ref.Hash),
			Node:  ref.Node,
			Round: ref.Round,
		})
	}
	return &blockData{
		deps: deps,
		id:   id,
		prev: prev,
		ref: &SignedData{
			Data:      enc,
			Signature: signed.Signature,
		},
	}
}

func includedKey(id blockmania.BlockID) []byte {
	key := []byte{includedPrefix}
	key = append(key, lexinum.Encode(id.Node)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func interpretedKey(round uint64) []byte {
	key := []byte{interpretedPrefix}
	return append(key, lexinum.Encode(round)...)
}

func notIncludedKey(id blockmania.BlockID) []byte {
	key := []byte{notIncludedPrefix}
	key = append(key, lexinum.Encode(id.Node)...)
	key = append(key, 0x00)
	key = append(key, lexinum.Encode(id.Round)...)
	key = append(key, 0x00)
	return append(key, id.Hash...)
}

func ownBlockKey(round uint64) []byte {
	key := []byte{ownBlockPrefix}
	return append(key, lexinum.Encode(round)...)
}

// Package broadcast implements the network broadcast and consensus within a
// shard.
package broadcast // import "chainspace.io/prototype/broadcast"

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"chainspace.io/prototype/byzco"
	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"github.com/dgraph-io/badger"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var (
	genesis = []byte("genesis.block")
)

// Callback defines the interface for the callback set registered with the
// broadcast service. The DeliverStart method is called as soon as a particular
// round reaches consensus, followed by individual DeliverTransaction calls for
// each of the transactions in consensus order, and finished off with a
// DeliverEnd call for each round. The DeliverEnd method should call Acknowledge
// at some point so as to mark the delivery of the round as successful.
type Callback interface {
	DeliverStart(round uint64)
	DeliverTransaction(txdata *TransactionData)
	DeliverEnd(round uint64)
}

// Config for the broadcast service.
type Config struct {
	BlockLimit     int
	Directory      string
	Key            signature.KeyPair
	Keys           map[uint64]signature.PublicKey
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	MaxPayload     int
	NodeID         uint64
	Peers          []uint64
	ReadTimeout    time.Duration
	RoundInterval  time.Duration
	WriteTimeout   time.Duration
}

// Service implements the broadcast and consensus system.
type Service struct {
	blockLimit     int
	cache          *cache
	cb             Callback
	cond           *sync.Cond
	ctx            context.Context
	dag            *byzco.DAG
	initialBackoff time.Duration
	interpreted    uint64
	interval       time.Duration
	key            signature.KeyPair
	keys           map[uint64]signature.PublicKey
	lru            *lru
	maxBackoff     time.Duration
	maxPayload     int
	mu             sync.RWMutex // protects previous, round, sent, signal
	nodeID         uint64
	peers          []uint64
	previous       []byte
	readTimeout    time.Duration
	refs           chan *SignedData
	round          uint64
	sent           map[uint64]uint64
	signal         chan struct{}
	store          *store
	toDeliver      []*byzco.Interpreted
	top            *network.Topology
	txs            chan *TransactionData
	writeTimeout   time.Duration
}

func (s *Service) dagCallback(data *byzco.Interpreted) {
	s.store.setInterpreted(data)
	s.cond.L.Lock()
	s.mu.RLock()
	if s.cb != nil {
		s.mu.RUnlock()
		s.toDeliver = append(s.toDeliver, data)
		s.cond.L.Unlock()
		s.cond.Signal()
		return
	}
	s.mu.RUnlock()
	s.cond.L.Unlock()
}

func (s *Service) deliver() {
	s.cond.L.Lock()
	ack, err := s.store.getDeliverAcknowledged()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load latest acknowledged round from DB", zap.Error(err))
		}
	}
	latest := s.interpreted
	for i := ack + 1; i <= latest; i++ {
		blocks, err := s.store.getInterpreted(i)
		if err != nil {
			log.Fatal("Unable to load blocks for a round", zap.Uint64("round", i), zap.Error(err))
		}
		s.deliverRound(i, blocks)
	}
	s.cond.L.Unlock()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		s.cond.L.Lock()
		for len(s.toDeliver) == 0 {
			s.cond.Wait()
		}
		data := s.toDeliver[0]
		s.toDeliver = s.toDeliver[1:]
		s.cond.L.Unlock()
		blocks := s.getBlocks(data.Blocks)
		s.deliverRound(data.Round, blocks)
	}
}

func (s *Service) deliverRound(round uint64, blocks []*SignedData) {
	var txlist transactionList
	for _, block := range blocks {
		txs, err := block.Transactions()
		if err != nil {
			log.Fatal("Unable to decode transactions", zap.Uint64("round", round), zap.String("block.hash", fmt.Sprintf("%X", block.Digest())))
		}
		txlist = append(txlist, txs...)
	}
	txlist.Sort()
	s.cb.DeliverStart(round)
	for _, tx := range txlist {
		s.cb.DeliverTransaction(tx)
	}
	s.cb.DeliverEnd(round)
}

func (s *Service) genBlocks() {
	var (
		atLimit     bool
		pendingRefs []*SignedData
		pendingTxs  []*TransactionData
		refs        []*SignedData
		txs         []*TransactionData
	)
	block := &Block{
		Node: s.nodeID,
	}
	hasher := combihash.New()
	rootRef := &BlockReference{
		Node: s.nodeID,
	}
	round := s.round
	// TODO(tav): time.Ticker's behaviour around slow receivers may not be the
	// exact semantics we want.
	tick := time.NewTicker(s.interval)
	total := 0
	for {
		select {
		case ref := <-s.refs:
			if atLimit {
				pendingRefs = append(pendingRefs, ref)
			} else {
				total += ref.Size()
				if total < s.blockLimit {
					refs = append(refs, ref)
				} else {
					atLimit = true
					pendingRefs = append(pendingRefs, ref)
				}
			}
		case tx := <-s.txs:
			if atLimit {
				pendingTxs = append(pendingTxs, tx)
			} else {
				total += tx.Size()
				if total < s.blockLimit {
					txs = append(txs, tx)
				} else {
					atLimit = true
					pendingTxs = append(pendingTxs, tx)
				}
			}
		case <-tick.C:
			round++
			block.Previous = s.previous
			block.References = refs
			block.Round = round
			block.Transactions = txs
			data, err := proto.Marshal(block)
			if err != nil {
				log.Fatal("Got unexpected error encoding latest block", zap.Error(err))
			}
			if _, err := hasher.Write(data); err != nil {
				log.Fatal("Could not hash encoded block", zap.Error(err))
			}
			hash := hasher.Digest()
			rootRef.Hash = hash
			rootRef.Round = round
			refData, err := proto.Marshal(rootRef)
			if err != nil {
				log.Fatal("Got unexpected error encoding latest block reference", zap.Error(err))
			}
			signed := &SignedData{
				Data:      data,
				Signature: s.key.Sign(refData),
			}
			s.previous = hash
			hasher.Reset()
			signal := s.signal
			if err := s.setOwnBlock(round, hash, signed); err != nil {
				log.Fatal("Could not write own block to the DB", zap.Uint64("round", round), zap.Error(err))
			}
			if err := s.store.setCurrentRoundAndHash(round, hash); err != nil {
				log.Fatal("Could not write current round and hash to the DB", zap.Error(err))
			}
			s.mu.Lock()
			s.round = round
			s.signal = make(chan struct{})
			s.mu.Unlock()
			close(signal)
			// TODO(tav): Remove this once it's fully wired together.
			s.dagCallback(&byzco.Interpreted{
				Blocks: []byzco.BlockID{{Hash: string(hash), NodeID: s.nodeID, Round: round}},
				Round:  round,
			})
			log.Info("Created block", zap.Uint64("round", block.Round))
			if round%100 == 0 {
				s.cache.prune(100)
				s.lru.prune(len(s.peers) * 100)
			}
			txs = nil
			total = 0
			if len(pendingTxs) > 0 {
				var (
					npendingRefs []*SignedData
					npendingTxs  []*TransactionData
				)
				for _, ref := range pendingRefs {
					if len(npendingTxs) > 0 {
						npendingRefs = append(npendingRefs, ref)
					} else {
						total += ref.Size()
						if total < s.blockLimit {
							refs = append(refs, ref)
						} else {
							npendingRefs = append(npendingRefs, ref)
						}
					}
				}
				for _, tx := range pendingTxs {
					if len(npendingTxs) > 0 {
						npendingTxs = append(npendingTxs, tx)
					} else {
						total += tx.Size()
						if total < s.blockLimit {
							txs = append(txs, tx)
						} else {
							npendingTxs = append(npendingTxs, tx)
						}
					}
				}
				pendingTxs = npendingTxs
			}
		case <-s.ctx.Done():
			tick.Stop()
			if err := s.store.db.Close(); err != nil {
				log.Error("Could not close the broadcast DB successfully", zap.Error(err))
			}
			return
		}
	}
}

func (s *Service) getBlockInfo(nodeID uint64, round uint64, hash []byte) *blockInfo {
	ref := byzco.BlockID{
		Hash:   string(hash),
		NodeID: nodeID,
		Round:  round,
	}
	info := s.lru.get(ref)
	if info != nil {
		return info
	}
	block, err := s.store.getBlock(ref)
	if err != nil {
		log.Error("Could not retrieve block", zap.Uint64("node.id", nodeID), zap.Uint64("round", round), zap.Error(err))
		return nil
	}
	info = &blockInfo{
		block: block,
		hash:  hash,
	}
	s.lru.set(byzco.BlockID{
		Hash:   string(hash),
		NodeID: nodeID,
		Round:  round,
	}, info)
	return info
}

func (s *Service) getBlocks(refs []byzco.BlockID) []*SignedData {
	var missing []byzco.BlockID
	blocks := make([]*SignedData, len(refs))
	for idx, ref := range refs {
		info := s.lru.get(ref)
		if info == nil {
			missing = append(missing, ref)
		} else {
			blocks[idx] = info.block
		}
	}
	if len(missing) > 0 {
		res, err := s.store.getBlocks(missing)
		if err != nil {
			log.Fatal("Unable to retrieve blocks", zap.Error(err))
		}
		used := 0
		for idx, block := range blocks {
			if block == nil {
				blocks[idx] = res[used]
				used++
			}
		}
	}
	return blocks
}

func (s *Service) getOwnBlock(round uint64) *SignedData {
	block := s.cache.get(round)
	if block != nil {
		return block
	}
	block, err := s.store.getOwnBlock(round)
	if err != nil {
		log.Error("Could not retrieve own block", zap.Uint64("round", round), zap.Error(err))
		return nil
	}
	s.cache.set(round, block)
	return block
}

func (s *Service) getOwnHash(round uint64) []byte {
	hash, err := s.store.getOwnHash(round)
	if err != nil {
		log.Fatal("Got error retrieving own hash", zap.Uint64("round", round), zap.Error(err))
	}
	return hash
}

// getUnsent returns a slice of unsent blocks for the given peer. If it looks
// like the peer is up-to-date, then the call blocks until a new block is
// available.
func (s *Service) getUnsent(peerID uint64) ([]*SignedData, uint64) {
	for {
		s.mu.RLock()
		cur := s.round
		last := s.sent[peerID] + 1
		if cur >= last {
			blocks := []*SignedData{}
			total := 0
			for i := last; i <= cur; i++ {
				block := s.getOwnBlock(i)
				if block == nil {
					log.Fatal("Unable to retrieve own block", zap.Uint64("round", i))
				}
				total += len(block.Data) + len(block.Signature) + 100
				if total > s.maxPayload {
					if i == last {
						log.Fatal("Size of individual block exceeds max payload size", zap.Int("size", total), zap.Int("limit", s.maxPayload))
					}
					s.mu.RUnlock()
					return blocks, i - 1
				}
				blocks = append(blocks, block)
			}
			s.mu.RUnlock()
			return blocks, cur
		}
		signal := s.signal
		s.mu.RUnlock()
		select {
		case <-signal:
		case <-s.ctx.Done():
			return nil, 0
		}
	}
}

func (s *Service) handleBlockList(peerID uint64, msg *service.Message) error {
	listing := &BlockList{}
	if err := proto.Unmarshal(msg.Payload, listing); err != nil {
		return err
	}
	for _, block := range listing.Blocks {
		if err := s.processBlock(block); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) handleGetBlocks(peerID uint64, msg *service.Message) (*service.Message, error) {
	req := &GetBlocks{}
	if err := proto.Unmarshal(msg.Payload, req); err != nil {
		return nil, err
	}
	blocks := make([]*SignedData, len(req.Blocks))
	for idx, ref := range req.Blocks {
		info := s.getBlockInfo(ref.Node, ref.Round, ref.Hash)
		if info == nil {
			blocks[idx] = nil
			log.Error("Got request for unknown block", zap.Uint64("peer.id", peerID), zap.Uint64("round", ref.Round))
			continue
		}
		blocks[idx] = info.block
	}
	data, err := proto.Marshal(&BlockList{
		Blocks: blocks,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  uint32(OP_BLOCK_LIST),
		Payload: data,
	}, nil
}

func (s *Service) handleGetHashes(peerID uint64, msg *service.Message) (*service.Message, error) {
	req := &GetHashes{}
	if err := proto.Unmarshal(msg.Payload, req); err != nil {
		return nil, err
	}
	var hashes [][]byte
	s.mu.RLock()
	latest := s.round
	s.mu.RUnlock()
	total := 0
	if req.Since < latest {
		for i := req.Since + 1; i <= latest; i++ {
			hash := s.getOwnHash(i)
			total += len(hash) + 100
			if total > s.maxPayload {
				break
			}
			hashes = append(hashes, hash)
		}
	}
	data, err := proto.Marshal(&HashList{
		Hashes: hashes,
		Latest: latest,
		Since:  req.Since,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  uint32(OP_HASH_LIST),
		Payload: data,
	}, nil
}

func (s *Service) handleHashList(peerID uint64, msg *service.Message) error {
	return nil
}

func (s *Service) loadState() {
	round, hash, err := s.store.getCurrentRoundAndHash()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load latest round and hash from DB", zap.Error(err))
		}
		hash = genesis
		round = 0
	}
	interpreted, err := s.store.getLastInterpreted()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load last interpreted from DB", zap.Error(err))
		}
		interpreted = 0
	}
	sent, err := s.store.getSentMap()
	if err != nil {
		if err != badger.ErrKeyNotFound {
			log.Fatal("Could not load sent map from DB", zap.Error(err))
		}
		sent = map[uint64]uint64{}
		for _, peer := range s.peers {
			sent[peer] = 0
		}
	}
	log.Info("STARTUP STATE", zap.Uint64("round", round), zap.Uint64("interpreted", interpreted))
	nodes := append([]uint64{s.nodeID}, s.peers...)
	s.dag = byzco.NewDAG(s.ctx, nodes, interpreted, s.dagCallback)
	s.interpreted = interpreted
	s.previous = hash
	s.round = round
	s.sent = sent
	s.signal = make(chan struct{})
	s.replayDAGChanges()
}

func (s *Service) maintainBroadcast(peerID uint64) {
	var (
		blocks []*SignedData
		round  uint64
	)
	backoff := s.initialBackoff
	msg := &service.Message{Opcode: uint32(OP_BLOCK_LIST)}
	listing := &BlockList{}
	retry := false
	for {
		if retry {
			backoff *= 2
			if backoff > s.maxBackoff {
				backoff = s.maxBackoff
			}
			time.Sleep(backoff)
			retry = false
		}
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		conn, err := s.top.Dial(s.ctx, peerID)
		if err == nil {
			backoff = s.initialBackoff
		} else {
			log.Error("Couldn't dial node", zap.Uint64("peer.id", peerID), zap.Error(err))
			retry = true
			continue
		}
		hello, err := service.SignHello(s.nodeID, peerID, s.key, service.CONNECTION_BROADCAST)
		if err != nil {
			log.Error("Couldn't create Hello payload for broadcast", zap.Error(err))
			retry = true
			continue
		}
		if err = conn.WritePayload(hello, s.maxPayload, s.writeTimeout); err != nil {
			log.Error("Couldn't send Hello", zap.Uint64("peer.id", peerID), zap.Error(err))
			retry = true
			continue
		}
		for {
			if len(blocks) == 0 {
				blocks, round = s.getUnsent(peerID)
				if blocks == nil {
					return
				}
			}
			listing.Blocks = blocks
			msg.Payload, err = proto.Marshal(listing)
			if err != nil {
				log.Fatal("Could not encode listing for broadcast", zap.Error(err))
			}
			if err = conn.WritePayload(msg, s.maxPayload, s.writeTimeout); err != nil {
				log.Error("Could not write listing", zap.Uint64("peer.id", peerID), zap.Error(err))
				retry = true
				break
			}
			s.markSent(peerID, round)
			blocks = nil
		}
	}
}

// markSent marks the given round as being sent to the peer in the shard.
func (s *Service) markSent(peerID uint64, round uint64) {
	s.mu.Lock()
	s.sent[peerID] = round
	s.mu.Unlock()
}

func (s *Service) processBlock(signed *SignedData) error {
	block := &Block{}
	if err := proto.Unmarshal(signed.Data, block); err != nil {
		return err
	}
	key, exists := s.keys[block.Node]
	if !exists {
		return fmt.Errorf("broadcast: unable to find signing.key for node %d", block.Node)
	}
	hasher := combihash.New()
	if _, err := hasher.Write(signed.Data); err != nil {
		return fmt.Errorf("broadcast: unable to hash received signed block data: %s", err)
	}
	hash := hasher.Digest()
	ref := &BlockReference{
		Hash:  hash,
		Node:  block.Node,
		Round: block.Round,
	}
	enc, err := proto.Marshal(ref)
	if err != nil {
		return fmt.Errorf("broadcast: unable to encode block reference: %s", err)
	}
	if !key.Verify(enc, signed.Signature) {
		return fmt.Errorf("broadcast: unable to verify signature for block %d from node %d", block.Round, block.Node)
	}
	log.Info("Received block", zap.Uint64("node.id", block.Node), zap.Uint64("round", block.Round))
	return nil
}

func (s *Service) replayDAGChanges() {
	for round := s.interpreted; round < s.round; round++ {
		// block := s.getOwnBlock(round)
	}
}

func (s *Service) setOwnBlock(round uint64, hash []byte, block *SignedData) error {
	ref := byzco.BlockID{
		Hash:   string(hash),
		NodeID: s.nodeID,
		Round:  round,
	}
	s.lru.set(ref, &blockInfo{block, hash})
	s.cache.set(round, block)
	return s.store.setOwnBlock(ref, block)
}

func (s *Service) writeSentMap() {
	ticker := time.NewTicker(20 * s.interval)
	for {
		select {
		case <-ticker.C:
			sent := make(map[uint64]uint64, len(s.peers))
			s.mu.RLock()
			for k, v := range s.sent {
				sent[k] = v
			}
			s.mu.RUnlock()
			s.store.setSentMap(sent)
		case <-s.ctx.Done():
			return
		}
	}
}

// Acknowledge should be called by BroadcastEnd in the registered Callback to
// mark a particular round as being fully delivered. If not, all unacknowledged
// rounds will be replayed when the node restarts.
func (s *Service) Acknowledge(round uint64) {
	s.store.setDeliverAcknowledged(round)
}

// AddTransaction adds the given transaction data onto a queue to be added to
// the current block.
func (s *Service) AddTransaction(txdata *TransactionData) {
	s.txs <- txdata
}

// Handle implements the service Handler interface for handling messages
// received over a connection.
func (s *Service) Handle(ctx context.Context, peerID uint64, msg *service.Message) (*service.Message, error) {
	switch OP(msg.Opcode) {
	case OP_BLOCK_LIST:
		return nil, s.handleBlockList(peerID, msg)
	case OP_GET_BLOCKS:
		return s.handleGetBlocks(peerID, msg)
	case OP_GET_HASHES:
		return s.handleGetHashes(peerID, msg)
	case OP_HASH_LIST:
		return nil, s.handleHashList(peerID, msg)
	default:
		return nil, fmt.Errorf("broadcast: unknown message opcode: %d", msg.Opcode)
	}
}

// Name specifies the name of the service for use in debugging service handlers.
func (s *Service) Name() string {
	return "broadcast"
}

// Register saves the given callback to be called when transactions have reached
// consensus. It'll also trigger the replaying of any unacknowledged rounds.
func (s *Service) Register(cb Callback) {
	s.mu.Lock()
	if s.cb != nil {
		s.mu.Unlock()
		log.Fatal("Attempt to register a broadcast.Callback when one already exists")
	}
	s.cb = cb
	s.mu.Unlock()
	go s.deliver()
}

// New returns a fully instantiated broadcaster service.
func New(ctx context.Context, cfg *Config, top *network.Topology) (*Service, error) {
	opts := badger.DefaultOptions
	opts.Dir = filepath.Join(cfg.Directory, "broadcast")
	opts.ValueDir = opts.Dir
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	cache := &cache{
		data: map[uint64]*SignedData{},
	}
	lru := &lru{
		data: map[byzco.BlockID]*blockInfoContainer{},
	}
	store := &store{
		db:    db,
		nodes: len(cfg.Peers) + 1,
	}
	var mu sync.Mutex
	s := &Service{
		blockLimit:     cfg.BlockLimit,
		cache:          cache,
		cond:           sync.NewCond(&mu),
		ctx:            ctx,
		interval:       cfg.RoundInterval,
		key:            cfg.Key,
		keys:           cfg.Keys,
		initialBackoff: cfg.InitialBackoff,
		lru:            lru,
		maxBackoff:     cfg.MaxBackoff,
		maxPayload:     cfg.MaxPayload,
		nodeID:         cfg.NodeID,
		peers:          cfg.Peers,
		readTimeout:    cfg.ReadTimeout,
		refs:           make(chan *SignedData, 10000),
		store:          store,
		top:            top,
		txs:            make(chan *TransactionData, 10000),
		writeTimeout:   cfg.WriteTimeout,
	}
	s.loadState()
	go s.genBlocks()
	go s.writeSentMap()
	for _, peer := range cfg.Peers {
		go s.maintainBroadcast(peer)
	}
	return s, nil
}

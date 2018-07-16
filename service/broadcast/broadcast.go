// Package broadcast implements the network broadcast and consensus within a
// shard.
package broadcast

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

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
// broadcast service. The BroadcastStart method is called as soon as a
// particular round reaches consensus, followed by individual
// BroadcastTransaction calls for each of the transactions in consensus order,
// and finished off with a BroadcastEnd call for each round. The BroadcastEnd
// method should call Acknowledge at some point so as to mark the delivery of
// the round as successful.
type Callback interface {
	BroadcastStart(round uint64)
	BroadcastTransaction(txdata *TransactionData)
	BroadcastEnd(round uint64)
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
	cb             Callback
	ctx            context.Context
	db             *badger.DB
	initialBackoff time.Duration
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
	refs           chan *BlockReference
	round          uint64
	sent           map[uint64]uint64
	signal         chan struct{}
	top            *network.Topology
	txs            chan *TransactionData
	writeTimeout   time.Duration
}

func (s *Service) genBlocks() {
	var (
		atLimit     bool
		pendingRefs []*BlockReference
		pendingTxs  []*TransactionData
		refs        []*BlockReference
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
			s.setBlock(s.nodeID, round, &blockInfo{
				block: signed,
				hash:  hash,
			})
			s.mu.Lock()
			s.round = round
			s.signal = make(chan struct{})
			s.mu.Unlock()
			close(signal)
			log.Info("Created block", zap.Uint64("round", block.Round))
			if round%100 == 0 {
				s.lru.prune(len(s.peers) * 100)
			}
			txs = nil
			total = 0
			if len(pendingTxs) > 0 {
				var (
					npendingRefs []*BlockReference
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
			if err := s.db.Close(); err != nil {
				log.Error("Could not close the broadcast DB successfully", zap.Error(err))
			}
			return
		}
	}
}

func (s *Service) getBlockInfo(nodeID uint64, round uint64) *blockInfo {
	return s.lru.get(blockPointer{nodeID, round})
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
				info := s.getBlockInfo(s.nodeID, i)
				total += len(info.block.Data) + len(info.block.Signature) + 100
				if total > s.blockLimit {
					if i == last {
						panic(fmt.Sprintf("broadcast: size of individual block(%v) exceeds max payload size(%v)", total, s.blockLimit))
					}
					s.mu.RUnlock()
					return blocks, i - 1
				}
				blocks = append(blocks, info.block)
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

func (s *Service) handleBlocksRequest(peerID uint64, msg *service.Message) (*service.Message, error) {
	req := &BlocksRequest{}
	if err := proto.Unmarshal(msg.Payload, req); err != nil {
		return nil, err
	}
	blocks := make([]*SignedData, len(req.Blocks))
	for idx, ref := range req.Blocks {
		info := s.getBlockInfo(ref.Node, ref.Round)
		if info == nil {
			blocks[idx] = nil
			log.Error("Got request for unknown block", zap.Uint64("peer.id", peerID), zap.Uint64("round", ref.Round))
			continue
		}
		if len(ref.Hash) > 0 {
			if !bytes.Equal(info.hash, ref.Hash) {
				blocks[idx] = nil
				log.Error("Got request for block with a bad hash", zap.Uint64("peer.id", peerID), zap.Uint64("round", ref.Round))
				continue
			}
		}
		blocks[idx] = info.block
	}
	data, err := proto.Marshal(&Listing{
		Blocks: blocks,
	})
	if err != nil {
		return nil, err
	}
	return &service.Message{
		Opcode:  uint32(OP_LISTING),
		Payload: data,
	}, nil
}

func (s *Service) handleListing(peerID uint64, msg *service.Message) error {
	listing := &Listing{}
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

func (s *Service) loadState() {
	// TODO(tav): Load these from a filestore-backed DB.
	s.previous = genesis
	s.round = 0
	s.sent = map[uint64]uint64{}
	for _, peer := range s.peers {
		s.sent[peer] = 0
	}
	s.signal = make(chan struct{})
}

func (s *Service) maintainBroadcast(peerID uint64) {
	var (
		blocks []*SignedData
		round  uint64
	)
	backoff := s.initialBackoff
	msg := &service.Message{Opcode: uint32(OP_LISTING)}
	listing := &Listing{}
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

func (s *Service) setBlock(nodeID uint64, round uint64, info *blockInfo) {
	s.lru.set(blockPointer{nodeID, round}, info)
}

// Acknowledge should be called by BroadcastEnd in the registered Callback to
// mark a particular round as being fully delivered. If not, all unacknowledged
// rounds will be replayed when the node restarts.
func (s *Service) Acknowledge(round uint64) {
}

// AddTransaction adds the given transaction data onto a queue to be added to
// the current block. Register should be used to register a Callback before any
// AddTransaction calls are made.
func (s *Service) AddTransaction(txdata *TransactionData) {
	s.txs <- txdata
}

// Handle implements the service Handler interface for handling messages
// received over a connection.
func (s *Service) Handle(ctx context.Context, peerID uint64, msg *service.Message) (*service.Message, error) {
	switch OP(msg.Opcode) {
	case OP_BLOCKS_REQUEST:
		return s.handleBlocksRequest(peerID, msg)
	case OP_LISTING:
		return nil, s.handleListing(peerID, msg)
	default:
		return nil, fmt.Errorf("broadcast: unknown message opcode: %d", msg.Opcode)
	}
}

// Name specifies the name of the service for use in debugging service handlers.
func (s *Service) Name() string {
	return "broadcast"
}

// Register saves the given callback to be called when transactions have reached
// consensus. Register should be called before any AddTransaction calls.
func (s *Service) Register(cb Callback) {
	s.cb = cb
}

// New returns a fully instantiated broadcaster service. Soon after the service
// is instantiated, Register should be called on it to register a callback,
// before any AddTransaction calls are made.
func New(ctx context.Context, cfg *Config, top *network.Topology) (*Service, error) {
	opts := badger.DefaultOptions
	opts.Dir = filepath.Join(cfg.Directory, "broadcast")
	opts.ValueDir = opts.Dir
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	lru := &lru{
		data: map[blockPointer]*blockInfoContainer{},
	}
	s := &Service{
		blockLimit:     cfg.BlockLimit,
		db:             db,
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
		refs:           make(chan *BlockReference, 10000),
		top:            top,
		txs:            make(chan *TransactionData, 10000),
		writeTimeout:   cfg.WriteTimeout,
	}
	s.loadState()
	go s.genBlocks()
	for _, peer := range cfg.Peers {
		go s.maintainBroadcast(peer)
	}
	return s, nil
}

package transactorclient // import "chainspace.io/prototype/transactor/client"

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/transactor"

	"github.com/gogo/protobuf/proto"
)

// Config represent the configuration required to send messages
// using the transactor
type Config struct {
	Top        *network.Topology
	MaxPayload config.ByteSize
}

type Client interface {
	SendTransaction(t *transactor.Transaction) error
	Create(obj string) error
	Query(key []byte) ([]*transactor.Object, error)
	Delete(key []byte) error
	Close()
}

type client struct {
	maxPaylod config.ByteSize
	top       *network.Topology
	// map of shardID, to a list of nodes connections
	nodesConn map[uint64][]NodeIDConnPair
}

type NodeIDConnPair struct {
	NodeID uint64
	Conn   *network.Conn
}

func New(cfg *Config) Client {
	return &client{
		maxPaylod: cfg.MaxPayload,
		top:       cfg.Top,
		nodesConn: map[uint64][]NodeIDConnPair{},
	}
}

func (c *client) Close() {
	for _, conns := range c.nodesConn {
		for _, c := range conns {
			if err := c.Conn.Close(); err != nil {
				log.Error("transactor client: error closing connection", zap.Error(err))
			}
		}
	}
}

func (c *client) dialNodesForTransaction(t *transactor.Transaction) error {
	shardIDs := map[uint64]struct{}{}
	// for each input object / reference, send the transaction.
	for _, trace := range t.Traces {
		for _, v := range trace.InputObjectsKeys {
			shardID := c.top.ShardForKey([]byte(v))
			shardIDs[shardID] = struct{}{}
		}
		for _, v := range trace.InputReferencesKeys {
			shardID := c.top.ShardForKey([]byte(v))
			shardIDs[shardID] = struct{}{}
		}
	}
	return c.dialNodes(shardIDs)
}

func (c *client) dialUntil(shardID uint64, nodes []uint64) error {
	ctx := context.TODO()
	retry := []uint64{}
	for _, n := range nodes {
		conn, err := c.top.Dial(ctx, n)
		if err != nil {
			log.Error("transactor client: unable to connect", zap.Uint64("peer.shard", shardID), zap.Uint64("peer.id", n), zap.Error(err))
			retry = append(retry, n)
		} else {
			c.nodesConn[shardID] = append(c.nodesConn[shardID], NodeIDConnPair{n, conn})
		}

	}
	if len(retry) == 0 {
		return nil
	}
	time.Sleep(100 * time.Millisecond)
	return c.dialUntil(shardID, retry)
}

func (c *client) dialNodes(shardIDs map[uint64]struct{}) error {
	for k, _ := range shardIDs {
		c.nodesConn[k] = []NodeIDConnPair{}
		c.dialUntil(k, c.top.NodesInShard(k))
	}
	return nil
}

func (c *client) helloNodes() error {
	hellomsg := &service.Hello{
		Type: service.CONNECTION_TRANSACTOR,
	}
	for k, v := range c.nodesConn {
		for _, nc := range v {
			log.Info("sending hello", zap.Uint64("peer.shard", k), zap.Uint64("peer.id", nc.NodeID))
			err := nc.Conn.WritePayload(hellomsg, int(c.maxPaylod), 5*time.Second)
			if err != nil {
				log.Error("unable to send hello", zap.Uint64("peer.shard", k), zap.Uint64("peer.id", nc.NodeID), zap.Error(err))
				return err
			}
		}
	}
	return nil
}

func (c *client) checkTransaction(t *transactor.Transaction) (map[uint64][]byte, error) {
	req := &transactor.CheckTransactionRequest{
		Tx: t,
	}
	txbytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("transctor client: unable to marshal transaction", zap.Error(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_CHECK_TRANSACTION),
		Payload: txbytes,
	}
	mtx := &sync.Mutex{}
	evidences := map[uint64][]byte{}
	f := func(s, n uint64, msg *service.Message) error {
		mtx.Lock()
		defer mtx.Unlock()
		res := transactor.CheckTransactionResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.Error(err))
			return err
		}
		// check if Ok then add it to the evidences
		if res.Ok {
			evidences[n] = res.Signature
		}
		log.Info("check transaction answer", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.Bool("ok", res.Ok))
		return nil
	}
	err = c.sendMessages(msg, f)
	if err != nil {
		return nil, err
	}
	return evidences, nil
}

func (c *client) addTransaction(t *transactor.Transaction) error {
	req := &transactor.AddTransactionRequest{
		Tx: t,
	}
	txbytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("transctor client: unable to marshal transaction", zap.Error(err))
		return err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_ADD_TRANSACTION),
		Payload: txbytes,
	}
	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.AddTransactionResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.Error(err))
			return err
		}
		for _, v := range res.Objects {
			for _, object := range v.List {
				log.Info("add transaction answer", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.String("object.id", b64(object.Key)))
			}
		}
		return nil
	}
	return c.sendMessages(msg, f)
}

func (c *client) SendTransaction(tx *transactor.Transaction) error {
	if err := c.dialNodesForTransaction(tx); err != nil {
		return err
	}
	if err := c.helloNodes(); err != nil {
		return err
	}

	start := time.Now()
	evidences, err := c.checkTransaction(tx)
	if err != nil {
		return err
	}
	twotplusone := (2*(len(c.nodesConn)/3) + 1)
	if len(evidences) < twotplusone {
		log.Error("not enough evidence returned by nodes", zap.Int("expected", twotplusone), zap.Int("got", len(evidences)))
		return fmt.Errorf("not enough evidences returned by nodes expected(%v) got(%v)", twotplusone, len(evidences))
	}
	err = c.addTransaction(tx)
	log.Info("add transaction finished", zap.Duration("time_taken", time.Since(start)))
	return err
}

func (c *client) Query(key []byte) ([]*transactor.Object, error) {
	shardID := c.top.ShardForKey(key)
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return nil, err
	}
	if err := c.helloNodes(); err != nil {
		return nil, err
	}

	req := &transactor.QueryObjectRequest{
		ObjectKey: key,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("unable to marshal QueryObject request", zap.Error(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_QUERY_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	objs := []*transactor.Object{}
	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.QueryObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.Error(err))
			return err
		}
		mu.Lock()
		objs = append(objs, res.Object)
		mu.Unlock()
		return nil
	}
	err = c.sendMessages(msg, f)
	if err != nil {
		return nil, err
	}

	return objs, nil
}

func (c *client) Delete(key []byte) error {
	shardID := c.top.ShardForKey(key)
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return err
	}
	if err := c.helloNodes(); err != nil {
		return err
	}

	req := &transactor.DeleteObjectRequest{
		ObjectKey: key,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_DELETE_OBJECT),
		Payload: bytes,
	}

	f := func(s, n uint64, msg *service.Message) error {
		res := &transactor.DeleteObjectResponse{}
		err = proto.Unmarshal(msg.Payload, res)
		if err != nil {
			log.Error("unable to unmarshal input message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.Error(err))
			return err
		}
		keybase64 := base64.StdEncoding.EncodeToString(res.Object.Key)
		log.Info("delete object answer",
			zap.Uint64("peer.shard", s),
			zap.Uint64("peer.id", n),
			zap.String("object.id", keybase64),
			zap.String("object.value", string(res.Object.Value)),
			zap.String("object.status", res.Object.Status.String()))

		return err
	}
	return c.sendMessages(msg, f)
}

func (c *client) sendMessages(msg *service.Message, f func(uint64, uint64, *service.Message) error) error {
	wg := &sync.WaitGroup{}
	for s, nc := range c.nodesConn {
		s := s
		for _, v := range nc {
			v := v
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Info("sending message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", v.NodeID))
				_, err := v.Conn.WriteRequest(msg, int(c.maxPaylod), 5*time.Second)
				if err != nil {
					log.Error("unable to write request", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", v.NodeID), zap.Error(err))
					return
				}
				rmsg, err := v.Conn.ReadMessage(int(c.maxPaylod), 5*time.Second)
				if err != nil {
					log.Error("unable to read message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", v.NodeID), zap.Error(err))
					return
				}
				_ = f(s, v.NodeID, rmsg)
			}()
		}
	}

	wg.Wait()
	return nil
}

func (c *client) Create(obj string) error {
	ch := combihash.New()
	ch.Write([]byte(obj))
	key := ch.Digest()
	shardID := c.top.ShardForKey(key)
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return err
	}
	if err := c.helloNodes(); err != nil {
		return err
	}
	req := &transactor.NewObjectRequest{
		Object: []byte(obj),
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_CREATE_OBJECT),
		Payload: bytes,
	}

	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.NewObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.Error(err))
			return err
		}
		idbase64 := base64.StdEncoding.EncodeToString(res.ID)
		log.Info("Create object answer", zap.Uint64("peer.shard", s), zap.Uint64("peer.id", n), zap.String("object.id", idbase64))
		return nil
	}
	return c.sendMessages(msg, f)
}

func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

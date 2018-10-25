package transactorclient // import "chainspace.io/prototype/transactor/transactorclient"

import (
	"encoding/base64"
	"sync"
	"time"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
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
	NodeID     uint64
	Key        signature.KeyPair
}

type Client interface {
	SendTransaction(t *transactor.Transaction, evidences map[uint64][]byte) ([]*transactor.Object, error)
	Query(key []byte) ([]*transactor.Object, error)
	Create(obj []byte) ([][]byte, error)
	Delete(key []byte) ([]*transactor.Object, error)
	States(nodeID uint64) (*transactor.StatesReportResponse, error)
	Close()
}

type client struct {
	maxPaylod   config.ByteSize
	top         *network.Topology
	txconns     *transactor.ConnsPool
	queryconns  *transactor.ConnsPool
	createconns *transactor.ConnsPool
	deleteconns *transactor.ConnsPool
	statesconns *transactor.ConnsPool
}

func New(cfg *Config) Client {
	cp := transactor.NewConnsPool(20, cfg.NodeID, cfg.Top, int(cfg.MaxPayload), cfg.Key, service.CONNECTION_TRANSACTOR)
	c := &client{
		maxPaylod:   cfg.MaxPayload,
		top:         cfg.Top,
		txconns:     cp,
		queryconns:  cp,
		createconns: cp,
		deleteconns: cp,
		statesconns: cp,
	}

	return c
}

func (c *client) Close() {
	c.createconns.Close()
}

func (c *client) addTransaction(nodes []uint64, t *transactor.Transaction, evidences map[uint64][]byte) ([]*transactor.Object, error) {
	req := &transactor.AddTransactionRequest{
		Tx:        t,
		Evidences: evidences,
	}
	txbytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("transactor client: unable to marshal transaction", fld.Err(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(transactor.Opcode_ADD_TRANSACTION),
		Payload: txbytes,
	}
	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	objects := map[string]*transactor.Object{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := transactor.AddTransactionResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", fld.PeerID(n), fld.Err(err))
			return
		}
		mu.Lock()
		for _, v := range res.Objects {
			for _, object := range v.List {
				object := object
				log.Info("add transaction answer", fld.PeerID(n), log.String("object.id", b64(object.Key)))
				objects[string(object.Key)] = object
			}
		}
		mu.Unlock()
	}
	conns := c.txconns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		conns.WriteRequest(nid, msg, 5*time.Second, true, f)
	}
	wg.Wait()
	objectsres := []*transactor.Object{}
	for _, v := range objects {
		v := v
		objectsres = append(objectsres, v)

	}

	return objectsres, nil
}

func (c *client) nodesForTx(t *transactor.Transaction) []uint64 {
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
	out := []uint64{}
	for k, _ := range shardIDs {
		out = append(out, c.top.NodesInShard(k)...)
	}
	return out
}

func (c *client) SendTransaction(tx *transactor.Transaction, evidences map[uint64][]byte) ([]*transactor.Object, error) {
	nodes := c.nodesForTx(tx)
	start := time.Now()
	// add the transaction
	objs, err := c.addTransaction(nodes, tx, evidences)
	log.Info("add transaction finished", log.Duration("time_taken", time.Since(start)))
	return objs, err
}

func (c *client) Query(key []byte) ([]*transactor.Object, error) {
	nodes := c.top.NodesInShard(c.top.ShardForKey(key))
	req := &transactor.QueryObjectRequest{
		ObjectKey: key,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("unable to marshal QueryObject request", fld.Err(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(transactor.Opcode_QUERY_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	objs := []*transactor.Object{}
	wg := &sync.WaitGroup{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := transactor.QueryObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			return
		}
		if res.Object == nil {
			return
		}
		mu.Lock()
		objs = append(objs, res.Object)
		mu.Unlock()
		return
	}
	conns := c.queryconns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		conns.WriteRequest(nid, msg, 5*time.Second, true, f)
	}
	wg.Wait()
	return objs, nil
}

func (c *client) Create(obj []byte) ([][]byte, error) {
	ch := combihash.New()
	ch.Write(obj)
	key := ch.Digest()
	nodes := c.top.NodesInShard(c.top.ShardForKey(key))

	req := &transactor.NewObjectRequest{
		Object: obj,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	msg := service.Message{
		Opcode:  int32(transactor.Opcode_CREATE_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	objs := [][]byte{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		log.Error("TIME ELAPSED TO CREATE OBJECT FROM NODE", log.Uint64("NODEID", n), log.String("duration", time.Since(now).String()))
		res := transactor.NewObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			return
		}
		mu.Lock()
		objs = append(objs, res.ID)
		mu.Unlock()
		return
	}
	conns := c.createconns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		msg := msg
		go conns.WriteRequest(nid, &msg, 5*time.Second, true, f)
	}
	wg.Wait()
	log.Error("TIME ELAPSED TO CREATE OBJECT", log.String("duration", time.Since(now).String()))

	return objs, nil
}

func (c *client) Delete(key []byte) ([]*transactor.Object, error) {
	nodes := c.top.NodesInShard(c.top.ShardForKey(key))
	req := &transactor.DeleteObjectRequest{
		ObjectKey: key,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(transactor.Opcode_DELETE_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	objs := []*transactor.Object{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := &transactor.DeleteObjectResponse{}
		err = proto.Unmarshal(msg.Payload, res)
		if err != nil {
			return
		}
		mu.Lock()
		objs = append(objs, res.Object)
		mu.Unlock()
	}
	conns := c.deleteconns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		conns.WriteRequest(nid, msg, 5*time.Second, true, f)
	}
	wg.Wait()

	return objs, nil
}

func (c *client) States(nodeID uint64) (*transactor.StatesReportResponse, error) {
	msg := &service.Message{
		Opcode: int32(transactor.Opcode_STATES),
	}

	wg := &sync.WaitGroup{}
	res := &transactor.StatesReportResponse{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		err := proto.Unmarshal(msg.Payload, res)
		if err != nil {
			return
		}
	}

	wg.Add(1)
	conns := c.statesconns.Borrow()
	conns.WriteRequest(nodeID, msg, 5*time.Second, true, f)
	return res, nil
}

func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

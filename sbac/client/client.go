package client // import "chainspace.io/prototype/sbac/client"

import (
	"encoding/base64"
	"sync"
	"time"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/combihash"
	"chainspace.io/prototype/internal/conns"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/sbac"
	"chainspace.io/prototype/service"

	"github.com/gogo/protobuf/proto"
)

// Config represent the configuration required to send messages
// using the sbac
type Config struct {
	Top        *network.Topology
	MaxPayload config.ByteSize
	NodeID     uint64
	Key        signature.KeyPair
}

type Client interface {
	SendTransaction(t *sbac.Transaction, evidences map[uint64][]byte) ([]*sbac.Object, error)
	Query(key []byte) ([]*sbac.Object, error)
	Create(obj []byte) ([][]byte, error)
	Delete(key []byte) ([]*sbac.Object, error)
	States(nodeID uint64) (*sbac.StatesReportResponse, error)
	Close()
}

type client struct {
	maxPaylod config.ByteSize
	top       *network.Topology
	conns     *conns.Pool
}

func New(cfg *Config) Client {
	cp := conns.NewPool(20, cfg.NodeID, cfg.Top, int(cfg.MaxPayload), cfg.Key, service.CONNECTION_SBAC)
	c := &client{
		maxPaylod: cfg.MaxPayload,
		top:       cfg.Top,
		conns:     cp,
	}

	return c
}

func (c *client) Close() {
	c.conns.Close()
}

func (c *client) addTransaction(nodes []uint64, t *sbac.Transaction, evidences map[uint64][]byte) ([]*sbac.Object, error) {
	req := &sbac.AddTransactionRequest{
		Tx:        t,
		Evidences: evidences,
	}
	txbytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("sbac client: unable to marshal transaction", fld.Err(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(sbac.Opcode_ADD_TRANSACTION),
		Payload: txbytes,
	}
	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	objects := map[string]*sbac.Object{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := sbac.AddTransactionResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", fld.PeerID(n), fld.Err(err))
			return
		}
		mu.Lock()
		for _, v := range res.Objects {
			for _, object := range v.List {
				object := object
				log.Info("add transaction answer", fld.PeerID(n), log.String("object.id", b64(object.VersionID)))
				objects[string(object.VersionID)] = object
			}
		}
		mu.Unlock()
	}
	conns := c.conns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		conns.WriteRequest(nid, msg, 5*time.Second, true, f)
	}
	wg.Wait()
	objectsres := []*sbac.Object{}
	for _, v := range objects {
		v := v
		objectsres = append(objectsres, v)

	}

	return objectsres, nil
}

func (c *client) nodesForTx(t *sbac.Transaction) []uint64 {
	shardIDs := map[uint64]struct{}{}
	// for each input object / reference, send the transaction.
	for _, trace := range t.Traces {
		for _, v := range trace.InputObjectVersionIDs {
			shardID := c.top.ShardForVersionID([]byte(v))
			shardIDs[shardID] = struct{}{}
		}
		for _, v := range trace.InputReferenceVersionIDs {
			shardID := c.top.ShardForVersionID([]byte(v))
			shardIDs[shardID] = struct{}{}
		}
	}
	out := []uint64{}
	for k, _ := range shardIDs {
		out = append(out, c.top.NodesInShard(k)...)
	}
	return out
}

func (c *client) SendTransaction(tx *sbac.Transaction, evidences map[uint64][]byte) ([]*sbac.Object, error) {
	nodes := c.nodesForTx(tx)
	start := time.Now()
	// add the transaction
	objs, err := c.addTransaction(nodes, tx, evidences)
	log.Info("add transaction finished", log.Duration("time_taken", time.Since(start)))
	return objs, err
}

func (c *client) Query(vid []byte) ([]*sbac.Object, error) {
	nodes := c.top.NodesInShard(c.top.ShardForVersionID(vid))
	req := &sbac.QueryObjectRequest{
		VersionID: vid,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("unable to marshal QueryObject request", fld.Err(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(sbac.Opcode_QUERY_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	objs := []*sbac.Object{}
	wg := &sync.WaitGroup{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := sbac.QueryObjectResponse{}
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
	conns := c.conns.Borrow()
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
	nodes := c.top.NodesInShard(c.top.ShardForVersionID(key))

	req := &sbac.NewObjectRequest{
		Object: obj,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	msg := service.Message{
		Opcode:  int32(sbac.Opcode_CREATE_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	objs := [][]byte{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		log.Error("TIME ELAPSED TO CREATE OBJECT FROM NODE", log.Uint64("NODEID", n), log.String("duration", time.Since(now).String()))
		res := sbac.NewObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			return
		}
		mu.Lock()
		objs = append(objs, res.ID)
		mu.Unlock()
		return
	}
	conns := c.conns.Borrow()
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

func (c *client) Delete(versionid []byte) ([]*sbac.Object, error) {
	nodes := c.top.NodesInShard(c.top.ShardForVersionID(versionid))
	req := &sbac.DeleteObjectRequest{
		VersionID: versionid,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(sbac.Opcode_DELETE_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	wg := &sync.WaitGroup{}
	objs := []*sbac.Object{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := &sbac.DeleteObjectResponse{}
		err = proto.Unmarshal(msg.Payload, res)
		if err != nil {
			return
		}
		mu.Lock()
		objs = append(objs, res.Object)
		mu.Unlock()
	}
	conns := c.conns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		conns.WriteRequest(nid, msg, 5*time.Second, true, f)
	}
	wg.Wait()

	return objs, nil
}

func (c *client) States(nodeID uint64) (*sbac.StatesReportResponse, error) {
	msg := &service.Message{
		Opcode: int32(sbac.Opcode_STATES),
	}

	wg := &sync.WaitGroup{}
	res := &sbac.StatesReportResponse{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		err := proto.Unmarshal(msg.Payload, res)
		if err != nil {
			return
		}
	}

	wg.Add(1)
	conns := c.conns.Borrow()
	conns.WriteRequest(nodeID, msg, 5*time.Second, true, f)
	return res, nil
}

func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

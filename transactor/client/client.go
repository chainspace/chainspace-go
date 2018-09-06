package transactorclient // import "chainspace.io/prototype/transactor/client"

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/transactor"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/sync/errgroup"
)

// Config represent the configuration required to send messages
// using the transactor
type Config struct {
	Top        *network.Topology
	MaxPayload config.ByteSize
}

type Client interface {
	SendTransaction(t *transactor.Transaction) ([]*transactor.Object, error)
	Create(obj []byte) ([][]byte, error)
	Query(key []byte) ([]*transactor.Object, error)
	Delete(key []byte) ([]*transactor.Object, error)
	States(states uint64) (*transactor.StatesReportResponse, error)
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
	c := &client{
		maxPaylod: cfg.MaxPayload,
		top:       cfg.Top,
		nodesConn: map[uint64][]NodeIDConnPair{},
	}
	return c
}

func (c *client) Close() {
	for _, conns := range c.nodesConn {
		conns := conns
		for _, c := range conns {
			c := c
			if err := c.Conn.Close(); err != nil {
				log.Error("transactor client: error closing connection", fld.Err(err))
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
	retry := []uint64{}
	for _, n := range nodes {
		conn, err := c.top.Dial(n, 5*time.Second)
		if err != nil {
			log.Error("transactor client: unable to connect", fld.PeerShard(shardID), fld.PeerID(n), fld.Err(err))
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
			log.Info("sending hello", fld.PeerShard(k), fld.PeerID(nc.NodeID))
			err := nc.Conn.WritePayload(hellomsg, int(c.maxPaylod), 5*time.Second)
			if err != nil {
				log.Error("unable to send hello", fld.PeerShard(k), fld.PeerID(nc.NodeID), fld.Err(err))
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
		log.Error("transctor client: unable to marshal transaction", fld.Err(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(transactor.Opcode_CHECK_TRANSACTION),
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
			log.Error("unable to unmarshal input message", fld.PeerShard(s), fld.PeerID(n), fld.Err(err))
			return err
		}
		// check if Ok then add it to the evidences
		if res.Ok {
			evidences[n] = res.Signature
		}
		log.Info("check transaction answer", fld.PeerShard(s), fld.PeerID(n), log.Bool("ok", res.Ok))
		return nil
	}
	err = c.sendMessages(msg, f)
	if err != nil {
		return nil, err
	}
	return evidences, nil
}

func (c *client) addTransaction(t *transactor.Transaction) ([]*transactor.Object, error) {
	req := &transactor.AddTransactionRequest{
		Tx: t,
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
	objects := map[string]*transactor.Object{}
	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.AddTransactionResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", fld.PeerShard(s), fld.PeerID(n), fld.Err(err))
			return err
		}
		mu.Lock()
		for _, v := range res.Objects {
			for _, object := range v.List {
				object := object
				log.Info("add transaction answer", fld.PeerShard(s), fld.PeerID(n), log.String("object.id", b64(object.Key)))
				objects[string(object.Key)] = object
			}
		}
		mu.Unlock()
		return nil
	}
	if err := c.sendMessages(msg, f); err != nil {
		return nil, err
	}
	objectsres := []*transactor.Object{}
	for _, v := range objects {
		v := v
		objectsres = append(objectsres, v)

	}

	return objectsres, nil
}

func (c *client) SendTransaction(tx *transactor.Transaction) ([]*transactor.Object, error) {
	if err := c.dialNodesForTransaction(tx); err != nil {
		return nil, err
	}
	if err := c.helloNodes(); err != nil {
		return nil, err
	}
	start := time.Now()
	evidences, err := c.checkTransaction(tx)
	if err != nil {
		return nil, err
	}
	twotplusone := (2*(len(c.nodesConn)/3) + 1)
	if len(evidences) < twotplusone {
		log.Error("not enough evidence returned by nodes", log.Int("expected", twotplusone), log.Int("got", len(evidences)))
		return nil, fmt.Errorf("not enough evidences returned by nodes expected(%v) got(%v)", twotplusone, len(evidences))
	}
	objs, err := c.addTransaction(tx)
	log.Info("add transaction finished", log.Duration("time_taken", time.Since(start)))
	return objs, err
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
		log.Error("unable to marshal QueryObject request", fld.Err(err))
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(transactor.Opcode_QUERY_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	objs := []*transactor.Object{}
	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.QueryObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", fld.PeerShard(s), fld.PeerID(n), fld.Err(err))
			return err
		}
		if res.Object == nil {
			return fmt.Errorf("lol")
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

func (c *client) Delete(key []byte) ([]*transactor.Object, error) {
	shardID := c.top.ShardForKey(key)
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return nil, err
	}
	if err := c.helloNodes(); err != nil {
		return nil, err
	}

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
	objs := []*transactor.Object{}
	f := func(s, n uint64, msg *service.Message) error {
		res := &transactor.DeleteObjectResponse{}
		err = proto.Unmarshal(msg.Payload, res)
		if err != nil {
			log.Error("unable to unmarshal input message", fld.PeerShard(s), fld.PeerID(n), fld.Err(err))
			return err
		}
		mu.Lock()
		objs = append(objs, res.Object)
		mu.Unlock()
		return err
	}
	if err := c.sendMessages(msg, f); err != nil {
		return nil, err
	}
	return objs, nil
}

func (c *client) States(nodeID uint64) (*transactor.StatesReportResponse, error) {
	shardID := c.top.ShardForNode(nodeID)
	log.Error("CALLING SHARD AND NODE", log.Uint64("NODE", nodeID), log.Uint64("SHARD", shardID))
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return nil, err
	}
	if err := c.helloNodes(); err != nil {
		return nil, err
	}

	msg := &service.Message{
		Opcode: int32(transactor.Opcode_STATES),
	}

	var nc NodeIDConnPair
	var ok bool
	for _, v := range c.nodesConn[shardID] {
		if v.NodeID == nodeID {
			nc = v
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("node with id(%v) do not exists", nodeID)
	}
	_, err := nc.Conn.WriteRequest(msg, int(c.maxPaylod), 5*time.Second)
	if err != nil {
		log.Error("unable to write request", fld.PeerShard(shardID), fld.PeerID(nodeID), fld.Err(err))
		return nil, err
	}
	rmsg, err := nc.Conn.ReadMessage(int(c.maxPaylod), 15*time.Second)
	if err != nil {
		return nil, err
	}

	res := &transactor.StatesReportResponse{}
	err = proto.Unmarshal(rmsg.Payload, res)
	if err != nil {
		log.Error("unable to unmarshal input message", fld.PeerShard(shardID), fld.PeerID(nodeID), fld.Err(err))
		return nil, err
	}

	return res, nil
}

func (c *client) sendMessages(msg *service.Message, f func(uint64, uint64, *service.Message) error) error {
	wg, _ := errgroup.WithContext(context.TODO())
	for s, nc := range c.nodesConn {
		s := s
		for _, v := range nc {
			v := v
			wg.Go(func() error {
				log.Info("sending message", fld.PeerShard(s), fld.PeerID(v.NodeID))
				_, err := v.Conn.WriteRequest(msg, int(c.maxPaylod), 5*time.Second)
				if err != nil {
					log.Error("unable to write request", fld.PeerShard(s), fld.PeerID(v.NodeID), fld.Err(err))
					return err
				}
				rmsg, err := v.Conn.ReadMessage(int(c.maxPaylod), 15*time.Second)
				if err != nil {
					// log.Error("unable to read message", fld.PeerShard(s), fld.PeerID(v.NodeID), fld.Err(err))
					return err
				}
				err = f(s, v.NodeID, rmsg)
				if err != nil {
					return err
				}
				return nil
			})
		}
	}

	if err := wg.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *client) Create(obj []byte) ([][]byte, error) {
	ch := combihash.New()
	ch.Write([]byte(obj))
	key := ch.Digest()
	shardID := c.top.ShardForKey(key)
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return nil, err
	}
	if err := c.helloNodes(); err != nil {
		return nil, err
	}
	req := &transactor.NewObjectRequest{
		Object: obj,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	msg := &service.Message{
		Opcode:  int32(transactor.Opcode_CREATE_OBJECT),
		Payload: bytes,
	}

	mu := sync.Mutex{}
	objs := [][]byte{}
	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.NewObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal input message", fld.PeerShard(s), fld.PeerID(n), fld.Err(err))
			return err
		}
		mu.Lock()
		objs = append(objs, res.ID)
		mu.Unlock()
		return nil
	}
	if err := c.sendMessages(msg, f); err != nil {
		return nil, err
	}
	return objs, nil
}

func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

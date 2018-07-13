package transactorclient // import "chainspace.io/prototype/service/transactor/client"

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"chainspace.io/prototype/combihash"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/service/transactor"

	"github.com/gogo/protobuf/proto"
)

type ClientTransaction struct {
	Traces []ClientTrace `json:"traces"`
}

func (ct *ClientTransaction) ToTransaction() *transactor.Transaction {
	traces := make([]*transactor.Trace, 0, len(ct.Traces))
	for _, t := range ct.Traces {
		traces = append(traces, t.ToTrace())
	}
	return &transactor.Transaction{
		Traces: traces,
	}
}

type ClientTrace struct {
	ContractID          string        `json:"contract_id"`
	Procedure           string        `json:"procedure"`
	InputObjectsKeys    []string      `json:"input_objects_keys"`
	InputReferencesKeys []string      `json:"input_references_keys"`
	OutputObjects       []string      `json:"output_objects"`
	Parameters          []string      `json:"parameters"`
	Returns             []string      `json:"returns"`
	Dependencies        []ClientTrace `json:"dependencies"`
}

func (ct *ClientTrace) ToTrace() *transactor.Trace {
	toBytes := func(s []string) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			out = append(out, []byte(v))
		}
		return out
	}
	toBytesB64 := func(s []string) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			bytes, _ := base64.StdEncoding.DecodeString(v)
			out = append(out, []byte(bytes))
		}
		return out
	}
	deps := make([]*transactor.Trace, 0, len(ct.Dependencies))
	for _, d := range ct.Dependencies {
		deps = append(deps, d.ToTrace())
	}
	return &transactor.Trace{
		ContractID:          ct.ContractID,
		Procedure:           ct.Procedure,
		InputObjectsKeys:    toBytesB64(ct.InputObjectsKeys),
		InputReferencesKeys: toBytesB64(ct.InputReferencesKeys),
		OutputObjects:       toBytes(ct.OutputObjects),
		Parameters:          toBytesB64(ct.Parameters),
		Returns:             toBytesB64(ct.Returns),
		Dependencies:        deps,
	}
}

// Config represent the configuration required to send messages
// using the transactor
type Config struct {
	NetworkConfig config.Network
	NetworkName   string
	ShardID       uint64
}

type Client interface {
	SendTransaction(t *ClientTransaction) error
	Create(obj string) error
	Query(key []byte) error
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

func New(cfg *Config) (Client, error) {
	topology, err := network.New(cfg.NetworkName, &cfg.NetworkConfig)
	if err != nil {
		return nil, err
	}
	topology.BootstrapMDNS()
	time.Sleep(1 * time.Second)

	return &client{
		maxPaylod: cfg.NetworkConfig.MaxPayload,
		top:       topology,
		nodesConn: map[uint64][]NodeIDConnPair{},
	}, nil
}

func (c *client) Close() {
	for _, conns := range c.nodesConn {
		for _, c := range conns {
			if err := c.Conn.Close(); err != nil {
				log.Errorf("transactor client: error closing connection: %v", err)
			}
		}
	}
}

func (c *client) dialNodesForTransaction(t *ClientTransaction) error {
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
			log.Errorf("transactor client: unable to connect to shard(%v)->node(%v): %v", shardID, n, err)
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
			log.Infof("sending hello to shard shard(%v)->node(%v)", k, nc.NodeID)
			err := nc.Conn.WritePayload(hellomsg, int(c.maxPaylod), 5*time.Second)
			if err != nil {
				log.Errorf("transactor client: unable to send hello message to shard(%v)->node(%v): %v", k, nc.NodeID, err)
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
		log.Errorf("transctor client: unable to marshal transaction: %v", err)
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
			log.Errorf("unable unmarshal message from shard(%v)->node(%v): %v", s, n, err)
			return err
		}
		// check if Ok then add it to the evidences
		if res.Ok {
			evidences[n] = res.Signature
		}
		log.Infof("shard(%v)->node(%v) answered with {ok(%v)}", s, n, res.Ok)
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
		log.Errorf("transctor client: unable to marshal transaction: %v", err)
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
			log.Errorf("unable unmarshal message from shard(%v)->node(%v): %v", s, n, err)
			return err
		}
		for _, v := range res.Objects {
			for _, object := range v.List {
				log.Infof("shard(%v)->node(%v) answered with output object key: %v", s, n, b64(object.Key))
			}
		}
		return nil
	}
	return c.sendMessages(msg, f)
}

func (c *client) SendTransaction(t *ClientTransaction) error {
	if err := c.dialNodesForTransaction(t); err != nil {
		return err
	}
	if err := c.helloNodes(); err != nil {
		return err
	}
	tx := t.ToTransaction()
	evidences, err := c.checkTransaction(tx)
	if err != nil {
		return err
	}
	twotplusone := (2*(len(c.nodesConn)/3) + 1)
	if len(evidences) < twotplusone {
		return fmt.Errorf("not enough evidences returned by nodes expected(%v) got(%v)", twotplusone, len(evidences))
	}
	return c.addTransaction(tx)
}

func (c *client) Query(key []byte) error {
	shardID := c.top.ShardForKey(key)
	if err := c.dialNodes(map[uint64]struct{}{shardID: struct{}{}}); err != nil {
		return err
	}
	if err := c.helloNodes(); err != nil {
		return err
	}

	req := &transactor.QueryObjectRequest{
		ObjectKey: key,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_QUERY_OBJECT),
		Payload: bytes,
	}

	f := func(s, n uint64, msg *service.Message) error {
		res := transactor.QueryObjectResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Errorf("unable unmarshal message from shard(%v)->node(%v): %v", s, n, err)
			return err
		}
		keybase64 := base64.StdEncoding.EncodeToString(res.Object.Key)
		log.Infof("shard(%v)->node(%v) answered with {id(%v), value(%v), status(%v)}", s, n, keybase64, string(res.Object.Value), res.Object.Status)
		return nil
	}
	return c.sendMessages(msg, f)
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
			log.Errorf("unable unmarshal message from shard(%v)->node(%v): %v", s, n, err)
			return err
		}
		keybase64 := base64.StdEncoding.EncodeToString(res.Object.Key)
		log.Infof("shard(%v)->node(%v) answered with {id(%v), value(%v), status(%v)}", s, n, keybase64, string(res.Object.Value), res.Object.Status)

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
				log.Infof("sending transaction to shard(%v)->node(%v)", s, v.NodeID)
				_, err := v.Conn.WriteRequest(msg, int(c.maxPaylod), 5*time.Second)
				if err != nil {
					log.Errorf("unable to write request to shard(%v)->node(%v): %v", s, v.NodeID, err)
					return
				}
				rmsg, err := v.Conn.ReadMessage(int(c.maxPaylod), 5*time.Second)
				if err != nil {
					log.Errorf("unable to read message from shard(%v)->node(%v): %v", s, v.NodeID, err)
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
			log.Errorf("unable unmarshal message from shard(%v)->node(%v): %v", s, n, err)
			return err
		}
		idbase64 := base64.StdEncoding.EncodeToString(res.ID)
		log.Infof("shard(%v)->node(%v) answered with id: %v", s, n, idbase64)
		return nil
	}
	return c.sendMessages(msg, f)
}

func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

package transactorclient // import "chainspace.io/prototype/service/transactor/client"

import (
	"context"
	"encoding/base64"
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
	deps := make([]*transactor.Trace, 0, len(ct.Dependencies))
	for _, d := range ct.Dependencies {
		deps = append(deps, d.ToTrace())
	}
	return &transactor.Trace{
		ContractID:          ct.ContractID,
		Procedure:           ct.Procedure,
		InputObjectsKeys:    toBytes(ct.InputObjectsKeys),
		InputReferencesKeys: toBytes(ct.InputReferencesKeys),
		OutputObjects:       toBytes(ct.OutputObjects),
		Parameters:          toBytes(ct.Parameters),
		Returns:             toBytes(ct.Returns),
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
	top *network.Topology
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
	time.Sleep(100 * time.Millisecond)

	return &client{
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

func (c *client) dialNodes(shardIDs map[uint64]struct{}) error {
	ctx := context.TODO()
	for k, _ := range shardIDs {
		c.nodesConn[k] = []NodeIDConnPair{}
		for _, n := range c.top.NodesInShard(k) {
			conn, err := c.top.Dial(ctx, n)
			if err != nil {
				log.Errorf("transactor client: unable to connect to shard(%v)->node(%v): %v", n, k, err)
				return err
			}
			c.nodesConn[k] = append(c.nodesConn[k], NodeIDConnPair{n, conn})
		}
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
			err := nc.Conn.WritePayload(hellomsg, 5*time.Second)
			if err != nil {
				log.Errorf("transactor client: unable to send hello message to shard(%v)->node(%v): %v", k, nc.NodeID, err)
				return err
			}
		}
	}
	return nil
}

func (c *client) SendTransaction(t *ClientTransaction) error {
	if err := c.dialNodesForTransaction(t); err != nil {
		return err
	}
	if err := c.helloNodes(); err != nil {
		return err
	}
	req := &transactor.AddTransactionRequest{
		Transaction: t.ToTransaction(),
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
		log.Infof("shard(%v)->node(%v) answered with: %v", s, n, msg)
		return nil
	}
	return c.sendMessages(msg, f)
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

	req := &transactor.RemoveObjectRequest{
		ObjectKey: key,
	}
	bytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	msg := &service.Message{
		Opcode:  uint32(transactor.Opcode_REMOVE_OBJECT),
		Payload: bytes,
	}

	f := func(s, n uint64, msg *service.Message) error {
		res := &transactor.RemoveObjectResponse{}
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
		for _, v := range nc {
			v := v
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Infof("sending transaction to shard(%v)->node(%v)", s, v.NodeID)
				err := v.Conn.WriteRequest(msg, 5*time.Second)
				if err != nil {
					log.Errorf("unable to write request to shard(%v)->node(%v): %v", s, v.NodeID, err)
					return
				}
				rmsg, err := v.Conn.ReadMessage(5 * time.Second)
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

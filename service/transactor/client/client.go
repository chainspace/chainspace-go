package transactorclient // import "chainspace.io/prototype/service/transactor/client"

import (
	"time"

	"golang.org/x/net/context"

	"github.com/gogo/protobuf/proto"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"chainspace.io/prototype/service/transactor"
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
	time.Sleep(time.Second)

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

func (c *client) dialNodes(t *ClientTransaction) error {
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
	if err := c.dialNodes(t); err != nil {
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
	for s, nc := range c.nodesConn {
		for _, v := range nc {
			log.Infof("sending transaction to shard(%v)->node(%v)", s, v.NodeID)
			v.Conn.WriteRequest(msg, 5*time.Second)
		}
	}

	time.Sleep(5 * time.Second)

	return nil
}

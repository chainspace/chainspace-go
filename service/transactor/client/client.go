package transactorclient // import "chainspace.io/prototype/service/transactor/client"

import (
	"time"

	"golang.org/x/net/context"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/network"
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
}

type client struct {
	shardID  uint64
	topology *network.Topology
}

func New(cfg *Config) (Client, error) {
	topology, err := network.New(cfg.NetworkName, &cfg.NetworkConfig)
	if err != nil {
		return nil, err
	}
	topology.BootstrapMDNS()

	return &client{
		shardID:  cfg.ShardID,
		topology: topology,
	}, nil
}

func (c *client) SendTransaction(t *ClientTransaction) error {
	// Bootstrap using mDNS.

	time.Sleep(time.Second)
	conn, err := c.topology.DialAnyInShard(context.TODO(), c.shardID)
	if err != nil {
		return err
	}

	conn.Write([]byte("testing"))
	time.Sleep(5 * time.Second)

	return nil
}

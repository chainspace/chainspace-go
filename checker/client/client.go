package client // import "chainspace.io/prototype/checker/client"

import (
	"fmt"
	"sync"
	"time"

	"chainspace.io/prototype/checker"
	"chainspace.io/prototype/config"
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

type Client struct {
	maxPaylod config.ByteSize
	top       *network.Topology
	conns     conns.Pool
}

func (c *Client) nodesForTx(t *sbac.Transaction) []uint64 {
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

func (c *Client) check(nodes []uint64, t *sbac.Transaction) (map[uint64][]byte, error) {
	req := &checker.CheckRequest{
		Tx: t,
	}
	txbytes, err := proto.Marshal(req)
	if err != nil {
		log.Error("transctor client: unable to marshal transaction", fld.Err(err))
		return nil, err
	}
	msg := service.Message{
		Opcode:  int32(checker.Opcode_CHECK),
		Payload: txbytes,
	}
	now := time.Now()
	mu := &sync.Mutex{}
	evidences := map[uint64][]byte{}
	wg := &sync.WaitGroup{}
	f := func(n uint64, msg *service.Message) {
		defer wg.Done()
		res := checker.CheckResponse{}
		err = proto.Unmarshal(msg.Payload, &res)
		if err != nil {
			log.Error("unable to unmarshal proto", fld.Err(err))
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if res.Ok {
			evidences[n] = res.Signature
		}

	}

	conns := c.conns.Borrow()
	for _, nid := range nodes {
		nid := nid
		wg.Add(1)
		msg := msg
		go conns.WriteRequest(nid, &msg, 5*time.Second, true, f)
	}
	wg.Wait()
	log.Error("TIME CHECKER", log.String("time-taken", time.Since(now).String()))
	return evidences, nil
}

func (c *Client) Check(tx *sbac.Transaction) (map[uint64][]byte, error) {
	nodes := c.nodesForTx(tx)
	evidences, err := c.check(nodes, tx)
	if err != nil {
		return nil, err
	}

	if len(evidences) != len(nodes) {
		return nil, fmt.Errorf("not enough evidences returned by nodes")
	}

	/*
		twotplusone := (2*(len(nodes)/3) + 1)
		if len(evidences) < twotplusone {
			log.Error("not enough evidence returned by nodes", log.Int("expected", twotplusone), log.Int("got", len(evidences)))
			return nil, fmt.Errorf("not enough evidences returned by nodes expected(%v) got(%v)", twotplusone, len(evidences))
		}
	*/

	return evidences, nil
}

func New(cfg *Config) *Client {
	cp := conns.NewPool(20, cfg.NodeID, cfg.Top, int(cfg.MaxPayload), cfg.Key, service.CONNECTION_CHECKER)
	c := &Client{
		maxPaylod: cfg.MaxPayload,
		top:       cfg.Top,
		conns:     cp,
	}

	return c
}

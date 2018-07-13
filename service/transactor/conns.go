package transactor

import (
	"crypto/rand"
	"sync"
	"time"

	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
)

type ConnsCache struct {
	conns      map[uint64]*network.Conn
	ctx        context.Context
	maxPayload int
	mtx        *sync.Mutex
	selfID     uint64
	key        signature.KeyPair
	top        *network.Topology
}

func (c *ConnsCache) Close() error {
	for _, v := range c.conns {
		if err := v.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConnsCache) helloMsg(clientID uint64, serverID uint64, key signature.KeyPair) (*service.Hello, error) {
	nonce := make([]byte, 36)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	payload, err := proto.Marshal(&service.HelloInfo{
		Client:    clientID,
		Nonce:     nonce,
		Server:    serverID,
		Timestamp: time.Now(),
	})
	if err != nil {
		return nil, err
	}
	return &service.Hello{
		Agent:     "go/0.0.1",
		Payload:   payload,
		Signature: key.Sign(payload),
		Type:      service.CONNECTION_TRANSACTOR,
	}, nil
}

func (c *ConnsCache) sendHello(nodeID uint64, conn *network.Conn) error {
	hellomsg, err := c.helloMsg(c.selfID, nodeID, c.key)
	if err != nil {
		return err
	}
	return conn.WritePayload(hellomsg, c.maxPayload, time.Second)
}

func (c *ConnsCache) dial(nodeID uint64) (*network.Conn, error) {
	c.mtx.Lock()
	if c, ok := c.conns[nodeID]; ok {
		return c, nil
	}
	c.mtx.Unlock()
	conn, err := c.top.Dial(c.ctx, nodeID)
	if err != nil {
		return nil, err
	}
	err = c.sendHello(nodeID, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.mtx.Lock()
	c.conns[nodeID] = conn
	c.mtx.Unlock()
	return conn, nil
}

func (c *ConnsCache) WriteRequest(nodeID uint64, msg *service.Message, timeout time.Duration) (uint64, error) {
	conn, err := c.dial(nodeID)
	if err != nil {
		return 0, err
	}
	return conn.WriteRequest(msg, c.maxPayload, timeout)
}

func NewConnsCache(ctx context.Context, nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair) *ConnsCache {
	return &ConnsCache{
		conns:      map[uint64]*network.Conn{},
		ctx:        ctx,
		maxPayload: maxPayload,
		mtx:        &sync.Mutex{},
		selfID:     nodeID,
		top:        top,
		key:        key,
	}
}

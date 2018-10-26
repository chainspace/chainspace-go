package conns // import "chainspace.io/prototype/internal/conns"

import (
	"sync"
	"time"

	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
)

type MuConn struct {
	mu   sync.Mutex
	conn *network.Conn
	die  chan bool
}

type PendingAck struct {
	nodeID  uint64
	msg     *service.Message
	sentAt  time.Time
	timeout time.Duration
	cb      func(uint64, *service.Message)
}

type AckID struct {
	NodeID    uint64
	RequestID uint64
}

type Cache struct {
	conns      map[uint64]*MuConn
	cmu        []sync.Mutex
	mu         sync.Mutex
	key        signature.KeyPair
	maxPayload int
	selfID     uint64
	top        *network.Topology
	connection service.CONNECTION

	pendingAcks   map[AckID]PendingAck
	pendingAcksMu sync.Mutex
}

func (c *Cache) Close() {}

func (c *Cache) sendHello(nodeID uint64, conn *network.Conn) error {
	if c.key == nil {
		log.Error("nil key")
	}
	hellomsg, err := service.SignHello(
		c.selfID, nodeID, c.key, c.connection)
	if err != nil {
		return err
	}
	return conn.WritePayload(hellomsg, c.maxPayload, time.Second)
}

func (c *Cache) dial(nodeID uint64) (*MuConn, error) {
	// conn exist
	c.mu.Lock()
	cc, ok := c.conns[nodeID]
	if ok {
		c.mu.Unlock()
		return cc, nil
	}

	defer c.mu.Unlock()
	log.Error("NEED TO DIAL", fld.NodeID(nodeID))
	// need to dial
	conn, err := c.top.Dial(nodeID, 5*time.Hour)
	if err != nil {
		return nil, err
	}
	err = c.sendHello(nodeID, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	cc = &MuConn{conn: conn, die: make(chan bool)}
	go c.readAckMessage(nodeID, cc.conn, cc.die)
	c.conns[nodeID] = cc
	return cc, nil
}

func (c *Cache) release(nodeID uint64) {
	c.mu.Lock()
	cc, ok := c.conns[nodeID]
	c.mu.Unlock()
	if ok {

		cc.die <- true
		cc.conn.Close()
		c.mu.Lock()
		delete(c.conns, nodeID)
		c.mu.Unlock()
	}
}

func (c *Cache) WriteRequest(
	nodeID uint64, msg *service.Message, timeout time.Duration, ack bool, cb func(uint64, *service.Message)) (uint64, error) {
	c.cmu[nodeID-1].Lock()
	mc, err := c.dial(nodeID)
	if err != nil {
		c.release(nodeID)
		c.cmu[nodeID-1].Unlock()
		return c.WriteRequest(nodeID, msg, timeout, ack, cb)
	}
	id, err := mc.conn.WriteRequest(msg, c.maxPayload, timeout)
	if err != nil {
		c.release(nodeID)
		c.cmu[nodeID-1].Unlock()
		return c.WriteRequest(nodeID, msg, timeout, ack, cb)
	}
	c.cmu[nodeID-1].Unlock()
	if ack {
		c.addPendingAck(nodeID, msg, timeout, id, cb)
	}
	return id, nil
}

func (c *Cache) addPendingAck(nodeID uint64, msg *service.Message, timeout time.Duration, id uint64, cb func(uint64, *service.Message)) {
	ack := PendingAck{
		sentAt:  time.Now(),
		nodeID:  nodeID,
		msg:     msg,
		timeout: timeout,
		cb:      cb,
	}
	c.pendingAcksMu.Lock()
	c.pendingAcks[AckID{nodeID, id}] = ack
	c.pendingAcksMu.Unlock()
}

func (c *Cache) processAckMessage(nodeID uint64, msg *service.Message) {
	c.pendingAcksMu.Lock()
	defer c.pendingAcksMu.Unlock()
	if m, ok := c.pendingAcks[AckID{nodeID, msg.ID}]; ok {
		if m.cb != nil {
			m.cb(m.nodeID, msg)
		}
		delete(c.pendingAcks, AckID{nodeID, msg.ID})
	} else {
		log.Error("unknown lastID", log.Uint64("lastid", msg.ID))
		if log.AtDebug() {
			log.Debug("unknown lastID", log.Uint64("lastid", msg.ID))
		}
	}
}

func (c *Cache) readAckMessage(nodeID uint64, conn *network.Conn, die chan bool) {
	for {
		select {
		case _ = <-die:
			log.Error("KILLING READ ACK", fld.PeerID(nodeID))
			return
		default:
			msg, err := conn.ReadMessage(int(c.maxPayload), 5*time.Second)
			// if we can read some message, try to process it.
			if err == nil {
				go c.processAckMessage(nodeID, msg)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (c *Cache) retryRequests() {
	for {
		redolist := []PendingAck{}
		time.Sleep(5 * time.Second)
		c.pendingAcksMu.Lock()
		for k, v := range c.pendingAcks {
			if time.Since(v.sentAt) >= 5*time.Second {
				redolist = append(redolist, v)
				delete(c.pendingAcks, k)
			}
		}
		c.pendingAcksMu.Unlock()
		for _, v := range redolist {
			c.WriteRequest(v.nodeID, v.msg, v.timeout, true, v.cb)
		}
	}
}

func NewCache(nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair, connection service.CONNECTION) *Cache {
	c := &Cache{
		conns:       map[uint64]*MuConn{},
		cmu:         make([]sync.Mutex, top.TotalNodes()),
		maxPayload:  maxPayload,
		selfID:      nodeID,
		top:         top,
		key:         key,
		pendingAcks: map[AckID]PendingAck{},
		connection:  connection,
	}
	go c.retryRequests()

	return c
}

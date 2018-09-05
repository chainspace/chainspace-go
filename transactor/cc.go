package transactor

import (
	"sync"
	"time"

	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
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
	cb      func(*service.Message)
}

type ConnsCache struct {
	conns      map[uint64]*MuConn
	connsmu    []sync.Mutex
	mu         sync.Mutex
	key        signature.KeyPair
	maxPayload int
	selfID     uint64
	top        *network.Topology

	pendingAcks   map[uint64]PendingAck
	pendingAcksMu sync.Mutex
}

func (c *ConnsCache) sendHello(nodeID uint64, conn *network.Conn) error {
	hellomsg, err := service.SignHello(
		c.selfID, nodeID, c.key, service.CONNECTION_TRANSACTOR)
	if err != nil {
		return err
	}
	return conn.WritePayload(hellomsg, c.maxPayload, time.Second)
}

func (c *ConnsCache) dial(nodeID uint64) (*MuConn, error) {
	// conn exist
	cc, ok := c.conns[nodeID]
	if ok {
		return cc, nil
	}
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
	go c.readAckMessage(cc.conn, cc.die)
	c.conns[nodeID] = cc
	return cc, nil
}

func (c *ConnsCache) release(nodeID uint64) {
	cc, ok := c.conns[nodeID]
	if ok {
		cc.die <- true
		delete(c.conns, nodeID)
	}
}

func (c *ConnsCache) WriteRequest(
	nodeID uint64, msg *service.Message, timeout time.Duration, ack bool) (uint64, error) {
	c.mu.Lock()
	mc, err := c.dial(nodeID)
	if err != nil {
		// FIXME(): handle this better
		c.release(nodeID)
		c.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
		return c.WriteRequest(nodeID, msg, timeout, ack)
	}
	id, err := mc.conn.WriteRequest(msg, c.maxPayload, timeout)
	if err != nil {
		c.release(nodeID)
		c.mu.Unlock()
		return c.WriteRequest(nodeID, msg, timeout, ack)
	}
	c.mu.Unlock()
	if ack {
		c.addPendingAck(nodeID, msg, timeout, id)
	}
	return id, nil
}

func (c *ConnsCache) addPendingAck(nodeID uint64, msg *service.Message, timeout time.Duration, id uint64) {
	ack := PendingAck{
		sentAt:  time.Now(),
		nodeID:  nodeID,
		msg:     msg,
		timeout: timeout,
		cb:      nil,
	}
	c.pendingAcksMu.Lock()
	c.pendingAcks[id] = ack
	c.pendingAcksMu.Unlock()
}

func (c *ConnsCache) processAckMessage(msg *service.Message) {
	c.pendingAcksMu.Lock()
	defer c.pendingAcksMu.Unlock()
	if m, ok := c.pendingAcks[msg.ID]; ok {
		if m.cb != nil {
			m.cb(msg)
		}
		delete(c.pendingAcks, msg.ID)
	} else {
		if log.AtDebug() {
			log.Debug("unknown lastID", log.Uint64("lastid", msg.ID))
		}
	}
}

func (c *ConnsCache) readAckMessage(conn *network.Conn, die chan bool) {
	for {
		select {
		case _ = <-die:
			return
		default:
			msg, err := conn.ReadMessage(int(c.maxPayload), 5*time.Second)
			// if we can read some message, try to process it.
			if err == nil {
				go c.processAckMessage(msg)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (c *ConnsCache) retryRequests() {
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
			c.WriteRequest(v.nodeID, v.msg, v.timeout, true)
		}
	}
}

func NewConnsCache(nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair) *ConnsCache {
	c := &ConnsCache{
		conns:       map[uint64]*MuConn{},
		connsmu:     make([]sync.Mutex, top.TotalNodes()),
		maxPayload:  maxPayload,
		selfID:      nodeID,
		top:         top,
		key:         key,
		pendingAcks: map[uint64]PendingAck{},
	}
	go c.retryRequests()

	return c
}

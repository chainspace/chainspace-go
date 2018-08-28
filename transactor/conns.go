package transactor // import "chainspace.io/prototype/transactor"

import (
	"context"
	"sync"
	"time"

	"chainspace.io/prototype/crypto/signature"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
)

type ConnChan struct {
	conn *network.Conn
	stop chan bool
}

type ConnsCache struct {
	conns      map[uint64]*ConnChan
	ctx        context.Context
	key        signature.KeyPair
	maxPayload int
	mu         sync.Mutex
	selfID     uint64
	top        *network.Topology
	msgAcks    map[uint64]MessageAck
	msgAcksmu  sync.Mutex
}

type MessageAck struct {
	nodeID  uint64
	msg     *service.Message
	sentAt  time.Time
	timeout time.Duration
}

func (c *ConnsCache) Close() error {
	for _, v := range c.conns {
		if err := v.conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ConnsCache) sendHello(nodeID uint64, conn *network.Conn) error {
	hellomsg, err := service.SignHello(c.selfID, nodeID, c.key, service.CONNECTION_TRANSACTOR)
	if err != nil {
		return err
	}
	return conn.WritePayload(hellomsg, c.maxPayload, time.Second)
}

func (c *ConnsCache) dial(nodeID uint64) (*ConnChan, error) {
	c.mu.Lock()
	cc, ok := c.conns[nodeID]
	c.mu.Unlock()
	if ok {
		return cc, nil
	}
	conn, err := c.top.Dial(nodeID, 5*time.Second)
	if err != nil {
		return nil, err
	}
	err = c.sendHello(nodeID, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	stop := make(chan bool)
	cc = &ConnChan{conn, stop}
	go c.readAck(cc.conn, stop)
	c.mu.Lock()
	c.conns[nodeID] = cc
	c.mu.Unlock()
	return cc, nil
}

func (c *ConnsCache) release(nodeID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.conns, nodeID)
}

func (c *ConnsCache) WriteRequest(nodeID uint64, msg *service.Message, timeout time.Duration, ack bool) (uint64, error) {
	cc, err := c.dial(nodeID)
	if err != nil {
		return 0, err
	}
	id, err := cc.conn.WriteRequest(msg, c.maxPayload, timeout)
	if err != nil {
		cc.stop <- true
		c.release(nodeID)
		return c.WriteRequest(nodeID, msg, timeout, ack)
	}
	if ack {
		c.addMessageAck(nodeID, msg, timeout, id)
	}
	return id, err
}

func (c *ConnsCache) addMessageAck(nodeID uint64, msg *service.Message, timeout time.Duration, id uint64) {
	mack := MessageAck{
		sentAt:  time.Now(),
		nodeID:  nodeID,
		msg:     msg,
		timeout: timeout,
	}
	c.msgAcksmu.Lock()
	c.msgAcks[id] = mack
	c.msgAcksmu.Unlock()
}

func (c *ConnsCache) check() {
	for {
		redolist := []MessageAck{}
		time.Sleep(500 * time.Millisecond)
		c.msgAcksmu.Lock()
		for k, v := range c.msgAcks {
			// resend message
			if time.Since(v.sentAt) >= 1*time.Second {
				redolist = append(redolist, v)
				delete(c.msgAcks, k)
			}
		}
		c.msgAcksmu.Unlock()
		for _, v := range redolist {
			c.WriteRequest(v.nodeID, v.msg, v.timeout, true)
		}
	}
}

func (c *ConnsCache) readAck(conn *network.Conn, stop chan bool) {
	for {
		select {
		case _ = <-stop:
			break
		default:
			rmsg, err := conn.ReadMessage(int(c.maxPayload), 5*time.Second)
			if err == nil {
				msgack := SBACMessageAck{}
				err := proto.Unmarshal(rmsg.Payload, &msgack)
				// TODO(): stream closed or sumthing
				if err != nil {
					log.Error("connscache: invalid proto", fld.Err(err))
				}
				c.msgAcksmu.Lock()
				_, ok := c.msgAcks[msgack.LastID]
				if !ok {
					// unknow lastID
					if log.AtDebug() {
						log.Debug("unknown lastID", log.Uint64("lastid", msgack.LastID))
					}
				} else {
					delete(c.msgAcks, msgack.LastID)
				}
				c.msgAcksmu.Unlock()

			}
		}
	}
}

func NewConnsCache(ctx context.Context, nodeID uint64, top *network.Topology, maxPayload int, key signature.KeyPair) *ConnsCache {
	c := &ConnsCache{
		conns:      map[uint64]*ConnChan{},
		ctx:        ctx,
		maxPayload: maxPayload,
		selfID:     nodeID,
		top:        top,
		key:        key,
		msgAcks:    map[uint64]MessageAck{},
	}

	go c.check()
	return c
}

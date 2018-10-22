package internal

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"time"
)

type Payload struct {
	ObjectID string `json:"object_id"`
	Success  bool   `json:"success"`
	NodeID   uint64 `json:"node_id"`
}

type Conn struct {
	buf  []byte
	conn net.Conn
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) Write(payload []byte, timeout time.Duration) error {
	size := len(payload)
	buf := c.buf
	binary.LittleEndian.PutUint32(buf, uint32(size))
	need := 4
	for need > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.conn.Write(buf)
		if err != nil {
			c.conn.Close()
			return err
		}
		buf = buf[n:]
		need -= n
	}
	for size > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.conn.Write(payload)
		if err != nil {
			c.conn.Close()
			return err
		}
		payload = payload[n:]
		size -= n
	}
	return nil
}

func (c *Conn) Read(timeout time.Duration) (*Payload, error) {
	buf := c.buf
	need := 4
	for need > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.conn.Read(buf[4-need : 4])
		if err != nil {
			c.conn.Close()
			return nil, err
		}
		need -= n
	}
	size := int(int32(binary.LittleEndian.Uint32(buf)))
	if len(buf) < size {
		buf = make([]byte, size)
		c.buf = buf
	}
	buf = buf[:size]
	need = size
	for need > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.conn.Read(buf[size-need:])
		if err != nil {
			c.conn.Close()
			return nil, err
		}
		need -= n
	}

	var payload Payload
	return &payload, json.Unmarshal(buf, &payload)
}

func NewConn(c net.Conn) *Conn {
	return &Conn{
		buf:  make([]byte, 1024),
		conn: c,
	}
}

package internal // import "chainspace.io/prototype/pubsub/internal"

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"time"
)

type Payload struct {
	ObjectID string   `json:"object_id"`
	Success  bool     `json:"success"`
	NodeID   uint64   `json:"node_id"`
	Labels   []string `json:"labels"`
}

type Conn struct {
	buf  []byte
	conn net.Conn
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) Read(timeout time.Duration) (*Payload, error) {
	buf := c.buf
	need := 4
	for need > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.conn.Read(buf[4-need : 4])
		e, ok := err.(net.Error)
		if (ok && e.Timeout()) || err == io.EOF {
			return nil, nil
		}
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
		e, ok := err.(net.Error)
		if (ok && e.Timeout()) || err == io.EOF {
			return nil, nil
		}
		if err != nil {
			c.conn.Close()
			return nil, err
		}
		need -= n
	}

	var payload Payload
	return &payload, json.Unmarshal(buf, &payload)
}

func (c *Conn) Write(payload []byte, timeout time.Duration) error {
	buf := make([]byte, len(payload)+4)
	binary.LittleEndian.PutUint32(buf, uint32(len(payload)))
	copy(buf[4:], payload)
	need := len(buf)
	for need > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.conn.Write(buf)
		e, ok := err.(net.Error)
		if (ok && e.Timeout()) || err == io.EOF {
			return nil
		}
		if err != nil {
			c.conn.Close()
			return err
		}
		buf = buf[n:]
		need -= n
	}
	return nil
}

func NewConn(c net.Conn) *Conn {
	return &Conn{
		buf:  make([]byte, 1024),
		conn: c,
	}
}

package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
	"github.com/tav/golly/log"
)

type timeoutError interface {
	Timeout() bool
}

// Conn represents a connection to a Chainspace node.
type Conn struct {
	buf    []byte
	conn   net.Conn
	lastID uint64
	mu     sync.Mutex
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// ReadHello reads a service.Hello from the underlying connection.
func (c *Conn) ReadHello(limit int, timeout time.Duration) (*service.Hello, error) {
	payload, err := c.ReadPayload(limit, timeout)
	if err != nil {
		return nil, err
	}
	hello := &service.Hello{}
	err = proto.Unmarshal(payload, hello)
	return hello, err
}

// ReadMessage reads a service.Message from the underlying connection.
func (c *Conn) ReadMessage(limit int, timeout time.Duration) (*service.Message, error) {
	payload, err := c.ReadPayload(limit, timeout)
	if err != nil {
		return nil, err
	}
	msg := &service.Message{}
	err = proto.Unmarshal(payload, msg)
	return msg, err
}

// ReadPayload returns the next protobuf-marshalled payload from the underlying
// connection.
func (c *Conn) ReadPayload(limit int, timeout time.Duration) ([]byte, error) {
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
	if size < 0 {
		c.conn.Close()
		return nil, fmt.Errorf("network: payload size overflows an int32")
	}
	if size > limit {
		c.conn.Close()
		return nil, fmt.Errorf("network: payload size %d exceeds max payload size", size)
	}
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
	return buf, nil
}

// WritePayload encodes the given payload into protobuf and writes the
// marshalled data to the underlying connection, along with a header indicating
// the length of the data.
func (c *Conn) WritePayload(pb proto.Message, limit int, timeout time.Duration) error {
	if pb == nil {
		log.Error("----------------- PB NIL -------------------")
	}
	switch x := pb.(type) {
	case *service.Message:
		if x.Opcode == 0 {
			log.Error("-------------------------- 0 OPCODE ----------------")
			debug.PrintStack()
		}
	case *service.Hello:
	default:
		log.Error("-------------------------- WEIRD MESSAGE ----------------")
	}

	payload, err := proto.Marshal(pb)
	if err != nil {
		c.conn.Close()
		return err
	}
	size := len(payload)
	if size > limit {
		c.conn.Close()
		return fmt.Errorf("network: payload size %d exceeds max payload size", size)
	}
	buf := c.buf[:4]
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

// WriteRequest adds a connection-specific ID to the given message before
// writing it out on the underlying connection.
func (c *Conn) WriteRequest(msg *service.Message, limit int, timeout time.Duration) (id uint64, err error) {
	c.mu.Lock()
	c.lastID++
	id = c.lastID
	c.mu.Unlock()
	msg.ID = id
	return id, c.WritePayload(msg, limit, timeout)
}

// AbnormalError returns whether the given error was abnormal, i.e. not EOF or a
// timeout error.
func AbnormalError(err error) bool {
	if err == io.EOF {
		return false
	}
	if nerr, ok := err.(timeoutError); ok && nerr.Timeout() {
		return false
	}
	return true
}

// NewConn instantiates a Conn value with the given network connection.
func NewConn(conn net.Conn) *Conn {
	return &Conn{
		buf:  make([]byte, 1024),
		conn: conn,
	}
}

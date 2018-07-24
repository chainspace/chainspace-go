package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"go.uber.org/zap"
)

// Conn represents a connection to a Chainspace node.
type Conn struct {
	lastID uint64
	mu     sync.Mutex
	stream quic.Stream
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.stream.Close()
}

// Context returns the context associated with the underlying connection.
func (c *Conn) Context() context.Context {
	return c.stream.Context()
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
	buf := make([]byte, 4)
	need := 4
	for need > 0 {
		c.stream.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.stream.Read(buf[4-need:])
		if err != nil {
			log.Error("Got error reading from stream", zap.Error(err))
			c.stream.Close()
			return nil, err
		}
		need -= n
	}
	// TODO(tav): Fix for 32-bit systems.
	size := int(binary.LittleEndian.Uint32(buf))
	if size > limit {
		log.Error("Max payload size exceeded", zap.Int("limit", limit), zap.Int("got", size))
		return nil, fmt.Errorf("network: payload size %d exceeds max payload size", size)
	}
	buf = make([]byte, size)
	need = size
	for need > 0 {
		c.stream.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.stream.Read(buf[size-need:])
		if err != nil {
			log.Error("Got error reading from stream", zap.Error(err))
			c.stream.Close()
			return nil, err
		}
		need -= n
	}
	return buf, nil
}

// Write writes the given bytes to the underlying connection.
func (c *Conn) Write(p []byte) (int, error) {
	return c.stream.Write(p)
}

// WritePayload encodes the given payload into protobuf and writes the
// marshalled data to the underlying connection, along with a header indicating
// the length of the data.
func (c *Conn) WritePayload(pb proto.Message, limit int, timeout time.Duration) error {
	payload, err := proto.Marshal(pb)
	if err != nil {
		return err
	}
	size := len(payload)
	if size > limit {
		log.Error("Max payload size exceeded", zap.Int("limit", limit), zap.Int("got", size))
		return fmt.Errorf("network: payload size %d exceeds max payload size", size)
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(size))
	need := 4
	for need > 0 {
		c.stream.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.stream.Write(buf)
		if err != nil {
			c.stream.Close()
			return err
		}
		buf = buf[n:]
		need -= n
	}
	for size > 0 {
		c.stream.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.stream.Write(payload)
		if err != nil {
			c.stream.Close()
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

// NewConn instantiates a connection value with the given QUIC stream.
func NewConn(stream quic.Stream) *Conn {
	return &Conn{
		stream: stream,
	}
}

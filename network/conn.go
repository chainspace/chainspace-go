package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"chainspace.io/prototype/log"
	"chainspace.io/prototype/service"
	"github.com/gogo/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
)

const (
	maxPayload = 1 << 27 // 128MB
)

// Conn represents a connection to a Chainspace node.
type Conn struct {
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

// Read reads from the underlying connection.
func (c *Conn) Read(p []byte) (int, error) {
	return c.stream.Read(p)
}

// ReadHello reads a service.Hello from the underlying connection.
func (c *Conn) ReadHello(timeout time.Duration) (*service.Hello, error) {
	payload, err := c.ReadPayload(timeout)
	if err != nil {
		return nil, err
	}
	hello := &service.Hello{}
	err = proto.Unmarshal(payload, hello)
	return hello, err
}

// ReadMessage reads a service.Message from the underlying connection.
func (c *Conn) ReadMessage(timeout time.Duration) (*service.Message, error) {
	payload, err := c.ReadPayload(timeout)
	if err != nil {
		return nil, err
	}
	msg := &service.Message{}
	err = proto.Unmarshal(payload, msg)
	return msg, err
}

// ReadPayload returns the next protobuf-marshalled payload from the underlying
// connection.
func (c *Conn) ReadPayload(timeout time.Duration) ([]byte, error) {
	buf := make([]byte, 4)
	need := 4
	for need > 0 {
		c.stream.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.stream.Read(buf[4-need:])
		if err != nil {
			log.Errorf("Got error reading from stream: %s", err)
			return nil, err
		}
		need -= n
	}
	// TODO(tav): Fix for 32-bit systems.
	size := int(binary.LittleEndian.Uint32(buf))
	if size > maxPayload {
		log.Errorf("Payload size of %d exceeds the max payload size", size)
		return nil, fmt.Errorf("network: payload size %d exceeds max payload size", size)
	}
	buf = make([]byte, size)
	need = size
	for need > 0 {
		c.stream.SetReadDeadline(time.Now().Add(timeout))
		n, err := c.stream.Read(buf[size-need:])
		if err != nil {
			log.Errorf("Got error reading from stream: %s", err)
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
func (c *Conn) WritePayload(pb proto.Message, timeout time.Duration) error {
	payload, err := proto.Marshal(pb)
	if err != nil {
		return err
	}
	size := len(payload)
	if size > maxPayload {
		log.Errorf("Payload size of %d exceeds the max payload size", size)
		return fmt.Errorf("network: payload size %d exceeds max payload size", size)
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(size))
	need := 4
	for need > 0 {
		c.stream.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.stream.Write(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
		need -= n
	}
	for size > 0 {
		c.stream.SetWriteDeadline(time.Now().Add(timeout))
		n, err := c.stream.Write(payload)
		if err != nil {
			return err
		}
		payload = payload[n:]
		size -= n
	}
	return nil
}

// NewConn instantiates a connection value with the given QUIC stream.
func NewConn(stream quic.Stream) *Conn {
	return &Conn{
		stream: stream,
	}
}

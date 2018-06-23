package network

import (
	"sync"

	"github.com/lucas-clemente/quic-go"
)

// Conn represents a connection to a Chainspace node.
type Conn struct {
	mu        sync.Mutex
	nonceBase []byte
	nonceID   uint64
	stream    quic.Stream
}

func (c *Conn) readLoop() {
	for {

	}
}

func (c *Conn) Write(p []byte) (int, error) {
	return c.stream.Write(p)
}

func initConn(nodeID uint64, stream quic.Stream) *Conn {
	return &Conn{
		stream: stream,
	}
}

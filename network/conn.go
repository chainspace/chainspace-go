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
	session   quic.Session
}

func (c *Conn) readLoop() {
	for {

	}
}

// Package service defines the generic interface for node services.
package service // import "chainspace.io/prototype/service"

import (
	"context"
	"crypto/rand"
	"time"

	"chainspace.io/prototype/crypto/signature"
	"github.com/gogo/protobuf/proto"
)

// Handler specifies the interface for a node service.
type Handler interface {
	Handle(ctx context.Context, peerID uint64, msg *Message) (*Message, error)
	Name() string
}

// BroadcastHello returns a signed payload for use as a Hello in a broadcast connection.
func BroadcastHello(clientID uint64, serverID uint64, key signature.KeyPair) (*Hello, error) {
	nonce := make([]byte, 36)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	payload, err := proto.Marshal(&HelloInfo{
		Client:    clientID,
		Nonce:     nonce,
		Server:    serverID,
		Timestamp: time.Now(),
	})
	if err != nil {
		return nil, err
	}
	return &Hello{
		Agent:     "go/0.0.1",
		Payload:   payload,
		Signature: key.Sign(payload),
		Type:      CONNECTION_BROADCAST,
	}, nil
}
// Package service defines the generic interface for node services.
package service // import "chainspace.io/chainspace-go/service"

import (
	"crypto/rand"
	"time"

	"chainspace.io/chainspace-go/internal/crypto/signature"

	"github.com/gogo/protobuf/proto"
)

// Handler specifies the interface for a node service.
type Handler interface {
	Handle(peerID uint64, msg *Message) (*Message, error)
	Name() string
}

// SignHello returns a signed payload for use as a Hello in a service
// connection.
func SignHello(clientID uint64, serverID uint64, key signature.KeyPair, c CONNECTION) (*Hello, error) {
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
		Type:      c,
	}, nil
}

// EncodeMessage takes a protobuf-compatible struct and encodes it into a
// service Message.
func EncodeMessage(opcode int32, pb proto.Message) *Message {
	payload, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return &Message{
		Opcode:  opcode,
		Payload: payload,
	}
}

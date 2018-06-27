// Package service defines the generic interface for node services.
package service // import "chainspace.io/prototype/service"

import (
	"context"
)

// Handler specifies the interface for a node service.
type Handler interface {
	Handle(context.Context, *Message) (*Message, error)
	Name() string
}

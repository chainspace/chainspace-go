// Package storage defines interfaces to the underlying data storage methods.
package storage // import "chainspace.io/prototype/storage"

// DB specifies the interface implemented by all Chainspace storage methods.
type DB interface {
	Sync() error
}

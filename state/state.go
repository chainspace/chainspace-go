// Package state implements the underlying state machine for a shard.
package state // import "chainspace.io/prototype/state"

import (
	"chainspace.io/prototype/storage"
)

// Machine represents the state machine for a shard.
type Machine struct {
	db storage.DB
}

// Load initialises a state machine from the state stored in the given DB.
func Load(db storage.DB) *Machine {
	return &Machine{
		db: db,
	}
}

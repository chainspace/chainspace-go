// Package memstore implements an in-memory version of the Chainspace datastore.
package memstore

// DB represents the in-memory datastore.
type DB struct {
	kv map[string][]byte
}

// New initialises a new memstore DB.
func New() *DB {
	db := &DB{
		kv: map[string][]byte{},
	}
	return db
}

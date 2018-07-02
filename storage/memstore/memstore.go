// Package memstore implements an in-memory version of the Chainspace datastore.
package memstore

// DB represents the in-memory datastore.
type DB struct {
	kv map[string][]byte
}

// Sync flushes the data to any underlying storage. It's a no-op in the case of
// the memstore.
func (d *DB) Sync() error {
	return nil
}

// New initialises a new memstore DB.
func New() *DB {
	db := &DB{
		kv: map[string][]byte{},
	}
	return db
}

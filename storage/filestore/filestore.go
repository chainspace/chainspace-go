// Package filestore implements a version of the Chainspace datastore that
// persists to disk.
package filestore

// DB represents the filestore.
type DB struct {
}

// New initialises a new filestore DB.
func New() *DB {
	db := &DB{}
	return db
}

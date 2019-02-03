package broadcast

import (
	"path/filepath"

	"chainspace.io/chainspace-go/blockmania"
	"github.com/dgraph-io/badger"
)

// Replay loads all blocks from the given start point and replays them onto the
// graph in the given batch size.
func Replay(dir string, nodeID uint64, g *blockmania.Graph, start uint64, batch int) error {
	opts := badger.DefaultOptions
	opts.Dir = filepath.Join(dir, "broadcast")
	opts.ValueDir = opts.Dir
	db, err := badger.Open(opts)
	if err != nil {
		return err
	}
	datastore := &store{
		db: db,
	}
	var from uint64
	for {
		blocks, err := datastore.getBlockGraphs(nodeID, from, batch)
		if err != nil {
			return err
		}
		for _, block := range blocks {
			g.Add(block)
		}
		if len(blocks) < batch {
			break
		}
		from += uint64(batch)
	}
	return nil
}

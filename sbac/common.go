package sbac

import (
	fmt "fmt"
	"hash/fnv"
	"time"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/service"
	proto "github.com/gogo/protobuf/proto"
)

func (s *ServiceSBAC) inputObjectsForShard(
	shardID uint64, tx *Transaction) (objects [][]byte, allInShard bool) {
	// get all objects part of this current shard state
	allInShard = true
	for _, t := range tx.Traces {
		for _, o := range t.InputObjectVersionIDs {
			o := o
			if shardID := s.top.ShardForVersionID(o); shardID == s.shardID {
				objects = append(objects, o)
				continue
			}
			allInShard = false
		}
		for _, ref := range t.InputReferenceVersionIDs {
			ref := ref
			if shardID := s.top.ShardForVersionID(ref); shardID == s.shardID {
				continue
			}
			allInShard = false
		}
	}
	return
}

// FIXME(): need to find a more reliable way to do this
func (s *ServiceSBAC) isNodeInitiatingBroadcast(txID uint32) bool {
	nodesInShard := s.top.NodesInShard(s.shardID)
	n := nodesInShard[txID%(uint32(len(nodesInShard)))]
	log.Debug("consensus will be started", log.Uint64("peer", n))
	return n == s.nodeID
}

func (s *ServiceSBAC) objectsExists(vids, refvids [][]byte) ([]*Object, bool) {
	ownvids := [][]byte{}
	for _, v := range append(vids, refvids...) {
		if s.top.ShardForVersionID(v) == s.shardID {
			ownvids = append(ownvids, v)
		}
	}

	objects, err := s.store.GetObjects(ownvids)
	if err != nil {
		return nil, false
	}
	return objects, true
}

func (s *ServiceSBAC) publishObjects(ids *IDs, success bool) {
	for _, topair := range ids.TraceObjectPairs {
		for _, outo := range topair.OutputObjects {
			shard := s.top.ShardForVersionID(outo.GetVersionID())
			if shard == s.shardID {
				s.ps.Publish(outo.VersionID, outo.Labels, success)
			}
		}
	}
}

func (s *ServiceSBAC) saveLabels(ids *IDs) error {
	for _, topair := range ids.TraceObjectPairs {
		for _, outo := range topair.OutputObjects {
			shard := s.top.ShardForVersionID(outo.GetVersionID())
			if shard == s.shardID {
				for _, label := range outo.Labels {
					s.kvstore.Set([]byte(label), outo.VersionID)
				}
			}
		}
	}

	return nil
}

func (s *ServiceSBAC) sendToAllShardInvolved(st *States, msg *service.Message) error {
	shards := s.shardsInvolvedWithoutSelf(st.detail.Tx)
	return s.sendToShards(shards, st, msg)
}

func (s *ServiceSBAC) sendToShards(shards []uint64, st *States, msg *service.Message) error {
	conns := s.conns.Borrow()
	for _, shard := range shards {
		shardid := shard
		nodes := s.top.NodesInShard(shardid)
		for _, node := range nodes {
			msgcpy := *msg
			nodeid := node

			go func() {
				_, err := conns.WriteRequest(nodeid, &msgcpy, 3*time.Second, true, nil)
				if err != nil {
					log.Error("unable to connect to node",
						fld.TxID(st.detail.HashID), fld.PeerID(nodeid))
					//return fmt.Errorf("unable to connect to node(%v): %v", node, err)
				}
			}()
		}
	}
	return nil
}

// shardsInvolved return a list of IDs of all shards involved in the transaction either by
// holding state for an input object or input reference.
func (s *ServiceSBAC) shardsInvolved(tx *Transaction) []uint64 {
	uniqids := map[uint64]struct{}{}
	for _, trace := range tx.Traces {
		for _, obj := range trace.InputObjectVersionIDs {
			uniqids[s.top.ShardForVersionID(obj)] = struct{}{}
		}
		for _, ref := range trace.InputReferenceVersionIDs {
			uniqids[s.top.ShardForVersionID(ref)] = struct{}{}
		}
	}
	ids := make([]uint64, 0, len(uniqids))
	for k, _ := range uniqids {
		ids = append(ids, k)
	}
	return ids
}

func (s *ServiceSBAC) shardsInvolvedWithoutSelf(tx *Transaction) []uint64 {
	shards := s.shardsInvolved(tx)
	for i, _ := range shards {
		if shards[i] == s.shardID {
			shards = append(shards[:i], shards[i+1:]...)
			break
		}
	}
	return shards
}

func (s *ServiceSBAC) signTransaction(tx *Transaction) ([]byte, error) {
	b, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal transaction, %v", err)
	}
	return s.privkey.Sign(b), nil
}

func (s *ServiceSBAC) signTransactionRaw(tx []byte) ([]byte, error) {
	return s.privkey.Sign(tx), nil
}

func (s *ServiceSBAC) verifyEvidenceSignature(txID []byte, evidences map[uint64][]byte) bool {
	sig, ok := evidences[s.nodeID]
	if !ok {
		log.Error("missing self signature")
		return false
	}

	key, ok := s.top.SeedPublicKeys()[s.nodeID]
	if !ok {
		log.Error("missing key for self signature")
	}

	valid := key.Verify(txID, sig)
	if !valid {
		log.Errorf("invalid signature")
	}

	return valid
}

func (s *ServiceSBAC) verifyTransactionRawSignature(
	tx []byte, signature []byte, nodeID uint64) (bool, error) {
	keys := s.top.SeedPublicKeys()
	key := keys[nodeID]
	return key.Verify(tx, signature), nil
}

func (s *ServiceSBAC) verifyTransactionSignature(
	tx *Transaction, signature []byte, nodeID uint64) (bool, error) {
	b, err := proto.Marshal(tx)
	if err != nil {
		return false, err
	}
	keys := s.top.SeedPublicKeys()
	key := keys[nodeID]
	return key.Verify(b, signature), nil
}

func fromUniqIds(m map[uint64]struct{}) []uint64 {
	out := []uint64{}
	for k, _ := range m {
		out = append(out, k)
	}
	return out
}

func ID(data []byte) uint32 {
	h := fnv.New32()
	h.Write(data)
	return h.Sum32()
}

func quorum2t1(shardSize uint64) uint64 {
	return (2*(shardSize/3) + 1)
}

func quorumt1(shardSize uint64) uint64 {
	return shardSize/3 + 1
}

func toUniqIds(ls []uint64) map[uint64]struct{} {
	out := map[uint64]struct{}{}
	for _, v := range ls {
		out[v] = struct{}{}
	}
	return out
}

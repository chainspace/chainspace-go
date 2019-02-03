package api // import "chainspace.io/chainspace-go/sbac/api"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"chainspace.io/chainspace-go/checker"
	checkerclient "chainspace.io/chainspace-go/checker/client"
	"chainspace.io/chainspace-go/network"
	"chainspace.io/chainspace-go/sbac"
	sbacclient "chainspace.io/chainspace-go/sbac/client"
)

type service struct {
	checker    checker.Service
	checkerclt *checkerclient.Client
	nodeID     uint64
	sbac       *sbac.ServiceSBAC
	sbacclt    sbacclient.Client
	shardID    uint64
	top        *network.Topology
	validator  TransactionValidator
}

// newService ...
func newService(cfg *Config) *service {
	return &service{
		checker:    cfg.Checker,
		checkerclt: cfg.Checkerclt,
		nodeID:     cfg.NodeID,
		sbac:       cfg.Sbac,
		sbacclt:    cfg.Sbacclt,
		shardID:    cfg.ShardID,
		top:        cfg.Top,
		validator:  Validator{},
	}
}

func (srv *service) CreateObject(obj interface{}) (string, int, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", http.StatusBadRequest, err
	}

	ids, err := srv.sbacclt.Create(b)
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	for _, v := range ids {
		if string(v) != string(ids[0]) {
			return "", http.StatusInternalServerError, err
		}
	}

	return base64.StdEncoding.EncodeToString(ids[0]), http.StatusOK, nil
}

func (srv *service) shardsForTraces(
	shards map[uint64]struct{}, ts []*sbac.Trace) map[uint64]struct{} {
	for _, t := range ts {
		for _, vid := range t.InputObjectVersionIDs {
			shards[srv.top.ShardForVersionID(vid)] = struct{}{}
		}
		for _, vid := range t.InputReferenceVersionIDs {
			shards[srv.top.ShardForVersionID(vid)] = struct{}{}
		}
		shards = srv.shardsForTraces(shards, t.Dependencies)
	}
	return shards

}

func (srv *service) shardsForTx(tx *sbac.Transaction) []uint64 {
	ids := srv.shardsForTraces(map[uint64]struct{}{}, tx.Traces)
	out := []uint64{}
	for id := range ids {
		out = append(out, id)
	}
	return out
}

func (srv *service) sendTransaction(ctx context.Context, tx *sbac.Transaction, evidences map[uint64][]byte) ([]*sbac.Object, error) {
	var objects []*sbac.Object
	var err error
	shards := srv.shardsForTx(tx)
	peerids := []uint64{}
	var self bool
	for _, shrd := range shards {
		if shrd == srv.shardID {
			self = true
		} else {
			peerids = append(peerids, srv.top.RandNodeInShard(shrd))
		}
	}
	if self {
		objects, err = srv.sbac.AddTransaction(ctx, tx, evidences)
		if err != nil {
			return nil, err
		}
	}
	if len(peerids) > 0 {
		objects, err = srv.sbacclt.AddTransaction(peerids, tx, evidences)
		if err != nil {
			return nil, err
		}
	}

	return objects, nil
}

// Add adds a transaction to a shard
func (srv *service) Add(ctx context.Context, tx *Transaction) (interface{}, int, error) {
	// require at least input object.
	for _, v := range tx.Traces {
		if len(v.InputObjectVersionIDs) <= 0 {
			return nil, http.StatusBadRequest, errors.New("no input objects for a trace")
		}
	}

	sbactx, err := tx.ToSBAC(srv.validator)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	evidences, err := srv.checkerclt.Check(sbactx)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	objects, err := srv.sendTransaction(ctx, sbactx, evidences)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return srv.buildObject(objects)
}

func (srv *service) AddChecked(ctx context.Context, tx *Transaction) (interface{}, int, error) {
	// require at least input object.
	for _, v := range tx.Traces {
		if len(v.InputObjectVersionIDs) <= 0 {
			return nil, http.StatusBadRequest, errors.New("no input objects for a trace")
		}
	}

	sbactx, err := tx.ToSBAC(srv.validator)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	objects := []*sbac.Object{}
	shards := srv.shardsForTx(sbactx)
	peerids := []uint64{}
	signatures, err := signaturesToBytes(tx.Signatures)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}

	var self bool
	for _, shrd := range shards {
		if shrd == srv.shardID {
			self = true
		} else {
			peerids = append(peerids, srv.top.RandNodeInShard(shrd))
		}
	}
	if self {
		objects, err = srv.sbac.AddTransaction(ctx, sbactx, signatures)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	}
	if len(peerids) > 0 {
		objects, err = srv.sbacclt.AddTransaction(peerids, sbactx, signatures)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	}

	return srv.buildObject(objects)
}

func (srv *service) buildObject(objects []*sbac.Object) (interface{}, int, error) {
	data := []Object{}
	for _, v := range objects {
		v := v
		o := Object{
			VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
			Value:     string(v.Value),
			Status:    v.Status.String(),
		}
		data = append(data, o)
	}
	return data, http.StatusOK, nil
}

func (srv *service) States(ctx context.Context) (interface{}, int, error) {
	states := srv.sbac.StatesReport(ctx)
	sort.Slice(states.States, func(i, j int) bool { return states.States[i].HashID < states.States[j].HashID })
	return states, http.StatusOK, nil
}

func signaturesToBytes(signs map[uint64]string) (map[uint64][]byte, error) {
	out := map[uint64][]byte{}
	for k, v := range signs {
		sbytes, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, err
		}
		out[k] = sbytes
	}
	return out, nil
}

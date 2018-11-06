package restsrv // import "chainspace.io/prototype/restsrv"

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"sort"
	"strings"

	"chainspace.io/prototype/checker"
	checkerclient "chainspace.io/prototype/checker/client"
	"chainspace.io/prototype/config"
	"chainspace.io/prototype/internal/crypto/signature"
	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/internal/log/fld"
	"chainspace.io/prototype/kv"
	"chainspace.io/prototype/network"
	"chainspace.io/prototype/sbac"
	sbacclient "chainspace.io/prototype/sbac/client"

	"github.com/rs/cors"
)

type Config struct {
	Addr        string
	Key         signature.KeyPair
	Port        int
	Top         *network.Topology
	MaxPayload  config.ByteSize
	SelfID      uint64
	Store       kv.Service
	SBAC        *sbac.Service
	Checker     *checker.Service
	SBACOnly    bool
	CheckerOnly bool
}

type Service struct {
	port        int
	srv         *http.Server
	store       kv.Service
	top         *network.Topology
	maxPayload  config.ByteSize
	sbacclt     sbacclient.Client
	sbac        *sbac.Service
	checkerclt  *checkerclient.Client
	shardID     uint64
	nodeID      uint64
	sbacOnly    bool
	checkerOnly bool
	checker     *checker.Service
}

type resp struct {
	Data   interface{} `json:"data"`
	Status string      `json:"status"`
}

func response(rw http.ResponseWriter, status int, resp resp) {
	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(status)
	b, _ := json.Marshal(resp)
	rw.Write(b)
}

func fail(rw http.ResponseWriter, status int, data interface{}) {
	response(rw, status, resp{data, "fail"})
}

func errorr(rw http.ResponseWriter, status int, data interface{}) {
	response(rw, status, resp{data, "error"})
}

func success(rw http.ResponseWriter, status int, data interface{}) {
	response(rw, status, resp{data, "success"})
}

func (s *Service) objectGet(rw http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
		return
	}
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.queryObject(rw, r)
}

func (s *Service) object(rw http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.createObject(rw, r)
		return
	default:
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
}

func readdata(rw http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return nil, false
	}
	req := struct {
		Data string `json:"data"`
	}{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return nil, false
	}
	if len(req.Data) <= 0 {
		fail(rw, http.StatusBadRequest, "empty data")
		return nil, false
	}
	key, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to b64decode: %v", err))
		return nil, false
	}
	return key, true
}

func BuildObjectResponse(objects []*sbac.Object) (Object, error) {
	if len(objects) <= 0 {
		return Object{}, errors.New("object already inactive")
	}
	for _, v := range objects {
		if string(v.Value) != string(objects[0].Value) {
			return Object{}, errors.New("inconsistent data")
		}
	}

	data := []Object{}
	for _, v := range objects {
		var val interface{}
		err := json.Unmarshal(v.Value, &val)
		if err != nil {
			return Object{}, fmt.Errorf("unable to unmarshal value: %v", err)
		}
		o := Object{
			VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
			Value:     val,
			Status:    v.Status.String(),
		}
		data = append(data, o)

	}
	return data[0], nil
}

func readifacedata(rw http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return nil, false
	}
	req := struct {
		Data interface{} `json:"data"`
	}{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return nil, false
	}
	if req.Data == nil {
		fail(rw, http.StatusBadRequest, "empty data")
		return nil, false
	}

	b, err := json.Marshal(req.Data)
	if err != nil {
		fail(rw, http.StatusBadRequest, "invalid data")
		return nil, false
	}

	return b, true
}

func (s *Service) createObject(rw http.ResponseWriter, r *http.Request) {
	rawObject, ok := readifacedata(rw, r)
	if !ok {
		return
	}
	ids, err := s.sbacclt.Create(rawObject)
	if err != nil {
		errorr(rw, http.StatusInternalServerError, err.Error())
		return
	}
	for _, v := range ids {
		if string(v) != string(ids[0]) {
			errorr(rw, http.StatusInternalServerError, "inconsistent data")
			return
		}
	}
	res := struct {
		ID string `json:"id"`
	}{
		ID: base64.StdEncoding.EncodeToString(ids[0]),
	}
	success(rw, http.StatusOK, res)
}

func readlistifacedata(rw http.ResponseWriter, r *http.Request) ([][]byte, bool) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return nil, false
	}
	req := struct {
		Data []interface{} `json:"data"`
	}{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return nil, false
	}
	if req.Data == nil {
		fail(rw, http.StatusBadRequest, "empty data")
		return nil, false
	}
	out := [][]byte{}
	for _, v := range req.Data {
		v := v
		b, err := json.Marshal(v)
		if err != nil {
			fail(rw, http.StatusBadRequest, "invalid data")
			return nil, false
		}
		out = append(out, b)

	}

	return out, true
}

func (s *Service) createObjects(rw http.ResponseWriter, r *http.Request) {
	rawObjects, ok := readlistifacedata(rw, r)
	if !ok {
		return
	}
	ids, err := s.sbacclt.CreateObjects(rawObjects)
	if err != nil {
		errorr(rw, http.StatusInternalServerError, err.Error())
		return
	}
	for _, v := range ids {
		if len(v) != len(ids[0]) {
			errorr(rw, http.StatusInternalServerError, "inconsistent data")
			return
		}
	}
	res := struct {
		IDs []string `json:"ids"`
	}{}
	for _, v := range ids[0] {
		v := v
		res.IDs = append(res.IDs, base64.StdEncoding.EncodeToString(v))
	}

	success(rw, http.StatusOK, res)
}

func (s *Service) swaggerJson(rw http.ResponseWriter, r *http.Request) {
	fp := path.Join("restsrv", "swagger", "swagger.json")
	http.ServeFile(rw, r, fp)
}

func (s *Service) docs(rw http.ResponseWriter, r *http.Request) {
	fp := path.Join("restsrv", "swagger", "index.html")
	http.ServeFile(rw, r, fp)
}

func (s *Service) queryObject(rw http.ResponseWriter, r *http.Request) {
	key, ok := readdata(rw, r)
	if !ok {
		return
	}
	objectraw, err := s.sbac.QueryObjectByVersionID(key)
	if err != nil {
		fail(rw, http.StatusInternalServerError, err.Error())
		return

	}

	var object interface{}
	err = json.Unmarshal(objectraw, &object)
	if err != nil {
		fail(rw, http.StatusInternalServerError, err.Error())
		return

	}

	res := struct {
		Object interface{} `json:"object"`
	}{
		Object: object,
	}

	success(rw, http.StatusOK, res)
}

func (s *Service) states(rw http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
		return
	}
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	req := struct {
		Id uint64 `json:"id"`
	}{}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return
	}
	states := s.sbac.StatesReport(r.Context())
	sort.Slice(states.States, func(i, j int) bool { return states.States[i].HashID < states.States[j].HashID })
	success(rw, http.StatusOK, states)
}

func (s *Service) shardsForTraces(
	shards map[uint64]struct{}, ts []*sbac.Trace) map[uint64]struct{} {
	for _, t := range ts {
		for _, vid := range t.InputObjectVersionIDs {
			shards[s.top.ShardForVersionID(vid)] = struct{}{}
		}
		for _, vid := range t.InputReferenceVersionIDs {
			shards[s.top.ShardForVersionID(vid)] = struct{}{}
		}
		shards = s.shardsForTraces(shards, t.Dependencies)
	}
	return shards

}

func (s *Service) shardsForTx(tx *sbac.Transaction) []uint64 {
	ids := s.shardsForTraces(map[uint64]struct{}{}, tx.Traces)
	out := []uint64{}
	for id, _ := range ids {
		out = append(out, id)
	}
	return out
}

func (s *Service) transactionUnchecked(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return
	}
	req := Transaction{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return
	}
	// require at least input object.
	for _, v := range req.Traces {
		if len(v.InputObjectVersionIDs) <= 0 {
			fail(rw, http.StatusBadRequest, "no input objects for a trace")
			return
		}
	}
	tx, err := req.ToSBAC()
	if err != nil {
		fail(rw, http.StatusBadRequest, err.Error())
		return
	}

	evidences, err := s.checkerclt.Check(tx)
	if err != nil {
		errorr(rw, http.StatusInternalServerError, err.Error())
		return
	}

	objects := []*sbac.Object{}
	shards := s.shardsForTx(tx)
	peerids := []uint64{}
	var self bool
	for _, shrd := range shards {
		if shrd == s.shardID {
			self = true
		} else {
			peerids = append(peerids, s.top.RandNodeInShard(shrd))
		}
	}
	if self {
		objects, err = s.sbac.AddTransaction(r.Context(), tx, evidences)
		if err != nil {
			errorr(rw, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(peerids) > 0 {
		objects, err = s.sbacclt.AddTransaction(peerids, tx, evidences)
		if err != nil {
			errorr(rw, http.StatusInternalServerError, err.Error())
			return
		}
	}

	data := []Object{}
	for _, v := range objects {
		v := v
		var val interface{}
		err = json.Unmarshal(v.Value, &val)
		if err != nil {
			errorr(rw, http.StatusInternalServerError, err.Error())
			return
		}
		o := Object{
			VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
			Value:     val,
			Status:    v.Status.String(),
		}
		data = append(data, o)
	}
	success(rw, http.StatusOK, data)
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

func (s *Service) transactionChecked(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return
	}
	req := Transaction{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return
	}
	// require at least input object.
	for _, v := range req.Traces {
		if len(v.InputObjectVersionIDs) <= 0 {
			fail(rw, http.StatusBadRequest, "no input objects for a trace")
			return
		}
	}
	tx, err := req.ToSBAC()
	if err != nil {
		fail(rw, http.StatusBadRequest, err.Error())
		return
	}

	objects := []*sbac.Object{}
	shards := s.shardsForTx(tx)
	peerids := []uint64{}
	signatures, err := signaturesToBytes(req.Signatures)
	if err != nil {
		fail(rw, http.StatusBadRequest, err.Error())
		return
	}
	var self bool
	for _, shrd := range shards {
		if shrd == s.shardID {
			self = true
		} else {
			peerids = append(peerids, s.top.RandNodeInShard(shrd))
		}
	}
	if self {
		objects, err = s.sbac.AddTransaction(
			r.Context(), tx, signatures)
		if err != nil {
			errorr(rw, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if len(peerids) > 0 {
		objects, err = s.sbacclt.AddTransaction(peerids, tx, signatures)
		if err != nil {
			errorr(rw, http.StatusInternalServerError, err.Error())
			return
		}
	}

	data := []Object{}
	for _, v := range objects {
		v := v
		var val interface{}
		err = json.Unmarshal(v.Value, &val)
		if err != nil {
			errorr(rw, http.StatusInternalServerError, err.Error())
			return
		}
		o := Object{
			VersionID: base64.StdEncoding.EncodeToString(v.VersionID),
			Value:     val,
			Status:    v.Status.String(),
		}
		data = append(data, o)
	}
	success(rw, http.StatusOK, data)
}

func (s *Service) objectsReady(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(rw, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		fail(rw, http.StatusBadRequest, "unsupported content-type")
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to read request: %v", err))
		return
	}
	req := struct {
		Data []string `json:"data"`
	}{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return
	}

	for _, v := range req.Data {
		key, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to b64decode: %v", err))
			return
		}
		objs, err := s.sbacclt.Query(key)
		if err != nil {
			success(rw, http.StatusOK, false)
			return
		}
		if uint64(len(objs)) != s.top.ShardSize() {
			success(rw, http.StatusOK, false)
			return
		}
		for _, v := range objs {
			if !bytes.Equal(v.VersionID, objs[0].VersionID) {
				fail(rw, http.StatusInternalServerError, "inconsistent data")
				return
			}
		}
	}
	success(rw, http.StatusOK, true)
}

func (s *Service) makeServ(addr string, port int) *http.Server {
	staticServer := http.FileServer(assetFS())
	mux := http.NewServeMux()
	// documentation
	mux.HandleFunc("/swagger.json", s.swaggerJson)
	mux.Handle("/docs/", http.StripPrefix("/docs", staticServer))
	// transactions
	if !s.checkerOnly {
		mux.HandleFunc("/object", s.object)
		mux.HandleFunc("/objects", s.createObjects)
		mux.HandleFunc("/object/get", s.objectGet)
		mux.HandleFunc("/object/ready", s.objectsReady)
		mux.HandleFunc("/states", s.states)
		mux.HandleFunc("/transaction", s.transactionChecked)
		mux.HandleFunc("/transaction/unchecked", s.transactionUnchecked)
		// kv store
		mux.HandleFunc("/kv/get", s.kvGet)
		mux.HandleFunc("/kv/get-objectid", s.kvGetObjectID)
	}
	// checkers
	if !s.sbacOnly {
		mux.HandleFunc("/transaction/check", s.checkTransaction)
	}

	handler := cors.Default().Handler(mux)
	h := &http.Server{
		Addr:    fmt.Sprintf("%v:%v", addr, port),
		Handler: handler,
	}
	h.SetKeepAlivesEnabled(false)
	return h
}

func New(cfg *Config) *Service {
	var checkrclt *checkerclient.Client
	var txclient sbacclient.Client
	var shardID uint64
	if !cfg.CheckerOnly {
		checkrcfg := checkerclient.Config{
			NodeID:     cfg.SelfID,
			Top:        cfg.Top,
			MaxPayload: cfg.MaxPayload,
			Key:        cfg.Key,
		}
		checkrclt = checkerclient.New(&checkrcfg)
		clcfg := sbacclient.Config{
			NodeID:     cfg.SelfID,
			Top:        cfg.Top,
			MaxPayload: cfg.MaxPayload,
			Key:        cfg.Key,
		}
		txclient = sbacclient.New(&clcfg)
		shardID = cfg.Top.ShardForNode(cfg.SelfID)
	}
	s := &Service{
		port:        cfg.Port,
		top:         cfg.Top,
		maxPayload:  cfg.MaxPayload,
		sbacclt:     txclient,
		store:       cfg.Store,
		sbac:        cfg.SBAC,
		checkerclt:  checkrclt,
		nodeID:      cfg.SelfID,
		shardID:     shardID,
		sbacOnly:    cfg.SBACOnly,
		checkerOnly: cfg.CheckerOnly,
		checker:     cfg.Checker,
	}
	s.srv = s.makeServ(cfg.Addr, cfg.Port)
	go func() {
		log.Info("http server started", fld.Port(cfg.Port))
		log.Fatal("http server exited", fld.Err(s.srv.ListenAndServe()))
	}()
	return s
}

package restsrv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"chainspace.io/prototype/internal/log"
	"chainspace.io/prototype/sbac"
)

type CheckTransactionResponse struct {
	NodeID    uint64 `json:"node_id"`
	OK        bool   `json:"ok"`
	Signature string `json:"signature"`
}

func (s *Service) checkTransaction(rw http.ResponseWriter, r *http.Request) {
	now := time.Now()
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
		log.Error("ERROR IN REST SRV", log.Err(err))
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

	ok, signature, err := s.checker.CheckAndSign(r.Context(), tx)
	if err != nil {
		errorr(rw, http.StatusInternalServerError, err.Error())
		return
	}
	log.Error("to sbac ok")
	resp := CheckTransactionResponse{
		NodeID:    s.nodeID,
		OK:        ok,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}
	success(rw, http.StatusOK, resp)
	log.Error("new transaction checked", log.String("time.taken", time.Since(now).String()))
}

type transactionRaw struct {
	ContractID      string           `json:"contractID"`
	MethodID        string           `json:"methodID"`
	Dependencies    []transactionRaw `json:"dependencies"`
	Inputs          []interface{}    `json:"inputs"`
	Outputs         []interface{}    `json:"outputs"`
	ReferenceInputs []interface{}    `json:"referenceInputs"`
	Parameters      []interface{}    `json:"parameters"`
	Returns         []interface{}    `json:"returns"`
}

func (tr *transactionRaw) ToSBAC() *sbac.Trace {
	toJsonList := func(s []interface{}) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			bytes, _ := json.Marshal(v)
			out = append(out, bytes)
		}
		return out
	}
	deps := []*sbac.Trace{}
	for _, v := range tr.Dependencies {
		deps = append(deps, v.ToSBAC())
	}
	trace := sbac.Trace{
		ContractID:      tr.ContractID,
		Procedure:       tr.MethodID,
		Dependencies:    deps,
		InputObjects:    toJsonList(tr.Inputs),
		OutputObjects:   toJsonList(tr.Outputs),
		InputReferences: toJsonList(tr.ReferenceInputs),
		Parameters:      toJsonList(tr.Parameters),
		Returns:         toJsonList(tr.Returns),
	}
	return &trace
}

func (s *Service) checkTransactionRaw(rw http.ResponseWriter, r *http.Request) {
	now := time.Now()
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
	req := transactionRaw{}
	if err := json.Unmarshal(body, &req); err != nil {
		fail(rw, http.StatusBadRequest, fmt.Sprintf("unable to unmarshal: %v", err))
		return
	}
	trace := req.ToSBAC()
	if err != nil {
		fail(rw, http.StatusBadRequest, err.Error())
		return
	}
	tx := &sbac.Transaction{
		Traces: []*sbac.Trace{trace},
	}

	ok, signature, err := s.checker.CheckAndSign(r.Context(), tx)
	if err != nil {
		errorr(rw, http.StatusInternalServerError, err.Error())
		return
	}
	resp := CheckTransactionResponse{
		NodeID:    s.nodeID,
		OK:        ok,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}
	success(rw, http.StatusOK, resp)
	log.Error("new transaction checked", log.String("time.taken", time.Since(now).String()))
}

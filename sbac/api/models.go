package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"chainspace.io/prototype/sbac"
)

// ObjectRequest contains a full json graph
type ObjectRequest struct {
	Object interface{} `json:"object"`
}

// ObjectResponse contains a full json graph
type ObjectResponse struct {
	Object interface{} `json:"object"`
}

// ObjectIDResponse contains a full json graph
type ObjectIDResponse struct {
	ID string `json:"id"`
}

// Error ...
type Error struct {
	Error string `json:"error"`
}

type Object struct {
	VersionID string      `json:"version_id"`
	Value     interface{} `json:"value"`
	Status    string      `json:"status"`
}

type StateReport struct {
	HashID          uint32          `json:"hash_id"`
	CommitDecisions map[uint64]bool `json:"commit_decisions"`
	Phase1Decisions map[uint64]bool `json:"phase1_decisions"`
	Phase2Decisions map[uint64]bool `json:"phase2_decisions"`
	State           string          `json:"state"`
	PendingEvents   int32           `json:"pending_events"`
}

type StateReportResponse struct {
	States        []StateReport `json:"states"`
	PendingEvents int32         `json:"pending_events"`
}

type Transaction struct {
	Traces     []Trace                `json:"traces"`
	Mappings   map[string]interface{} `json:"mappings"`
	Signatures map[uint64]string      `json:"signatures"` //base64 encoded
}

func (ct *Transaction) ToSBAC() (*sbac.Transaction, error) {
	traces := make([]*sbac.Trace, 0, len(ct.Traces))
	for _, t := range ct.Traces {
		ttrace, err := t.ToSBAC(ct.Mappings)
		if err != nil {
			return nil, err
		}
		traces = append(traces, ttrace)
	}
	return &sbac.Transaction{
		Traces: traces,
	}, nil
}

type Dependency Trace

type Trace struct {
	ContractID               string        `json:"contract_id"`
	Procedure                string        `json:"procedure"`
	InputObjectVersionIDs    []string      `json:"input_object_version_ids"`
	InputReferenceVersionIDs []string      `json:"input_reference_version_ids"`
	OutputObjects            []interface{} `json:"output_objects"`
	Parameters               []interface{} `json:"parameters"`
	Returns                  []interface{} `json:"returns"`
	Labels                   [][]string    `json:"labels"`
	Dependencies             []Dependency  `json:"dependencies"`
}

func (ct *Trace) ToSBAC(mappings map[string]interface{}) (*sbac.Trace, error) {
	fromB64String := func(s []string) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			bytes, _ := base64.StdEncoding.DecodeString(v)
			out = append(out, []byte(bytes))
		}
		return out
	}
	toJsonList := func(s []interface{}) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			bytes, _ := json.Marshal(v)
			out = append(out, bytes)
		}
		return out
	}
	deps := make([]*sbac.Trace, 0, len(ct.Dependencies))
	for _, d := range ct.Dependencies {
		t := Trace(d)
		ttrace, err := t.ToSBAC(mappings)
		if err != nil {
			return nil, err
		}
		deps = append(deps, ttrace)
	}

	inputObjects := make([][]byte, 0, len(ct.InputObjectVersionIDs))
	for _, v := range ct.InputObjectVersionIDs {
		object, ok := mappings[v]
		if !ok {
			return nil, fmt.Errorf("missing object mapping for key [%v]", v)
		}
		bobject, _ := json.Marshal(object)
		inputObjects = append(inputObjects, bobject)

	}
	inputReferences := make([][]byte, 0, len(ct.InputReferenceVersionIDs))
	for _, v := range ct.InputReferenceVersionIDs {
		object, ok := mappings[v]
		if !ok {
			return nil, fmt.Errorf("missing object mapping for key [%v]", v)
		}
		bobject, _ := json.Marshal(object)
		inputReferences = append(inputReferences, bobject)

	}
	return &sbac.Trace{
		ContractID:               ct.ContractID,
		Procedure:                ct.Procedure,
		InputObjectVersionIDs:    fromB64String(ct.InputObjectVersionIDs),
		InputReferenceVersionIDs: fromB64String(ct.InputReferenceVersionIDs),
		InputObjects:             inputObjects,
		InputReferences:          inputReferences,
		OutputObjects:            toJsonList(ct.OutputObjects),
		Parameters:               toJsonList(ct.Parameters),
		Returns:                  toJsonList(ct.Returns),
		Labels:                   sbac.StringsSlice{}.FromSlice(ct.Labels),
		Dependencies:             deps,
	}, nil
}

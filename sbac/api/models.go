package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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

// Object ...
type Object struct {
	Status    string      `json:"status"`
	Value     interface{} `json:"value"`
	VersionID string      `json:"versionId"`
}

// StateReport ...
type StateReport struct {
	CommitDecisions map[uint64]bool `json:"commitDecisions"`
	HashID          uint32          `json:"hashId"`
	PendingEvents   int32           `json:"pendingEvents"`
	Phase1Decisions map[uint64]bool `json:"phase1Decisions"`
	Phase2Decisions map[uint64]bool `json:"phase2Decisions"`
	State           string          `json:"state"`
}

// StateReportResponse ...
type StateReportResponse struct {
	PendingEvents int32         `json:"pendingEvents"`
	States        []StateReport `json:"states"`
}

// Transaction ...
type Transaction struct {
	Mappings   map[string]interface{} `json:"mappings"`
	Signatures map[uint64]string      `json:"signatures"` //base64 encoded
	Traces     []Trace                `json:"traces"`
}

// ToSBAC ...
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

// Dependency ...
type Dependency Trace

// Trace ...
type Trace struct {
	ContractID               string        `json:"contractId"`
	Dependencies             []Dependency  `json:"dependencies"`
	InputObjectVersionIDs    []string      `json:"inputObjectVersionIds"`
	InputReferenceVersionIDs []string      `json:"inputReferenceVersionIds"`
	Labels                   [][]string    `json:"labels"`
	OutputObjects            []interface{} `json:"outputObjects"`
	Parameters               []interface{} `json:"parameters"`
	Procedure                string        `json:"procedure"`
	Returns                  []interface{} `json:"returns"`
}

// ToSBAC ...
func (ct *Trace) ToSBAC(mappings map[string]interface{}) (*sbac.Trace, error) {
	fromB64String := func(s []string) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			bytes, _ := base64.StdEncoding.DecodeString(v)
			out = append(out, []byte(bytes))
		}
		return out
	}
	toJSONList := func(s []interface{}) [][]byte {
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
			return nil, fmt.Errorf("missing reference mapping for key [%v]", v)
		}
		bobject, _ := json.Marshal(object)
		inputReferences = append(inputReferences, bobject)

	}
	return &sbac.Trace{
		ContractID:               ct.ContractID,
		Dependencies:             deps,
		InputObjects:             inputObjects,
		InputObjectVersionIDs:    fromB64String(ct.InputObjectVersionIDs),
		InputReferences:          inputReferences,
		InputReferenceVersionIDs: fromB64String(ct.InputReferenceVersionIDs),
		Labels:                   sbac.StringsSlice{}.FromSlice(ct.Labels),
		OutputObjects:            toJSONList(ct.OutputObjects),
		Parameters:               toJSONList(ct.Parameters),
		Procedure:                ct.Procedure,
		Returns:                  toJSONList(ct.Returns),
	}, nil
}

// BuildObjectResponse ...
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

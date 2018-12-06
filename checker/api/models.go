package api

import (
	"encoding/base64"
	"encoding/json"

	"chainspace.io/prototype/sbac"
)

// CheckTransactionResponse ...
type CheckTransactionResponse struct {
	NodeID    uint64 `json:"nodeId"`
	OK        bool   `json:"ok"`
	Signature string `json:"signature"`
}

// Error ...
type Error struct {
	Error string `json:"error"`
}

// Transaction ...
type Transaction struct {
	Mappings   map[string]interface{} `json:"mappings"`
	Signatures map[uint64]string      `json:"signatures"` //base64 encoded
	Traces     []Trace                `json:"traces"`
}

// ToSBAC ...
func (tx *Transaction) ToSBAC(validator TransactionValidator) (*sbac.Transaction, error) {
	err := validator.Validate(tx)
	if err != nil {
		return nil, err
	}

	traces := make([]*sbac.Trace, 0, len(tx.Traces))
	for _, tc := range tx.Traces {
		sbacTrace := tc.ToSBAC(tx.Mappings)
		traces = append(traces, sbacTrace)
	}

	return &sbac.Transaction{
		Traces: traces,
	}, nil
}

// Dependency ...
type Dependency Trace

type OutputObject struct {
	Labels []string `json:"labels"`
	Object string   `json:"object"`
}

// Trace ...
type Trace struct {
	ContractID               string         `json:"contractId"`
	Dependencies             []Dependency   `json:"dependencies"`
	InputObjectVersionIDs    []string       `json:"inputObjectVersionIds"`
	InputReferenceVersionIDs []string       `json:"inputReferenceVersionIds"`
	OutputObjects            []OutputObject `json:"outputObjects"`
	Parameters               []string       `json:"parameters"`
	Procedure                string         `json:"procedure"`
	Returns                  []string       `json:"returns"`
}

func b64DecodeStrings(s []string) [][]byte {
	out := make([][]byte, 0, len(s))
	for _, v := range s {
		bytes, _ := base64.StdEncoding.DecodeString(v)
		out = append(out, []byte(bytes))
	}
	return out
}

func toByteSlice(s []string) [][]byte {
	out := make([][]byte, 0, len(s))
	for _, v := range s {
		out = append(out, []byte(v))
	}
	return out
}

// ToSBAC ...
func (tc *Trace) ToSBAC(mappings map[string]interface{}) *sbac.Trace {
	deps := make([]*sbac.Trace, 0, len(tc.Dependencies))
	for _, d := range tc.Dependencies {
		t := Trace(d)
		ttrace := t.ToSBAC(mappings)
		deps = append(deps, ttrace)
	}

	inputObjects := make([][]byte, 0, len(tc.InputObjectVersionIDs))
	for _, v := range tc.InputObjectVersionIDs {
		object := mappings[v]
		bobject, _ := json.Marshal(object)
		inputObjects = append(inputObjects, bobject)
	}

	inputReferences := make([][]byte, 0, len(tc.InputReferenceVersionIDs))
	for _, v := range tc.InputReferenceVersionIDs {
		object := mappings[v]
		bobject, _ := json.Marshal(object)
		inputReferences = append(inputReferences, bobject)
	}

	outputObjects := make([]*sbac.OutputObject, 0, len(tc.OutputObjects))
	for _, v := range tc.OutputObjects {
		obj := &sbac.OutputObject{Labels: v.Labels, Object: []byte(v.Object)}
		outputObjects = append(outputObjects, obj)

	}

	return &sbac.Trace{
		ContractID:               tc.ContractID,
		Dependencies:             deps,
		InputObjects:             inputObjects,
		InputObjectVersionIDs:    b64DecodeStrings(tc.InputObjectVersionIDs),
		InputReferences:          inputReferences,
		InputReferenceVersionIDs: b64DecodeStrings(tc.InputReferenceVersionIDs),
		OutputObjects:            outputObjects,
		Parameters:               toByteSlice(tc.Parameters),
		Procedure:                tc.Procedure,
		Returns:                  toByteSlice(tc.Returns),
	}
}

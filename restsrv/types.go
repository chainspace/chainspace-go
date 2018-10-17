package restsrv // import "chainspace.io/prototype/restsrv"

import (
	"encoding/base64"
	"encoding/json"

	"chainspace.io/prototype/transactor"
)

type Object struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Status string      `json:"status"`
}

type Transaction struct {
	Traces []Trace `json:"traces"`
}

func (ct *Transaction) ToTransactor() *transactor.Transaction {
	traces := make([]*transactor.Trace, 0, len(ct.Traces))
	for _, t := range ct.Traces {
		traces = append(traces, t.ToTransactor())
	}
	return &transactor.Transaction{
		Traces: traces,
	}
}

type Trace struct {
	ContractID          string        `json:"contract_id"`
	Procedure           string        `json:"procedure"`
	InputObjectsKeys    []string      `json:"input_objects_keys"`
	InputReferencesKeys []string      `json:"input_references_keys"`
	OutputObjects       []interface{} `json:"output_objects"`
	Parameters          []interface{} `json:"parameters"`
	Returns             []interface{} `json:"returns"`
	Labels              [][]string    `json:"labels"`
	Dependencies        []Trace       `json:"dependencies"`
}

func (ct *Trace) ToTransactor() *transactor.Trace {
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
	deps := make([]*transactor.Trace, 0, len(ct.Dependencies))
	for _, d := range ct.Dependencies {
		deps = append(deps, d.ToTransactor())
	}
	return &transactor.Trace{
		ContractID:          ct.ContractID,
		Procedure:           ct.Procedure,
		InputObjectsKeys:    fromB64String(ct.InputObjectsKeys),
		InputReferencesKeys: fromB64String(ct.InputReferencesKeys),
		OutputObjects:       toJsonList(ct.OutputObjects),
		Parameters:          toJsonList(ct.Parameters),
		Returns:             toJsonList(ct.Returns),
		Labels:              transactor.StringsSlice{}.FromSlice(ct.Labels),
		Dependencies:        deps,
	}
}

package restsrv // import "chainspace.io/prototype/restsrv"

import (
	"encoding/base64"

	"chainspace.io/prototype/transactor"
)

type Object struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Status string `json:"status"`
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
	ContractID          string     `json:"contract_id"`
	Procedure           string     `json:"procedure"`
	InputObjectsKeys    []string   `json:"input_objects_keys"`
	InputReferencesKeys []string   `json:"input_references_keys"`
	OutputObjects       []string   `json:"output_objects"`
	Parameters          []string   `json:"parameters"`
	Returns             []string   `json:"returns"`
	Labels              [][]string `json:"labels"`
	Dependencies        []Trace    `json:"dependencies"`
}

func (ct *Trace) ToTransactor() *transactor.Trace {
	toBytesB64 := func(s []string) [][]byte {
		out := make([][]byte, 0, len(s))
		for _, v := range s {
			bytes, _ := base64.StdEncoding.DecodeString(v)
			out = append(out, []byte(bytes))
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
		InputObjectsKeys:    toBytesB64(ct.InputObjectsKeys),
		InputReferencesKeys: toBytesB64(ct.InputReferencesKeys),
		OutputObjects:       toBytesB64(ct.OutputObjects),
		Parameters:          toBytesB64(ct.Parameters),
		Returns:             toBytesB64(ct.Returns),
		Labels:              transactor.StringsSlice{}.FromSlice(ct.Labels),
		Dependencies:        deps,
	}
}

package restsrv // import "chainspace.io/prototype/restsrv"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"chainspace.io/prototype/sbac"
)

type Object struct {
	Key    string      `json:"key"`
	Value  interface{} `json:"value"`
	Status string      `json:"status"`
}

type Transaction struct {
	Traces   []Trace                `json:"traces"`
	Mappings map[string]interface{} `json:"mappings"`
}

func (ct *Transaction) ToTransactor() (*sbac.Transaction, error) {
	traces := make([]*sbac.Trace, 0, len(ct.Traces))
	for _, t := range ct.Traces {
		ttrace, err := t.ToTransactor(ct.Mappings)
		if err != nil {
			return nil, err
		}
		traces = append(traces, ttrace)
	}
	return &sbac.Transaction{
		Traces: traces,
	}, nil
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

func (ct *Trace) ToTransactor(mappings map[string]interface{}) (*sbac.Trace, error) {
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
		ttrace, err := d.ToTransactor(mappings)
		if err != nil {
			return nil, err
		}
		deps = append(deps, ttrace)
	}

	inputObjects := make([][]byte, 0, len(ct.InputObjectsKeys))
	for _, v := range ct.InputObjectsKeys {
		object, ok := mappings[v]
		if !ok {
			return nil, fmt.Errorf("missing object mapping for key [%v]", v)
		}
		bobject, _ := json.Marshal(object)
		inputObjects = append(inputObjects, bobject)

	}
	inputReferences := make([][]byte, 0, len(ct.InputReferencesKeys))
	for _, v := range ct.InputReferencesKeys {
		object, ok := mappings[v]
		if !ok {
			return nil, fmt.Errorf("missing object mapping for key [%v]", v)
		}
		bobject, _ := json.Marshal(object)
		inputReferences = append(inputReferences, bobject)

	}
	return &sbac.Trace{
		ContractID:          ct.ContractID,
		Procedure:           ct.Procedure,
		InputObjectsKeys:    fromB64String(ct.InputObjectsKeys),
		InputReferencesKeys: fromB64String(ct.InputReferencesKeys),
		InputObjects:        inputObjects,
		InputReferences:     inputReferences,
		OutputObjects:       toJsonList(ct.OutputObjects),
		Parameters:          toJsonList(ct.Parameters),
		Returns:             toJsonList(ct.Returns),
		Labels:              sbac.StringsSlice{}.FromSlice(ct.Labels),
		Dependencies:        deps,
	}, nil
}

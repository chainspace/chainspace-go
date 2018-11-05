package sbac // import "chainspace.io/prototype/sbac"

import (
	"crypto/sha512"
	"encoding/binary"
	"fmt"
)

type IDs struct {
	TxID             []byte
	TraceObjectPairs []TraceObjectPair
}

// TraceIdentifierPair is a pair of a trace and it's identifier
type TraceIdentifierPair struct {
	ID    []byte
	Trace *Trace
}

// TraceOutputObjectIDPair is composed of a trace and the list of output object IDs it create
// ordered in the same order than the orginal output objects
type TraceObjectPair struct {
	OutputObjects []*Object
	Trace         TraceIdentifierPair
}

// MakeTraceIDs generate trace IDs for all traces in the given list
func MakeTraceIDs(traces []*Trace) ([]TraceIdentifierPair, error) {
	out := make([]TraceIdentifierPair, 0, len(traces))
	for _, trace := range traces {
		trace := trace
		id, err := MakeTraceID(trace)
		if err != nil {
			return nil, err
		}
		p := TraceIdentifierPair{
			ID:    id,
			Trace: trace,
		}
		out = append(out, p)
	}

	return out, nil
}

// MakeTraceID generate an identifier for the given trace
// the ID is composed of: the contract ID, the procedure, input objects keys, input
// reference keys, trace ID of the dependencies
func MakeTraceID(trace *Trace) ([]byte, error) {
	hasher := sha512.New512_256()
	data := []byte{}
	data = append(data, []byte(trace.ContractID)...)
	data = append(data, []byte(trace.Procedure)...)
	for _, v := range trace.InputObjectVersionIDs {
		data = append(data, v...)
	}
	for _, v := range trace.InputReferenceVersionIDs {
		data = append(data, v...)
	}
	for _, v := range trace.Dependencies {
		v := v
		id, err := MakeTraceID(v)
		if err != nil {
			return nil, err
		}
		data = append(data, id...)
	}
	_, err := hasher.Write(data)
	if err != nil {
		return nil, fmt.Errorf("sbac: unable to create hash: %v", err)
	}

	return hasher.Sum(nil), nil
}

// MakeTraceObjectIDs create a list of Objects based on the Trace / Trace ID input
// Objects are ordered the same as the output objects of the trace
func MakeObjectIDs(pair *TraceIdentifierPair) ([]*Object, error) {
	hasher := sha512.New512_256()
	out := []*Object{}
	for i, outobj := range pair.Trace.OutputObjects {
		hasher.Reset()
		id := make([]byte, len(pair.ID))
		copy(id, pair.ID)
		id = append(id, outobj...)
		index := make([]byte, 4)
		binary.LittleEndian.PutUint32(index, uint32(i))
		id = append(id, index...)
		_, err := hasher.Write(id)
		if err != nil {
			return nil, fmt.Errorf("sbac: unable to create hash: %v", err)
		}
		var labels []string
		if len(pair.Trace.Labels) > i {
			labels = pair.Trace.Labels[i].AsSlice()
		}
		o := &Object{
			Value:     outobj,
			VersionID: hasher.Sum(nil),
			Labels:    labels,
			Status:    ObjectStatus_ACTIVE,
		}

		out = append(out, o)
	}
	return out, nil
}

// MakeObjectID create a list of Object based on the traces / traces identifier
func MakeTraceObjectPairs(traces []TraceIdentifierPair) ([]TraceObjectPair, error) {
	out := []TraceObjectPair{}
	for _, trace := range traces {
		objs, err := MakeObjectIDs(&trace)
		if err != nil {
			return nil, err
		}
		pair := TraceObjectPair{
			OutputObjects: objs,
			Trace:         trace,
		}
		out = append(out, pair)
	}
	return out, nil
}

func MakeTransactionID(top []TraceObjectPair) ([]byte, error) {
	hasher := sha512.New512_256()
	bytes := []byte{}
	for _, v := range top {
		bytes = append(bytes, v.Trace.ID...)
		for _, o := range v.OutputObjects {
			bytes = append(bytes, o.VersionID...)
		}
	}
	_, err := hasher.Write(bytes)
	if err != nil {
		return nil, fmt.Errorf("sbac: unable to create hash: %v", err)
	}
	return hasher.Sum(nil), nil
}

func MakeIDs(tx *Transaction) (*IDs, error) {
	tracesIDPairs, err := MakeTraceIDs(tx.Traces)
	if err != nil {
		return nil, err
	}
	traceObjectsPairs, err := MakeTraceObjectPairs(tracesIDPairs)
	if err != nil {
		return nil, err
	}
	txID, err := MakeTransactionID(traceObjectsPairs)
	if err != nil {
		return nil, err
	}
	return &IDs{txID, traceObjectsPairs}, nil
}

package checker // import "chainspace.io/prototype/checker"

import (
	"encoding/base64"
	"fmt"

	"chainspace.io/prototype/sbac"
)

type idstate int8

const (
	// id seen as a ref
	active idstate = iota
	// id seen as input
	inactive
)

type statedata struct {
	state    idstate
	contract string
}

type idmap map[string]statedata

func TypeCheck(tx *sbac.Transaction) error {
	return typeCheck(idmap{}, tx.Traces)
}

func typeCheck(ids idmap, traces []*sbac.Trace) error {
	// type checks all traces
	for _, trace := range traces {
		trace := trace
		// first typecheck dependencies
		if len(trace.Dependencies) > 0 {
			if err := typeCheck(ids, trace.Dependencies); err != nil {
				return err
			}
		}

		if len(trace.ContractID) <= 0 {
			return fmt.Errorf("missing contractid")
		}

		if len(trace.Procedure) <= 0 {
			return fmt.Errorf("missing procedure")
		}

		if len(trace.OutputObjects) != len(trace.InputObjects) {
			return fmt.Errorf("%v.%v() expect %v outputs, have %v",
				trace.ContractID, trace.Procedure,
				len(trace.InputObjects), len(trace.OutputObjects))
		}

		for _, v := range trace.InputObjectVersionIDs {
			v := v
			if sd, ok := ids[string(v)]; ok {
				if sd.state == inactive {
					return fmt.Errorf("%v.%v using inactive versionid(%v) as input", trace.ContractID, trace.Procedure, b64(v))
				}
				if sd.state == active {
					return fmt.Errorf("%v.%v using versionid(%v) previously reference by another contract(%v)", trace.ContractID, trace.Procedure, b64(v), sd.contract)
				}
				sd.state = inactive
				ids[string(v)] = sd
			} else {
				ids[string(v)] = statedata{inactive, trace.ContractID}
			}
		}

		for _, v := range trace.InputReferenceVersionIDs {
			v := v
			if sd, ok := ids[string(v)]; ok {
				if sd.state == inactive {
					return fmt.Errorf("%v.%v using inactive versionid(%v) as reference", trace.ContractID, trace.Procedure, b64(v))
				}
			} else {
				ids[string(v)] = statedata{active, trace.ContractID}
			}
		}
	}
	return nil
}

func b64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

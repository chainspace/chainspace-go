package api_test

import (
	sbacapi "chainspace.io/chainspace-go/sbac/api"
)

var transactionFixture = sbacapi.Transaction{
	Mappings: map[string]string{
		"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE=": "foobar",
	},
	Signatures: nil,
	Traces: []sbacapi.Trace{{
		ContractID: "dummy",
		Dependencies: []sbacapi.Dependency{{
			ContractID:               "dummy",
			Dependencies:             nil,
			InputObjectVersionIDs:    []string{"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE="},
			InputReferenceVersionIDs: []string{"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE="},
			OutputObjects: []sbacapi.OutputObject{
				{
					Labels: []string{"foo"},
					Object: "thisissomethingmagical!",
				},
			},
			Parameters: nil,
			Procedure:  "dummy_ok",
			Returns:    nil,
		}},
		InputObjectVersionIDs:    []string{"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE="},
		InputReferenceVersionIDs: []string{"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE="},
		OutputObjects: []sbacapi.OutputObject{
			{
				Labels: []string{"foo"},
				Object: "thisissomethingmagical!",
			},
		},
		Parameters: nil,
		Procedure:  "dummy_ok",
		Returns:    nil,
	}},
}

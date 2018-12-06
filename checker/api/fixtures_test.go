package api_test

import "chainspace.io/prototype/checker/api"

var transactionFixture = api.Transaction{
	Mappings: map[string]interface{}{
		"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE=": "foobar",
	},
	Signatures: nil,
	Traces: []api.Trace{{
		ContractID: "dummy",
		Dependencies: []api.Dependency{{
			ContractID:               "dummy",
			Dependencies:             nil,
			InputObjectVersionIDs:    []string{"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE="},
			InputReferenceVersionIDs: []string{"4S8G4OhhLcXFgzXvnFoGPdEC/QtjbX6zQHJmeevOPhE="},
			OutputObjects: []api.OutputObject{
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
		OutputObjects: []api.OutputObject{
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

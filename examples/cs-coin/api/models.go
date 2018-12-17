package api

import sbacapi "chainspace.io/prototype/sbac/api"

// InitRequest is the payload expected by the Init endpoint
type InitRequest struct {
	Address string `json:"address"`
}

type InitResponse struct {
	Error      string `json:"error,omitempty"`
	InitObject string `json:"initObject,omitempty"`
}

// CreateWalletRequest is the payload expected by the CreateWallet endpoint
type CreateWalletRequest struct {
	Address string `json:"address"`
	PubKey  string `json:"pubKey"`

	InitObject string `json:"initObject"`

	Mappings map[string]string `json:"mappings"`
}

// AddFundsRequest is the payload expected by the AddFunds endpoint
// Signature over the 2 first field of the request as json:
// {"wallet": "...", "value": 42} -> sign this string
type AddFundsRequest struct {
	Wallet string `json:"wallet"`
	Amount uint   `json:"amount"`

	Signature string `json:"signature"`

	Mappings map[string]string `json:"mappings"`
}

// AddFundsRequest is the payload expected by the TransferFunds endpoint
// Signature over the 3 first field of the request as json:
// {"fromWallet": "...", "toWallet": "..." "value": 42} -> sign this string
type TransferFundsRequest struct {
	FromWallet string `json:"fromWallet"`
	ToWallet   string `json:"toWallet"`
	Amount     uint   `json:"amount"`

	Signature string `json:"signature"`

	Mappings map[string]string `json:"mappings"`
}

// Response returned by all the methods
type Response struct {
	Error       string               `json:"error,omitempty"`
	Transaction *sbacapi.Transaction `json:"transaction,omitempty"`
}

// CheckerResponse ...
type CheckerResponse struct {
	Success bool `json:"success"`
}

package api

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

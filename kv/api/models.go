package api

// VersionIDResponse contains only a version id
type VersionIDResponse struct {
	VersionID string `json:"version_id"`
}

// ObjectResponse contains a full json graph
type ObjectResponse struct {
	Object interface{} `json:"object"`
}

// Error ...
type Error struct {
	Error string `json:"error"`
}

package api

// Error ...
type Error struct {
	Error string `json:"error"`
}

// LabelObject ...
type LabelObject struct {
	Label  string      `json:"label"`
	Object interface{} `json:"object"`
}

// LabelVersionID ...
type LabelVersionID struct {
	Label     string `json:"label"`
	VersionID string `json:"version_id"`
}

// ListObjectsResponse contains a full json graph
type ListObjectsResponse struct {
	Pairs []LabelObject `json:"pairs"`
}

// ListVersionIDsResponse contains a collection of LabelVersionIDs
type ListVersionIDsResponse struct {
	Pairs []LabelVersionID `json:"pairs"`
}

// ObjectResponse contains a full json graph
type ObjectResponse struct {
	Object interface{} `json:"object"`
}

// VersionIDResponse contains only a version id
type VersionIDResponse struct {
	VersionID string `json:"version_id"`
}

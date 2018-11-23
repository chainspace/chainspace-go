package api

// VersionIDResponse contains only a version id
type VersionIDResponse struct {
	VersionID string `json:"version_id"`
}

// LabelVersionID ...
type LabelVersionID struct {
	Label     string `json:"label"`
	VersionID string `json:"version_id"`
}

// VersionIDResponse contains only a version id
type VersionIDListResponse struct {
	Pairs []LabelVersionID `json:"pairs"`
}

// ObjectResponse contains a full json graph
type ObjectResponse struct {
	Object interface{} `json:"object"`
}

// LabelObject ...
type LabelObject struct {
	Label  string      `json:"label"`
	Object interface{} `json:"object"`
}

// ObjectResponse contains a full json graph
type ObjectListResponse struct {
	Pairs []LabelObject `json:"pairs"`
}

// Error ...
type Error struct {
	Error string `json:"error"`
}

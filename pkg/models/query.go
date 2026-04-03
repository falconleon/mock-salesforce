package models

// QueryResponse represents a SOQL query result.
type QueryResponse struct {
	TotalSize      int              `json:"totalSize"`
	Done           bool             `json:"done"`
	Records        []map[string]any `json:"records"`
	NextRecordsURL string           `json:"nextRecordsUrl,omitempty"`
}

// RecordAttributes contains metadata about a record.
type RecordAttributes struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

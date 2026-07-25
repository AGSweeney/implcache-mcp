package tools

// Options configures MCP tool safety and resource limits.
type Options struct {
	ReadOnly         bool
	AllowIngest      bool
	AllowDelete      bool
	AllowOutputWrite bool
	OutputRoot       string // absolute path; vomit writes only under here
	MaxResults       int
	MaxIngestFiles   int
	MaxDocumentBytes int64
}

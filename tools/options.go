// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

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

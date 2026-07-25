// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package librarian provides a stable inventory/health/preview surface for the
// ImplCache Librarian GUI and admin MCP tools.
package librarian

import "implcache-mcp/store"

// SourceKind identifies a first-class or synthesized source.
type SourceKind string

const (
	KindWeb   SourceKind = "web"
	KindPDF   SourceKind = "pdf"
	KindRepo  SourceKind = "repo"
	KindLocal SourceKind = "local"
)

// SourceRef identifies a source across kinds.
type SourceRef struct {
	Kind     SourceKind `json:"kind"`
	ID       string     `json:"id"`
	RootName string     `json:"rootName"`
	Title    string     `json:"title,omitempty"`
}

// SourceSummary is one row in the unified inventory.
type SourceSummary struct {
	SourceRef
	Enabled       bool           `json:"enabled,omitempty"`
	LastStatus    string         `json:"lastStatus,omitempty"`
	LastAttemptAt int64          `json:"lastAttemptAt,omitempty"`
	LastSuccessAt int64          `json:"lastSuccessAt,omitempty"`
	DocumentCount int64          `json:"documentCount"`
	ChunkCount    int64          `json:"chunkCount"`
	SymbolCount   int64          `json:"symbolCount,omitempty"`
	Detail        map[string]any `json:"detail,omitempty"`
}

// SourceHealth aggregates status for GUI badges and detail panes.
type SourceHealth struct {
	SourceRef
	State         string   `json:"state"` // idle|running|ok|degraded|failed|unknown
	LastStatus    string   `json:"lastStatus,omitempty"`
	DocumentCount int64    `json:"documentCount"`
	ChunkCount    int64    `json:"chunkCount"`
	SymbolCount   int64    `json:"symbolCount"`
	ErrorCount    int      `json:"errorCount"`
	RecentErrors  []string `json:"recentErrors,omitempty"`
}

// PreviewResult is a bounded document/chunk preview.
type PreviewResult struct {
	Document    store.Document `json:"document"`
	Chunks      []store.Chunk  `json:"chunks"`
	Body        string         `json:"body,omitempty"`
	Truncated   bool           `json:"truncated,omitempty"`
	TotalChunks int            `json:"totalChunks"`
}

// SearchPlaygroundResult wraps search hits with optional plan explain.
type SearchPlaygroundResult struct {
	Query   string                `json:"query"`
	Roots   []string              `json:"roots,omitempty"`
	Hits    []store.SearchHit     `json:"hits"`
	Count   int                   `json:"count"`
	Explain []store.QueryPlanStep `json:"explain,omitempty"`
}

// ProgressEvent is emitted during long-running ingest/crawl operations.
type ProgressEvent struct {
	OpID      string    `json:"opId,omitempty"`
	Source    SourceRef `json:"source"`
	Phase     string    `json:"phase"`
	Done      int       `json:"done"`
	Total     int       `json:"total,omitempty"`
	Bytes     int64     `json:"bytes,omitempty"`
	Current   string    `json:"current,omitempty"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt int64     `json:"updatedAt"`
}

// Operation tracks one in-process admin job for progress polling.
type Operation struct {
	OpID       string         `json:"opId"`
	Source     SourceRef      `json:"source"`
	State      string         `json:"state"` // queued|running|ok|failed|cancelled
	Progress   ProgressEvent  `json:"progress"`
	StartedAt  int64          `json:"startedAt"`
	FinishedAt int64          `json:"finishedAt,omitempty"`
	Errors     []string       `json:"errors,omitempty"`
	Report     map[string]any `json:"report,omitempty"`
}

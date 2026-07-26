// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package httpapi implements the ImplCache Librarian REST API v1: a stable
// HTTP surface for source inventory, ingest, search, and job-progress
// operations built on the same store/librarian/web/pdf/gitrepo/ingest
// packages used by the MCP tool surface. The MCP transport itself is mounted
// separately by the calling binary (see main.go); this package only serves
// /api/v1/* and, optionally, a bundled SPA frontend.
package httpapi

import (
	"io/fs"

	"implcache-mcp/librarian"
	"implcache-mcp/store"
	"implcache-mcp/usage"
)

// Options configures the Librarian REST API v1 handler returned by NewHandler.
type Options struct {
	Store         *store.Store
	DBPath        string
	ServerVersion string

	ReadOnly       bool
	AllowIngest    bool
	AllowDelete    bool
	EnableSemantic bool

	MaxDocumentBytes int64
	MaxIngestFiles   int

	LibrarianEnabled  bool
	LibrarianBasePath string // e.g. "/"

	APIToken       string // if non-empty, require Bearer auth on /api/v1/* (administrator)
	ViewerAPIToken string // optional Bearer token with read-only role (mutations denied)

	UploadDir string // for PDF uploads

	StaticFS fs.FS // optional embedded frontend dist (may be nil)

	Tracker *librarian.Tracker // default librarian.DefaultTracker if nil

	// Usage is optional local analytics (nil-safe).
	Usage *usage.Store
}

// normalize fills in defaults for optional fields.
func (o Options) normalize() Options {
	if o.Tracker == nil {
		o.Tracker = librarian.DefaultTracker
	}
	if o.LibrarianBasePath == "" {
		o.LibrarianBasePath = "/"
	}
	if o.MaxIngestFiles <= 0 {
		o.MaxIngestFiles = 50000
	}
	if o.MaxDocumentBytes <= 0 {
		o.MaxDocumentBytes = 8 << 20
	}
	return o
}

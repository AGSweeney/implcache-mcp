// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

// ToolMode controls which MCP tool schemas are registered.
type ToolMode string

const (
	// ModeAgent registers retrieval tools only (default for coding agents).
	ModeAgent ToolMode = "agent"
	// ModeAdmin registers retrieval tools plus ingest/delete/list/vomit.
	ModeAdmin ToolMode = "admin"
)

// Options configures MCP tool safety and resource limits.
type Options struct {
	Mode ToolMode
	// EnableAdminTools forces admin tool registration even when Mode is agent.
	EnableAdminTools bool
	ReadOnly         bool
	AllowIngest      bool
	AllowDelete      bool
	AllowOutputWrite bool
	OutputRoot       string // absolute path; vomit writes only under here
	// DefaultProjectRoot is prepended when a request omits projectRoot.
	DefaultProjectRoot string
	// DefaultPreferredRoots used when a request omits preferredRoots (e.g. from manifest).
	DefaultPreferredRoots []string
	MaxResults            int
	MaxIngestFiles        int
	MaxDocumentBytes      int64
}

// EffectiveMode returns the tool registration mode.
func (o Options) EffectiveMode() ToolMode {
	if o.EnableAdminTools || o.Mode == ModeAdmin {
		return ModeAdmin
	}
	if o.Mode == "" {
		return ModeAgent
	}
	return o.Mode
}

// AdminEnabled reports whether administrative tools should be registered.
func (o Options) AdminEnabled() bool {
	return o.EffectiveMode() == ModeAdmin
}

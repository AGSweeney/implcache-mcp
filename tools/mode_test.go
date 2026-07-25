// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

import (
	"slices"
	"testing"
)

func TestPlannedToolsAgentDefault(t *testing.T) {
	names := PlannedTools(Options{})
	for _, a := range AgentTools {
		if !slices.Contains(names, a) {
			t.Fatalf("agent missing %s", a)
		}
	}
	for _, a := range AdminOnlyTools {
		if slices.Contains(names, a) {
			t.Fatalf("agent should not expose %s", a)
		}
	}
}

func TestPlannedToolsAdmin(t *testing.T) {
	names := PlannedTools(Options{Mode: ModeAdmin})
	for _, a := range AdminOnlyTools {
		if !slices.Contains(names, a) {
			t.Fatalf("admin missing %s", a)
		}
	}
	names = PlannedTools(Options{Mode: ModeAgent, EnableAdminTools: true})
	if !slices.Contains(names, "vomit") {
		t.Fatal("enable-admin-tools should expose vomit")
	}
}

func TestReadOnlyDoesNotChangePlannedSurface(t *testing.T) {
	// Read-only gates calls; admin schemas still register in admin mode.
	names := PlannedTools(Options{Mode: ModeAdmin, ReadOnly: true})
	if !slices.Contains(names, "ingest_project") {
		t.Fatal("readonly should still register admin schemas when mode=admin")
	}
}

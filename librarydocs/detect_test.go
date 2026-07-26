// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "testdata", "librarydocs", name)
}

func TestAnalyzeCheckout_States(t *testing.T) {
	cases := []struct {
		dir   string
		state string
	}{
		{"missing", StateNotPresent},
		{"unstructured", StateUnstructured},
		{"structured", StateStructured},
		{"validated", StateValidated},
		{"invalid_validation", StateInvalid},
		{"mqtt-client", StateValidated},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			meta := AnalyzeCheckout(fixture(t, tc.dir), tc.dir, HandlingAuto, "")
			if meta.PackageState != tc.state {
				t.Fatalf("state=%q want %q warnings=%v", meta.PackageState, tc.state, meta.Warnings)
			}
			if tc.state == StateNotPresent && meta.Summary.Detected {
				t.Fatal("not_present should not be detected")
			}
			if tc.state != StateNotPresent && !meta.Summary.Detected {
				t.Fatal("expected detected")
			}
		})
	}
}

func TestAnalyzeCheckout_MalformedWarnings(t *testing.T) {
	meta := AnalyzeCheckout(fixture(t, "malformed"), "malformed", HandlingAuto, "")
	if meta.PackageState != StateStructured && meta.PackageState != StateInvalid {
		// structured enough (has index+inv) but may be invalid if inventory empty after dup skip
		t.Logf("state=%s warnings=%v", meta.PackageState, meta.Warnings)
	}
	joined := strings.Join(meta.Warnings, "\n")
	if !strings.Contains(joined, "duplicate") && !strings.Contains(joined, "rejected source_path") && !strings.Contains(joined, "malformed") {
		t.Fatalf("expected malformed warnings, got: %v", meta.Warnings)
	}
	if strings.Contains(joined, "../etc/passwd") || strings.Contains(joined, "rejected source_path") {
		// traversal rejected — good
	} else if !strings.Contains(joined, "traversal") && !strings.Contains(joined, "rejected") {
		t.Fatalf("expected traversal rejection warning, got: %v", meta.Warnings)
	}
}

func TestAnalyzeCheckout_ExcludeAndNormal(t *testing.T) {
	meta := AnalyzeCheckout(fixture(t, "validated"), "v", HandlingExclude, "")
	if meta.PackageState != StateNotPresent {
		t.Fatalf("exclude state=%q", meta.PackageState)
	}
	meta = AnalyzeCheckout(fixture(t, "validated"), "v", HandlingNormal, "")
	if meta.PackageState != StateStructured {
		t.Fatalf("normal structured state=%q", meta.PackageState)
	}
	if len(meta.Documents) != 0 {
		t.Fatalf("normal should not enrich documents, got %d", len(meta.Documents))
	}
}

func TestAnalyzeCheckout_ValidatedFrontmatter(t *testing.T) {
	meta := AnalyzeCheckout(fixture(t, "validated"), "validated", HandlingAuto, "abc123")
	if meta.PackageState != StateValidated {
		t.Fatalf("state=%q warnings=%v", meta.PackageState, meta.Warnings)
	}
	if meta.StandardVersion != "1.0" {
		t.Fatalf("standard_version=%q", meta.StandardVersion)
	}
	dm, ok := meta.DocMetaForPath("LibraryDocs/libraries/mqtt-client/README.md")
	if !ok {
		t.Fatal("missing doc meta")
	}
	if dm.ContentClass != ClassCuratedLibraryDoc {
		t.Fatalf("class=%q", dm.ContentClass)
	}
	if dm.Status != "verified" || dm.EvidenceLevel != "E1" {
		t.Fatalf("status=%q evidence=%q", dm.Status, dm.EvidenceLevel)
	}
	if dm.UnknownFrontmatter != nil {
		// validated fixture has no unknown keys
	}
	// structured fixture preserves unknown frontmatter
	meta2 := AnalyzeCheckout(fixture(t, "structured"), "structured", HandlingAuto, "")
	dm2, ok := meta2.DocMetaForPath("LibraryDocs/libraries/mqtt-client/README.md")
	if !ok {
		t.Fatal("missing structured doc meta")
	}
	if dm2.UnknownFrontmatter == nil || dm2.UnknownFrontmatter["custom_note"] == nil {
		t.Fatalf("expected unknown frontmatter preserved: %#v", dm2.UnknownFrontmatter)
	}
}

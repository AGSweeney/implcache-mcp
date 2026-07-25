// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import "testing"

func TestFreshnessVersionSpecificAndMixed(t *testing.T) {
	cits := []Citation{{
		URI: "project://example-device-sdk/docs/v3.2/api.md", Authority: "official_documentation",
	}}
	if got := freshnessFromSources(cits, nil, 0); got != "version-specific" {
		t.Fatalf("got %q want version-specific", got)
	}

	cits = []Citation{
		{URI: "project://sdk/docs/v1.0/a.md", Authority: "official_documentation", Version: "1.0"},
		{URI: "project://sdk/docs/v2.0/b.md", Authority: "official_documentation", Version: "2.0"},
	}
	if got := freshnessFromSources(cits, nil, 0); got != "mixed" {
		t.Fatalf("got %q want mixed", got)
	}

	if got := freshnessFromSources([]Citation{{URI: "x", Authority: "current_project"}}, nil, 1); got != "stale" {
		t.Fatalf("got %q want stale", got)
	}
}

func TestFreshnessAuthorityNotCurrent(t *testing.T) {
	// Official docs without version metadata are authoritative but freshness-unknown.
	docs := []Citation{{URI: "project://sdk/api.md", Authority: "official_documentation"}}
	if got := freshnessFromSources(docs, nil, 0); got != "unknown" {
		t.Fatalf("docs-only: got %q want unknown", got)
	}
	proj := []Citation{{URI: "project://app/main.cpp", Authority: "current_project"}}
	if got := freshnessFromSources(proj, nil, 0); got != "current" {
		t.Fatalf("project: got %q want current", got)
	}
	both := []Citation{
		{URI: "project://app/main.cpp", Authority: "current_project"},
		{URI: "project://sdk/api.md", Authority: "official_documentation"},
	}
	if got := freshnessFromSources(both, nil, 0); got != "current" {
		t.Fatalf("project+docs: got %q want current", got)
	}
}

func TestWebSearchFromCoverageFreshness(t *testing.T) {
	cases := []struct {
		coverage, freshness string
		want                bool
	}{
		{"high", "current", false},
		{"high", "unknown", true},
		{"high", "stale", true},
		{"high", "mixed", true},
		{"low", "current", true},
		{"medium", "version-specific", false},
	}
	for _, tc := range cases {
		if got := webSearchFrom(tc.coverage, tc.freshness); got != tc.want {
			t.Fatalf("%s/%s: got %v want %v", tc.coverage, tc.freshness, got, tc.want)
		}
	}
}

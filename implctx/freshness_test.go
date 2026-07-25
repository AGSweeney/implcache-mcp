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

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestLargeCorpusRootScopedPlan builds a multi-root corpus and verifies
// root-filtered search stays correct and EXPLAIN QUERY PLAN remains usable.
func TestLargeCorpusRootScopedPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("large corpus")
	}
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "big.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	roots := []string{"example-control-app", "example-device-sdk", "example-plugin-sdk", "noise-a", "noise-b"}
	const docsPerRoot = 80
	for _, root := range roots {
		for i := 0; i < docsPerRoot; i++ {
			body := fmt.Sprintf("generic prose document %d about configuration and setup", i)
			syms := []SymbolInput{}
			if root == "example-device-sdk" && i == 7 {
				body = "SpiTransfer ConfigurePin device driver initialization sequence"
				syms = []SymbolInput{{Name: "SpiTransfer", Kind: "function", Language: "cpp", StartLine: 1}}
			}
			_, err := st.UpsertDocument(ctx, UpsertInput{
				URI: fmt.Sprintf("project://%s/doc_%03d.md", root, i), Title: fmt.Sprintf("doc %d", i),
				SourceType: SourceMarkdown, Path: fmt.Sprintf("docs/doc_%03d.md", i), RootName: root,
				Authority: AuthorityOfficialDocs, Hash: fmt.Sprintf("%s-%d", root, i),
				Chunks:  []Chunk{{Heading: "Overview", Body: body, StartLine: 1, EndLine: 5}},
				Symbols: syms,
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	hits, err := st.SearchOpts(ctx, SearchOptions{
		Query: "SpiTransfer ConfigurePin", Limit: 20, Roots: []string{"example-device-sdk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits in device sdk root")
	}
	for _, h := range hits {
		if h.RootName != "example-device-sdk" {
			t.Fatalf("root leak: %s", h.RootName)
		}
	}

	plan, err := st.ExplainSearchPlan(ctx, "SpiTransfer", []string{"example-device-sdk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) == 0 {
		t.Fatal("empty query plan")
	}
	joined := ""
	for _, p := range plan {
		joined += p.Detail + "\n"
	}
	// Soft assertion: plan should mention MATCH and/or an index/search step.
	if !strings.Contains(strings.ToLower(joined), "match") &&
		!strings.Contains(strings.ToLower(joined), "scan") &&
		!strings.Contains(strings.ToLower(joined), "search") {
		t.Fatalf("unexpected plan details:\n%s", joined)
	}

	syms, err := st.FindSymbols(ctx, "SpiTransfer", []string{"example-device-sdk"}, 5)
	if err != nil || len(syms) == 0 {
		t.Fatalf("symbol lookup failed: %v %+v", err, syms)
	}
}

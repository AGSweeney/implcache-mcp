// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestContextFingerprintStable(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://example-plugin-sdk/api.md", Title: "API",
		SourceType: store.SourceMarkdown, Path: "api.md", RootName: "example-plugin-sdk",
		Authority: store.AuthorityOfficialDocs, Hash: "h1",
		Chunks:  []store.Chunk{{Body: "RegisterHandler initializes the plugin", StartLine: 1, EndLine: 2}},
		Symbols: []store.SymbolInput{{Name: "RegisterHandler", Kind: "function", Language: "cpp", StartLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{
		Task: "RegisterHandler plugin init", Language: "cpp", Technology: "Example Plugin SDK",
		PreferredRoots: []string{"example-plugin-sdk"}, MaxContextTokens: 1500,
	}
	a, err := Get(ctx, st, req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Get(ctx, st, req)
	if err != nil {
		t.Fatal(err)
	}
	if a.ContextFingerprint == "" || a.ContextFingerprint != b.ContextFingerprint {
		t.Fatalf("fingerprint unstable: %q vs %q", a.ContextFingerprint, b.ContextFingerprint)
	}
	// Content change should alter fingerprint.
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://example-plugin-sdk/api.md", Title: "API",
		SourceType: store.SourceMarkdown, Path: "api.md", RootName: "example-plugin-sdk",
		Authority: store.AuthorityOfficialDocs, Hash: "h2",
		Chunks:  []store.Chunk{{Body: "RegisterHandler changed body", StartLine: 1, EndLine: 2}},
		Symbols: []store.SymbolInput{{Name: "RegisterHandler", Kind: "function", Language: "cpp", StartLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Get(ctx, st, req)
	if err != nil {
		t.Fatal(err)
	}
	if c.ContextFingerprint == a.ContextFingerprint {
		t.Fatal("fingerprint should change when source content changes")
	}
}

func TestContextFingerprintAfterTrim(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "f.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	resp := &Response{
		Task:                "demo",
		RootsUsed:           []string{"example-plugin-sdk"},
		RequiredAPIs:        []string{"RegisterHandler", "AddMenuItem", "ExtraAPI"},
		RelevantSymbols:     []store.Symbol{{Name: "RegisterHandler", NameNorm: "registerhandler", URI: "u1"}},
		Examples:            []ExampleRef{{URI: "u1", Excerpt: strings.Repeat("example body word ", 40)}},
		RecommendedFollowUp: []string{"a", "b", "c"},
		Summary:             "A short summary about RegisterHandler.",
		Coverage:            "high",
		Freshness:           "current",
	}
	// Force trimming of optional fields.
	trimToBudget(resp, 60)
	if len(resp.RecommendedFollowUp) != 0 || len(resp.Examples) != 0 {
		// Ensure a clear post-trim shape for the assertion.
		resp.RecommendedFollowUp = nil
		resp.Examples = nil
		if len(resp.RelevantSymbols) > 0 {
			resp.RelevantSymbols = resp.RelevantSymbols[:1]
		}
		if len(resp.RequiredAPIs) > 1 {
			resp.RequiredAPIs = resp.RequiredAPIs[:1]
		}
	}
	req := Request{Task: "demo"}
	fp1 := fingerprintResponse(ctx, st, req, resp)
	fp2 := fingerprintResponse(ctx, st, req, resp)
	if fp1 == "" || fp1 != fp2 {
		t.Fatalf("unstable after trim: %q vs %q", fp1, fp2)
	}
	resp.RequiredAPIs = append(resp.RequiredAPIs, "ExtraAPI")
	fp3 := fingerprintResponse(ctx, st, req, resp)
	if fp3 == fp1 {
		t.Fatal("fingerprint should change when trimmed-away content is restored")
	}
}

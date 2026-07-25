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

func TestNaturalLanguageTaskHarvestsSymbols(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "nl.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://example-network-sdk/retry.cpp", Title: "retry",
		SourceType: store.SourceSource, Path: "src/retry.cpp", RootName: "example-network-sdk",
		Authority: store.AuthorityOfficialDocs, Hash: "n1",
		Chunks: []store.Chunk{{
			Heading:   "Reconnect handling",
			Body:      "Implement reconnect with backoff policy helpers in the network client.",
			StartLine: 1, EndLine: 20,
		}},
		Symbols: []store.SymbolInput{
			{Name: "RetryPolicy", Kind: "type", Language: "cpp", StartLine: 5},
			{Name: "Connect", Kind: "function", Language: "cpp", StartLine: 10},
			{Name: "Disconnect", Kind: "function", Language: "cpp", StartLine: 15},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// No PascalCase identifiers in the task — must come from retrieved docs.
	res, err := Get(ctx, st, Request{
		Task:             "add reconnect and backoff handling to a network client",
		Language:         "cpp",
		Technology:       "Example Networking SDK",
		PreferredRoots:   []string{"example-network-sdk"},
		MaxContextTokens: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	blob := strings.ToLower(strings.Join(res.RequiredAPIs, " "))
	for _, s := range res.RelevantSymbols {
		blob += " " + strings.ToLower(s.Name)
	}
	if !strings.Contains(blob, "retry") && !strings.Contains(blob, "connect") {
		t.Fatalf("expected NL harvest of network symbols, got apis=%v syms=%v", res.RequiredAPIs, res.RelevantSymbols)
	}
}

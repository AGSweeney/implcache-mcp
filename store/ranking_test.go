// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFilenameAndAuthorityRanking(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-control-app/src/client_connection.cpp", Title: "other",
		SourceType: SourceSource, Path: "src/client_connection.cpp", RootName: "example-control-app",
		Authority: AuthorityCurrentProject, Hash: "a",
		Chunks: []Chunk{{Heading: "misc", Body: "general prose about networking", StartLine: 1, EndLine: 2}},
	})
	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-device-docs/guide.md", Title: "Reconnect guide",
		SourceType: SourceMarkdown, Path: "guide.md", RootName: "example-device-docs",
		Authority: AuthorityOfficialDocs, Hash: "b",
		Chunks: []Chunk{{Heading: "ReconnectPolicy", Body: "client_connection reconnect prose", StartLine: 1, EndLine: 2}},
	})
	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://archived-reference/old.md", Title: "old",
		SourceType: SourceMarkdown, Path: "archive/old.md", RootName: "archived-reference",
		Authority: AuthorityThirdParty, Archived: true, Hash: "c",
		Chunks: []Chunk{{Heading: "client_connection", Body: "client_connection archived notes", StartLine: 1, EndLine: 2}},
	})

	hits, err := st.SearchOpts(ctx, SearchOptions{
		Query: "client_connection", Limit: 10,
		Roots: []string{"example-control-app", "example-device-docs", "archived-reference"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	if hits[0].RootName != "example-control-app" {
		t.Fatalf("expected current project first, got %+v", hits[0])
	}
	if hits[0].MatchKind != "filename" && hits[0].MatchKind != "path" {
		t.Fatalf("expected filename/path matchKind, got %q score=%.2f", hits[0].MatchKind, hits[0].Score)
	}
}

func TestExplainSearchPlan(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-control-app/a.md", Title: "a", SourceType: SourceMarkdown,
		Path: "a.md", RootName: "example-control-app", Hash: "1",
		Chunks: []Chunk{{Body: "RegisterHandler example", StartLine: 1, EndLine: 1}},
	})
	plan, err := st.ExplainSearchPlan(ctx, "RegisterHandler", []string{"example-control-app"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) == 0 {
		t.Fatal("empty plan")
	}
}

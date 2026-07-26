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

func TestAuthorityTierBeatsPathBias(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "tier.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://proj/notes.md", Title: "notes",
		SourceType: SourceMarkdown, Path: "notes.md", RootName: "proj",
		Authority: AuthorityCurrentProject, Hash: "p",
		Chunks: []Chunk{{Heading: "misc", Body: "mentions WidgetAPI briefly", StartLine: 1, EndLine: 1}},
	})
	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://gen/WidgetAPI.md", Title: "WidgetAPI",
		SourceType: SourceMarkdown, Path: "WidgetAPI.md", RootName: "gen",
		Authority: AuthorityGeneratedSummary, Hash: "g",
		Chunks: []Chunk{{Heading: "WidgetAPI", Body: "WidgetAPI generated summary full match", StartLine: 1, EndLine: 1}},
	})

	hits, err := st.SearchOpts(ctx, SearchOptions{
		Query: "WidgetAPI", Limit: 10, Roots: []string{"proj", "gen"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 2 {
		t.Fatalf("want both hits, got %+v", hits)
	}
	if hits[0].Authority != AuthorityCurrentProject {
		t.Fatalf("current_project must beat generated+filename, got %+v", hits[0])
	}
}

func TestAuthorityBoostOrderMatchesDocs(t *testing.T) {
	order := []string{
		AuthorityCurrentProject,
		AuthorityRelatedProject,
		AuthorityCuratedRecipe,
		AuthorityOfficialExample,
		AuthorityOfficialDocs,
		AuthorityGeneratedSummary,
		AuthorityThirdParty,
		AuthorityUnknown,
	}
	for i := 1; i < len(order); i++ {
		if AuthorityRank(order[i-1]) >= AuthorityRank(order[i]) {
			t.Fatalf("rank order broken: %s (%d) vs %s (%d)",
				order[i-1], AuthorityRank(order[i-1]), order[i], AuthorityRank(order[i]))
		}
		if AuthorityBoost(order[i-1]) <= AuthorityBoost(order[i]) && order[i] != AuthorityUnknown {
			// unknown is 0; third_party is 4 — allow equality only if identical (shouldn't happen)
			if AuthorityBoost(order[i-1]) < AuthorityBoost(order[i]) {
				t.Fatalf("boost order broken: %s (%.0f) vs %s (%.0f)",
					order[i-1], AuthorityBoost(order[i-1]), order[i], AuthorityBoost(order[i]))
			}
		}
	}
	if AuthorityBoost(AuthorityGeneratedSummary) <= AuthorityBoost(AuthorityThirdParty) {
		t.Fatalf("generated must outrank third_party in boost")
	}
}

func TestDeprecatedPenaltyInScore(t *testing.T) {
	h := SearchHit{
		Authority: AuthorityOfficialDocs, Rank: -1,
		Path: "api.md", Title: "API", Heading: "API", Snippet: "API details",
		Deprecated: true,
	}
	scoreDep, _ := compositeScore(h, "API")
	h.Deprecated = false
	scoreOK, _ := compositeScore(h, "API")
	if scoreDep >= scoreOK {
		t.Fatalf("deprecated should lower score: dep=%.2f ok=%.2f", scoreDep, scoreOK)
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

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpsertRecipeCannotElevateGeneratedAuthority(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	id, err := st.UpsertKnowledgeEntry(ctx, KnowledgeEntry{
		URI: "project://demo/_recipes/x.md", Subject: "x", BodyMarkdown: "# x",
		ReviewStatus: ReviewGenerated, Authority: AuthorityCuratedRecipe,
		RootName: "demo", SourceURIs: []string{"project://demo/a.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := st.SearchKnowledgeEntries(ctx, "x", "", "", []string{"demo"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != id {
		t.Fatalf("entries=%+v", entries)
	}
	if entries[0].Authority != AuthorityGeneratedSummary {
		t.Fatalf("authority=%q want generated_summary", entries[0].Authority)
	}
	if len(entries[0].SourceURIs) != 1 || entries[0].SourceURIs[0] != "project://demo/a.md" {
		t.Fatalf("lineage=%v", entries[0].SourceURIs)
	}
}

func TestSearchKnowledgeEntriesLoadsSourceURIs(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertKnowledgeEntry(ctx, KnowledgeEntry{
		URI: "project://demo/_recipes/y.md", Subject: "y topic", BodyMarkdown: "body y",
		ReviewStatus: ReviewHumanReviewed, Authority: AuthorityCuratedRecipe,
		RootName: "demo", SourceURIs: []string{"project://demo/b.md", "project://demo/c.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := st.SearchKnowledgeEntries(ctx, "y topic", "", "", []string{"demo"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].SourceURIs) != 2 {
		t.Fatalf("want 2 sources, got %+v", entries)
	}
}

func TestSetKnowledgeEntryReviewStatus(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	uri := "project://demo/_recipes/z.md"
	_, err = st.UpsertKnowledgeEntry(ctx, KnowledgeEntry{
		URI: uri, Subject: "z", BodyMarkdown: "body",
		ReviewStatus: ReviewGenerated, RootName: "demo",
		SourceURIs: []string{"project://demo/d.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetKnowledgeEntryReviewStatus(ctx, uri, ReviewHumanReviewed); err != nil {
		t.Fatal(err)
	}
	entries, err := st.SearchKnowledgeEntries(ctx, "z", "", "", []string{"demo"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].ReviewStatus != ReviewHumanReviewed || entries[0].Authority != AuthorityCuratedRecipe {
		t.Fatalf("promote failed: %+v", entries[0])
	}
	if entries[0].VerifiedAt == 0 {
		t.Fatal("expected verified_at")
	}
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"context"
	"path/filepath"
	"testing"

	"implcache-mcp/store"
)

func TestScoreContribution_Additive(t *testing.T) {
	cfg := DefaultRankingConfig()
	cfg.Enabled = true
	base := 100.0
	dm := DocMeta{Status: "verified", EvidenceLevel: "E1", ContentClass: ClassCuratedLibraryDoc}
	contrib := scoreContribution(cfg, StateValidated, dm)
	if contrib <= 0 {
		t.Fatalf("expected positive boost, got %v", contrib)
	}
	if base+contrib <= base {
		t.Fatal("boost must be additive")
	}
	pen := scoreContribution(cfg, StateValidated, DocMeta{
		Status: "draft", EvidenceLevel: "E4", ContentClass: ClassIndex,
	})
	if pen >= 0 {
		t.Fatalf("expected penalty, got %v", pen)
	}
}

func TestPersistLoadMeta(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	meta := AnalyzeCheckout(fixture(t, "validated"), "validated", HandlingAuto, "deadbeef")
	uri, err := PersistMeta(ctx, st, "project", "validated", meta)
	if err != nil {
		t.Fatal(err)
	}
	if uri != MetaURI("project", "validated") {
		t.Fatalf("uri=%q", uri)
	}
	got, err := LoadMeta(ctx, st, "project", "validated")
	if err != nil || got == nil {
		t.Fatalf("load err=%v meta=%v", err, got)
	}
	if got.PackageState != StateValidated {
		t.Fatalf("state=%q", got.PackageState)
	}
	if err := DeleteMeta(ctx, st, "project", "validated"); err != nil {
		t.Fatal(err)
	}
	got, err = LoadMeta(ctx, st, "project", "validated")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestFilterHits(t *testing.T) {
	hits := []store.SearchHit{
		{Path: "src/a.go", Score: 1},
		{Path: "LibraryDocs/libraries/x/README.md", Score: 2, LibraryDocs: &HitMeta{Level: "library", Status: "verified"}},
	}
	only := FilterHits(hits, true, false, "", "")
	if len(only) != 1 {
		t.Fatalf("libraryDocsOnly=%d", len(only))
	}
	excl := FilterHits(hits, false, true, "", "")
	if len(excl) != 1 || excl[0].Path != "src/a.go" {
		t.Fatalf("exclude=%+v", excl)
	}
	st := FilterHits(hits, false, false, "library", "verified")
	if len(st) != 1 {
		t.Fatalf("level/status filter=%d", len(st))
	}
}

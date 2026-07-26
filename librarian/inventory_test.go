// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian_test

import (
	"context"
	"path/filepath"
	"testing"

	"implcache-mcp/librarian"
	"implcache-mcp/store"
)

func TestListSourcesUnified(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "lib.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name: "docs", RootName: "docs-root", StartURL: "https://example.com/docs/",
		Profile: "sphinx", AllowedPrefixes: []string{"https://example.com/docs/"},
		Authority: store.AuthorityOfficialDocs, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://local-root/a.md", Title: "A", SourceType: store.SourceMarkdown,
		RootName: "local-root", Hash: "h1", Chunks: []store.Chunk{{Body: "hello", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	list, err := librarian.ListSources(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[librarian.SourceKind]bool{}
	for _, s := range list {
		kinds[s.Kind] = true
	}
	if !kinds[librarian.KindWeb] || !kinds[librarian.KindLocal] {
		t.Fatalf("expected web+local sources, got %#v", list)
	}

	h, err := librarian.GetSourceHealth(ctx, st, librarian.KindWeb, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if h.State == "" {
		t.Fatal("expected health state")
	}

	prev, err := librarian.PreviewDocument(ctx, st, librarian.PreviewOptions{
		URI: "project://local-root/a.md", MaxChunks: 1, MaxChars: 100, IncludeBody: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prev.Body == "" || prev.TotalChunks != 1 {
		t.Fatalf("preview=%+v", prev)
	}
}

func TestLocalIndexedSourceCountsAsHealthy(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "stats-local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://local-root/a.md", Title: "A", SourceType: store.SourceMarkdown,
		RootName: "local-root", Hash: "h1", Chunks: []store.Chunk{{Body: "hello", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats, err := librarian.GetLibraryStats(ctx, st, filepath.Join(t.TempDir(), "missing.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.SourcesTotal < 1 || stats.SourcesOK < 1 {
		t.Fatalf("expected local root counted healthy: %+v", stats)
	}

	h, err := librarian.GetSourceHealth(ctx, st, librarian.KindLocal, "local-root")
	if err != nil {
		t.Fatal(err)
	}
	if h.State != "ok" {
		t.Fatalf("local health state=%q want ok", h.State)
	}
}

func TestOperationTracker(t *testing.T) {
	tr := librarian.NewTracker(8)
	id := tr.Start(librarian.SourceRef{Kind: librarian.KindWeb, ID: "x"}, "crawl")
	tr.Update(id, librarian.ProgressEvent{Done: 3, Total: 10, Current: "https://example.com/"})
	op, ok := tr.Get(id)
	if !ok || op.Progress.Done != 3 {
		t.Fatalf("op=%+v ok=%v", op, ok)
	}
	tr.Finish(id, "ok", map[string]any{"new": 3}, nil)
	op, _ = tr.Get(id)
	if op.State != "ok" || op.FinishedAt == 0 {
		t.Fatalf("finished=%+v", op)
	}
}

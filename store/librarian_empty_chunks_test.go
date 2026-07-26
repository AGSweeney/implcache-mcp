// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDocumentsWithoutChunksReport(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "empty-chunks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	rep, err := st.DocumentsWithoutChunksReport(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 0 || len(rep.ByRoot) != 0 {
		t.Fatalf("expected empty report, got %+v", rep)
	}

	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://demo/a.md", Title: "a", SourceType: SourceMarkdown,
		Path: "a.md", RootName: "demo", Authority: AuthorityOfficialDocs, Hash: "h1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://demo/b.md", Title: "b", SourceType: SourceMarkdown,
		Path: "b.md", RootName: "demo", Authority: AuthorityOfficialDocs, Hash: "h2",
		Chunks: []Chunk{{Body: "hello world"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://other/c.md", Title: "c", SourceType: SourceMarkdown,
		Path: "c.md", RootName: "other", Authority: AuthorityOfficialDocs, Hash: "h3",
	}); err != nil {
		t.Fatal(err)
	}

	rep, err = st.DocumentsWithoutChunksReport(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 2 {
		t.Fatalf("total=%d want 2", rep.Total)
	}
	if len(rep.ByRoot) < 2 {
		t.Fatalf("byRoot=%+v", rep.ByRoot)
	}
	if len(rep.SampleURIs) != 2 {
		t.Fatalf("samples=%v", rep.SampleURIs)
	}

	deleted, err := st.DeleteDocumentsWithoutChunks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted=%d want 2", deleted)
	}
	rep, err = st.DocumentsWithoutChunksReport(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 0 {
		t.Fatalf("after purge total=%d", rep.Total)
	}
	// Document with chunks must remain.
	doc, _, err := st.GetDocumentByURI(ctx, "project://demo/b.md")
	if err != nil || doc == nil {
		t.Fatalf("kept doc missing: %v", err)
	}
}

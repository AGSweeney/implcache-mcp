// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpsertSearchGet(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	written, err := st.UpsertDocument(ctx, UpsertInput{
		URI:        "file:///D:/docs/guide.md",
		Title:      "guide.md",
		SourceType: SourceMarkdown,
		Path:       "D:/docs/guide.md",
		Hash:       "abc123",
		Chunks: []Chunk{
			{Heading: "Intro", Body: "SQLite backed knowledge base for agents", StartLine: 1, EndLine: 3},
			{Heading: "Usage", Body: "Call search_knowledge with a query", StartLine: 5, EndLine: 8},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected write")
	}

	// Same hash skips
	written, err = st.UpsertDocument(ctx, UpsertInput{
		URI:        "file:///D:/docs/guide.md",
		Title:      "guide.md",
		SourceType: SourceMarkdown,
		Path:       "D:/docs/guide.md",
		Hash:       "abc123",
		Chunks:     []Chunk{{Body: "ignored"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Fatal("expected skip on same hash")
	}

	hits, err := st.Search(ctx, "knowledge", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected search hits")
	}
	if hits[0].Snippet == "" {
		t.Fatal("expected snippet")
	}

	doc, chunks, err := st.GetDocumentByURI(ctx, "file:///D:/docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "guide.md" || len(chunks) != 2 {
		t.Fatalf("doc=%+v chunks=%d", doc, len(chunks))
	}

	docs, err := st.ListDocuments(ctx, SourceMarkdown)
	if err != nil || len(docs) != 1 {
		t.Fatalf("list: %v len=%d", err, len(docs))
	}

	ok, err := st.DeleteDocument(ctx, "file:///D:/docs/guide.md")
	if err != nil || !ok {
		t.Fatalf("delete: ok=%v err=%v", ok, err)
	}
}

func TestProjectURIDocument(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI:        "project://demo/cmd/main.go",
		Title:      "main.go",
		SourceType: SourceSource,
		Path:       "cmd/main.go",
		RootName:   "demo",
		Hash:       "deadbeef",
		Chunks:     []Chunk{{Body: "package main\nfunc main() {}", StartLine: 1, EndLine: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := st.GetDocumentByURI(ctx, "project://demo/cmd/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if doc.RootName != "demo" {
		t.Fatalf("rootName=%q", doc.RootName)
	}
}

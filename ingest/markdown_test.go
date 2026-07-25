// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestIngestMarkdownAndSearch(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(md, []byte("# Notes\n\nKnowledge about WAL mode and FTS5 search.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	res, err := IngestMarkdown(ctx, st, md, false, "notes-root")
	if err != nil {
		t.Fatal(err)
	}
	if res.Ingested != 1 {
		t.Fatalf("ingested=%d errors=%v", res.Ingested, res.Errors)
	}
	if len(res.URIs) != 1 || res.URIs[0] != "project://notes-root/notes.md" {
		t.Fatalf("uris=%v", res.URIs)
	}
	doc, _, err := st.GetDocumentByURI(ctx, "project://notes-root/notes.md")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path != "notes.md" || doc.RootName != "notes-root" {
		t.Fatalf("path=%q root=%q", doc.Path, doc.RootName)
	}

	// Second pass should skip
	res2, err := IngestMarkdown(ctx, st, md, false, "notes-root")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Skipped != 1 {
		t.Fatalf("skipped=%d", res2.Skipped)
	}

	hits, err := st.Search(ctx, "FTS5", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if !strings.HasPrefix(hits[0].URI, "project://") || strings.HasPrefix(hits[0].URI, "file:///") {
		t.Fatalf("URI should be portable project://, got %q", hits[0].URI)
	}
}

func TestIngestMarkdownWritesTermVectors(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "reconnect.md")
	body := `# Reconnect Handling

A network client should reconnect with bounded exponential backoff.

Use RetryPolicy to calculate the delay and reset the retry counter after a successful connection.
`
	if err := os.WriteFile(md, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if _, err := IngestMarkdown(ctx, st, md, false, "example-network-sdk"); err != nil {
		t.Fatal(err)
	}
	n, err := st.TermVectorCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("term vectors=%d want at least one", n)
	}
	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query: "network retry reconnect", Roots: []string{"example-network-sdk"}, Semantic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected semantic/FTS result from markdown ingestion")
	}
}

func TestIngestProject(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "x", "index.js"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(filepath.Join(root, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	res, err := IngestProject(ctx, st, root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if res.Ingested < 1 {
		t.Fatalf("ingested=%d errors=%v", res.Ingested, res.Errors)
	}
	for _, u := range res.URIs {
		if u == "project://demo/node_modules/x/index.js" {
			t.Fatal("node_modules should be ignored")
		}
	}

	doc, _, err := st.GetDocumentByURI(ctx, "project://demo/cmd/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if doc.RootName != "demo" {
		t.Fatalf("root=%q", doc.RootName)
	}
}

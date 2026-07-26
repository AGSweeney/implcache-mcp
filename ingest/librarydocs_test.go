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

	"implcache-mcp/librarydocs"
	"implcache-mcp/store"
)

func TestIngestProject_LibraryDocsValidated(t *testing.T) {
	src := filepath.Join("..", "testdata", "librarydocs", "mqtt-client")
	st, err := store.Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	res, err := IngestProjectOpts(ctx, st, ProjectOptions{
		Path: src, RootName: "mqtt-client", LibraryDocsHandling: librarydocs.HandlingAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.LibraryDocs == nil || res.LibraryDocs.PackageState != librarydocs.StateValidated {
		t.Fatalf("libraryDocs=%+v", res.LibraryDocs)
	}
	meta, err := librarydocs.LoadMeta(ctx, st, "project", "mqtt-client")
	if err != nil || meta == nil {
		t.Fatalf("meta err=%v", err)
	}
	doc, _, err := st.GetDocumentByURI(ctx, "project://mqtt-client/LibraryDocs/libraries/mqtt-client/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Authority != store.AuthorityCuratedRecipe {
		t.Fatalf("authority=%q want curated", doc.Authority)
	}
	// source still ingested
	if _, _, err := st.GetDocumentByURI(ctx, "project://mqtt-client/src/mqtt_client.go"); err != nil {
		t.Fatal(err)
	}
}

func TestIngestProject_LibraryDocsExclude(t *testing.T) {
	src := filepath.Join("..", "testdata", "librarydocs", "mqtt-client")
	st, err := store.Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	res, err := IngestProjectOpts(ctx, st, ProjectOptions{
		Path: src, RootName: "mqtt-client", LibraryDocsHandling: librarydocs.HandlingExclude,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range res.URIs {
		if strings.Contains(u, "/LibraryDocs/") || strings.HasSuffix(u, "/LibraryDocs") {
			t.Fatalf("exclude should skip LibraryDocs URI %s", u)
		}
	}
	meta, err := librarydocs.LoadMeta(ctx, st, "project", "mqtt-client")
	if err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		t.Fatal("meta should be absent when excluded")
	}
	if _, _, err := st.GetDocumentByURI(ctx, "project://mqtt-client/src/mqtt_client.go"); err != nil {
		t.Fatal("source should still ingest")
	}
}

func TestIngestProject_LibraryDocsRemovedOnReimport(t *testing.T) {
	root := t.TempDir()
	// copy minimal validated package then remove LibraryDocs on second ingest
	copyTree(t, filepath.Join("..", "testdata", "librarydocs", "validated"), root)
	st, err := store.Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	res, err := IngestProjectOpts(ctx, st, ProjectOptions{Path: root, RootName: "reimport"})
	if err != nil {
		t.Fatal(err)
	}
	if res.LibraryDocs == nil || !res.LibraryDocs.Detected {
		t.Fatalf("first ingest: %+v", res.LibraryDocs)
	}
	meta, err := librarydocs.LoadMeta(ctx, st, "project", "reimport")
	if err != nil || meta == nil {
		t.Fatal("expected meta after first ingest")
	}
	if err := os.RemoveAll(filepath.Join(root, "LibraryDocs")); err != nil {
		t.Fatal(err)
	}
	res2, err := IngestProjectOpts(ctx, st, ProjectOptions{Path: root, RootName: "reimport"})
	if err != nil {
		t.Fatal(err)
	}
	if res2.LibraryDocs != nil && res2.LibraryDocs.Detected {
		t.Fatalf("after delete expected not detected: %+v", res2.LibraryDocs)
	}
	meta2, err := librarydocs.LoadMeta(ctx, st, "project", "reimport")
	if err != nil {
		t.Fatal(err)
	}
	if meta2 != nil {
		t.Fatal("meta should be removed when LibraryDocs/ deleted")
	}
}

func TestIngestProject_OrdinaryRepoUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	res, err := IngestProject(ctx, st, root, "plain")
	if err != nil {
		t.Fatal(err)
	}
	if res.Ingested < 1 {
		t.Fatalf("ingested=%d", res.Ingested)
	}
	if res.LibraryDocs == nil || res.LibraryDocs.Detected {
		t.Fatalf("ordinary repo should report not detected: %+v", res.LibraryDocs)
	}
	meta, err := librarydocs.LoadMeta(ctx, st, "project", "plain")
	if err != nil {
		t.Fatal(err)
	}
	if meta != nil {
		t.Fatal("no meta doc for ordinary repo")
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

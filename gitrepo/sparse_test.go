// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/gitrepo"
	"implcache-mcp/store"
)

func TestSparseCheckout(t *testing.T) {
	requireGit(t)
	src := initRepo(t, filepath.Join(t.TempDir(), "src"), map[string]string{
		"components/net/api.go": "package net\n\nfunc NetOnly() {}\n",
		"docs/readme.md":        "# Docs\n",
		"other/skip.go":         "package other\n\nfunc SkipMe() {}\n",
	})
	remote := bareClone(t, src, filepath.Join(t.TempDir(), "r.git"))
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cache := filepath.Join(t.TempDir(), "cache")
	res, err := gitrepo.IngestRepo(context.Background(), st, gitrepo.IngestOptions{
		Name: "sparse", RemoteURL: remote, RootName: "sparse",
		AcquisitionMode: "managed_clone", Ref: "main", PersistSource: true, CacheRoot: cache,
		SparsePaths:     []string{"components", "docs"},
		IncludePatterns: []string{"**/*"},
		ExcludePatterns: []string{".git/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.DocumentsIngested < 1 {
		t.Fatalf("report=%+v", res)
	}
	hits, _ := st.SearchOpts(context.Background(), store.SearchOptions{Query: "SkipMe", Limit: 5, Roots: []string{"sparse"}})
	if len(hits) != 0 {
		t.Fatal("other/ should be sparse-excluded")
	}
	hits2, _ := st.SearchOpts(context.Background(), store.SearchOptions{Query: "NetOnly", Limit: 5, Roots: []string{"sparse"}})
	if len(hits2) == 0 {
		t.Fatal("expected components content")
	}
	rs, _ := st.GetRepoSourceByName(context.Background(), "sparse")
	for _, f := range mustFiles(t, st, rs.ID) {
		if strings.HasPrefix(f.RelativePath, "other/") {
			t.Fatalf("unexpected %s", f.RelativePath)
		}
	}
}

func mustFiles(t *testing.T, st *store.Store, id int64) []store.RepoFile {
	t.Helper()
	files, err := st.ListRepoFiles(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

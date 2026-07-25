// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"implcache-mcp/gitrepo"
	"implcache-mcp/store"
)

func TestScaleManyFilesSparse(t *testing.T) {
	requireGit(t)
	if testing.Short() {
		t.Skip("scale")
	}
	root := filepath.Join(t.TempDir(), "mono")
	files := map[string]string{
		"keep/core.go": "package keep\n\nfunc CoreAPI() {}\n",
	}
	for i := 0; i < 200; i++ {
		files[fmt.Sprintf("noise/f%03d.go", i)] = fmt.Sprintf("package noise\n\nfunc N%d() {}\n", i)
	}
	initRepo(t, root, files)
	remote := bareClone(t, root, filepath.Join(t.TempDir(), "r.git"))
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cache := filepath.Join(t.TempDir(), "cache")
	res, err := gitrepo.IngestRepo(context.Background(), st, gitrepo.IngestOptions{
		Name: "mono", RemoteURL: remote, RootName: "mono",
		AcquisitionMode: "managed_clone", Ref: "main", PersistSource: true, CacheRoot: cache,
		SparsePaths:     []string{"keep"},
		IncludePatterns: []string{"**/*.go"},
		ExcludePatterns: []string{".git/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.DocumentsIngested != 1 {
		t.Fatalf("sparse should index 1 file, got %d", res.DocumentsIngested)
	}
	_ = os.RemoveAll(cache)
}

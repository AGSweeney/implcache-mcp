// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/gitrepo"
	"implcache-mcp/store"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func initRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	requireGit(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

func bareClone(t *testing.T, src, bare string) string {
	t.Helper()
	cmd := exec.Command("git", "clone", "--bare", src, bare)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare clone: %v\n%s", err, out)
	}
	// file URL for Windows
	abs, _ := filepath.Abs(bare)
	return "file://" + filepath.ToSlash(abs)
}

func TestSnapshotIngestLocalRemote(t *testing.T) {
	requireGit(t)
	src := initRepo(t, filepath.Join(t.TempDir(), "src"), map[string]string{
		"README.md":        "# Demo\n\nRetryPolicy reconnect backoff\n",
		"pkg/client.go":    "package pkg\n\nfunc RegisterHandler() {}\n",
		"examples/demo.go": "package main\n\nfunc main() {}\n",
		"vendor/skip.go":   "package vendor\n",
	})
	remote := bareClone(t, src, filepath.Join(t.TempDir(), "remote.git"))

	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	cache := filepath.Join(t.TempDir(), "cache")

	rep, err := gitrepo.InspectRepo(ctx, gitrepo.InspectOptions{RemoteURL: remote, Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Accessible || rep.ResolvedCommitSHA == "" {
		t.Fatalf("inspect=%+v", rep)
	}

	res, err := gitrepo.IngestRepo(ctx, st, gitrepo.IngestOptions{
		Name: "demo", RemoteURL: remote, RootName: "demo-main",
		AcquisitionMode: "snapshot", Ref: "main", PersistSource: true, CacheRoot: cache,
		IncludePatterns: []string{"**/*.go", "**/*.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.DocumentsIngested < 2 {
		t.Fatalf("ingested=%d report=%+v", res.DocumentsIngested, res)
	}
	if res.ResolvedCommit == "" {
		t.Fatal("missing commit")
	}

	hits, err := st.SearchOpts(ctx, store.SearchOptions{Query: "RegisterHandler", Limit: 5, Roots: []string{"demo-main"}})
	if err != nil || len(hits) == 0 {
		t.Fatalf("search hits=%d err=%v", len(hits), err)
	}
	if !strings.HasPrefix(hits[0].URI, "git://demo-main/") {
		t.Fatalf("uri=%s", hits[0].URI)
	}

	rs, err := st.GetRepoSourceByName(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if rs.ResolvedCommitSHA != res.ResolvedCommit {
		t.Fatalf("source commit mismatch")
	}
	files, err := st.ListRepoFiles(ctx, rs.ID)
	if err != nil || len(files) < 2 {
		t.Fatalf("files=%d err=%v", len(files), err)
	}
	for _, f := range files {
		if strings.Contains(f.RelativePath, "vendor") {
			t.Fatalf("vendor should be excluded: %s", f.RelativePath)
		}
	}
}

func TestManagedRefreshAddDelete(t *testing.T) {
	requireGit(t)
	work := filepath.Join(t.TempDir(), "work")
	initRepo(t, work, map[string]string{
		"a.go": "package main\n\nfunc AlphaToken() {}\n",
	})
	remoteBare := filepath.Join(t.TempDir(), "remote.git")
	remote := bareClone(t, work, remoteBare)

	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	cache := filepath.Join(t.TempDir(), "cache")

	_, err = gitrepo.IngestRepo(ctx, st, gitrepo.IngestOptions{
		Name: "trk", RemoteURL: remote, RootName: "trk",
		AcquisitionMode: "managed_clone", Ref: "main", PersistSource: true, CacheRoot: cache,
		IncludePatterns: []string{"**/*.go"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// push new commit to bare remote
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0",
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@e.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(work, "b.go"), []byte("package main\n\nfunc BetaToken() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "add", "b.go")
	run(work, "commit", "-m", "add b")
	run(work, "rm", "a.go")
	run(work, "commit", "-m", "rm a")
	run(work, "remote", "add", "pub", remote)
	run(work, "push", "pub", "main")

	rep, err := gitrepo.RefreshRepoSource(ctx, st, "trk", cache, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != "refreshed" && rep.Status != "ingested" {
		t.Fatalf("status=%s", rep.Status)
	}

	hits, _ := st.SearchOpts(ctx, store.SearchOptions{Query: "BetaToken", Limit: 5, Roots: []string{"trk"}})
	if len(hits) == 0 {
		t.Fatal("expected BetaToken")
	}
	hitsOld, _ := st.SearchOpts(ctx, store.SearchOptions{Query: "AlphaToken", Limit: 5, Roots: []string{"trk"}})
	if len(hitsOld) != 0 {
		t.Fatal("AlphaToken should be deleted")
	}

	// unchanged refresh
	rep2, err := gitrepo.RefreshRepoSource(ctx, st, "trk", cache, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Status != "unchanged" {
		t.Fatalf("want unchanged got %s", rep2.Status)
	}
}

func TestLocalCheckoutWorkingTree(t *testing.T) {
	requireGit(t)
	dir := initRepo(t, filepath.Join(t.TempDir(), "local"), map[string]string{
		"main.go": "package main\n\nfunc CommittedFn() {}\n",
	})
	// dirty file
	if err := os.WriteFile(filepath.Join(dir, "dirty.go"), []byte("package main\n\nfunc DirtyFn() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	res, err := gitrepo.IngestRepo(context.Background(), st, gitrepo.IngestOptions{
		Name: "loc", LocalPath: dir, RootName: "loc",
		AcquisitionMode: "local_checkout", WorkingTreeMode: "working_tree",
		PersistSource: true, IncludePatterns: []string{"**/*.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.WorkingTreeDirty {
		t.Fatal("expected dirty flag")
	}
	hits, _ := st.SearchOpts(context.Background(), store.SearchOptions{Query: "DirtyFn", Limit: 5, Roots: []string{"loc"}})
	if len(hits) == 0 {
		t.Fatal("expected dirty file indexed")
	}
}

func TestFailedRefreshKeepsPrior(t *testing.T) {
	requireGit(t)
	src := initRepo(t, filepath.Join(t.TempDir(), "src"), map[string]string{
		"ok.go": "package main\n\nfunc KeepMe() {}\n",
	})
	remote := bareClone(t, src, filepath.Join(t.TempDir(), "r.git"))
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cache := filepath.Join(t.TempDir(), "cache")
	ctx := context.Background()
	res, err := gitrepo.IngestRepo(ctx, st, gitrepo.IngestOptions{
		Name: "keep", RemoteURL: remote, RootName: "keep",
		AcquisitionMode: "managed_clone", Ref: "main", PersistSource: true, CacheRoot: cache,
		IncludePatterns: []string{"**/*.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sha := res.ResolvedCommit

	// Break remote URL
	rs, _ := st.GetRepoSourceByName(ctx, "keep")
	rs.RemoteURL = "file:///nonexistent/path/nope.git"
	_, _ = st.UpsertRepoSource(ctx, *rs)

	_, err = gitrepo.RefreshRepoSource(ctx, st, "keep", cache, nil)
	if err == nil {
		t.Fatal("expected refresh failure")
	}
	rs2, err := st.GetRepoSourceByName(ctx, "keep")
	if err != nil {
		t.Fatal(err)
	}
	if rs2.ResolvedCommitSHA != sha {
		t.Fatalf("commit advanced on failure: %s vs %s", rs2.ResolvedCommitSHA, sha)
	}
	hits, _ := st.SearchOpts(ctx, store.SearchOptions{Query: "KeepMe", Limit: 5, Roots: []string{"keep"}})
	if len(hits) == 0 {
		t.Fatal("prior index should remain")
	}
}

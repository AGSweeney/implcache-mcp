// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import "testing"

func TestNormalizeGitHubHTML(t *testing.T) {
	n, err := NormalizeRemoteURL("https://github.com/example-org/example-sdk/tree/main/docs")
	if err != nil {
		t.Fatal(err)
	}
	if !n.IsHTMLPage {
		t.Fatal("expected HTML page flag")
	}
	if n.CloneURL != "https://github.com/example-org/example-sdk.git" {
		t.Fatalf("clone=%s", n.CloneURL)
	}
	if n.Owner != "example-org" || n.Repository != "example-sdk" {
		t.Fatalf("identity=%s/%s", n.Owner, n.Repository)
	}
}

func TestLooksLikeGitRepoURL(t *testing.T) {
	if !LooksLikeGitRepoURL("https://github.com/foo/bar") {
		t.Fatal("expected github repo")
	}
	if !LooksLikeGitRepoURL("https://github.com/foo/bar.git") {
		t.Fatal("expected .git")
	}
	if LooksLikeGitRepoURL("https://docs.espressif.com/projects/esp-idf/en/stable/esp32/") {
		t.Fatal("docs site should not look like git repo")
	}
}

func TestMatchGlob(t *testing.T) {
	if !MatchGlob("**/*.go", "cmd/app/main.go") {
		t.Fatal("expected match")
	}
	if PathAllowed("vendor/lib/x.go", nil, nil) {
		t.Fatal("vendor should be excluded by default")
	}
	if !PathAllowed("vendor/lib/x.go", []string{"vendor/**"}, []string{}) {
		t.Fatal("explicit include should allow vendor")
	}
}

func TestClassifyPath(t *testing.T) {
	if ClassifyPath("include/api.h") != "public_header" {
		t.Fatal(ClassifyPath("include/api.h"))
	}
	if ClassifyPath("examples/demo/main.go") != "example" {
		t.Fatal(ClassifyPath("examples/demo/main.go"))
	}
}

func TestRedactSecrets(t *testing.T) {
	s := redactSecrets("https://user:token@github.com/a/b.git")
	if s != "https://***@github.com/a/b.git" {
		t.Fatalf("got %s", s)
	}
}

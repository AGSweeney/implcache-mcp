// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package vomit

import (
	"path/filepath"
	"testing"
)

func TestResolveVomitOutPathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveVomitOutPath(root, filepath.Join("..", "x.md"), "subj"); err == nil {
		t.Fatal("expected escape rejection")
	}
	got, err := resolveVomitOutPath(root, "ok.md", "subj")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(got) != root {
		t.Fatalf("got %q", got)
	}
}

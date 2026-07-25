// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifest(t *testing.T) {
	m, err := Parse([]byte(`
rootName: example-control-app
technology:
  - Example Device SDK
languages:
  - cpp
authority: current_project
relatedRoots:
  - example-device-sdk
versions:
  device_sdk: "3.x"
`))
	if err != nil {
		t.Fatal(err)
	}
	roots := m.PreferredRoots()
	if len(roots) != 2 || roots[0] != "example-control-app" {
		t.Fatalf("preferred=%v", roots)
	}
}

func TestInvalidManifest(t *testing.T) {
	if _, err := Parse([]byte(`technology: [x]`)); err == nil {
		t.Fatal("expected rootName required")
	}
	if _, err := Parse([]byte(`rootName: a/b`)); err == nil {
		t.Fatal("expected path separator rejection")
	}
}

func TestMissingManifestOK(t *testing.T) {
	m, err := LoadFromDir(t.TempDir())
	if err != nil || m != nil {
		t.Fatalf("got %#v err=%v", m, err)
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultFilename)
	if err := os.WriteFile(path, []byte("rootName: demo-embedded-project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadFromDir(dir)
	if err != nil || m == nil || m.RootName != "demo-embedded-project" {
		t.Fatalf("got %#v err=%v", m, err)
	}
}

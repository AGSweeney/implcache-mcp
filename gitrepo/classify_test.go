// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import "testing"

func TestClassifyPathClasses(t *testing.T) {
	cases := map[string]string{
		"store/schema.go":           "source",
		"store/schema.sql":          "source",
		"store/schema_test.go":      "test",
		"testdata/pdf/manual.pdf":   "test",
		"docs/DATA_MODEL.md":        "documentation",
		"README.md":                 "documentation",
		"LICENSE":                   "project_meta",
		"NOTICE":                    "project_meta",
		".gitignore":                "project_meta",
		"go.mod":                    "build_file",
		"vendor/lib/x.go":           "third_party",
		"examples/uart/main.c":      "example",
		"include/device.h":          "public_header",
		"config/app.yaml":           "configuration",
	}
	for path, want := range cases {
		if got := ClassifyPath(path); got != want {
			t.Fatalf("%s: got %q want %q", path, got, want)
		}
	}
}

func TestIncludeFromSparse(t *testing.T) {
	got := includeFromSparse([]string{"store", "ingest/"})
	want := []string{"store", "store/**", "ingest", "ingest/**"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import "testing"

func TestClassifyPath(t *testing.T) {
	cases := map[string]string{
		"README.md":                                    "",
		"LibraryDocs/INDEX.md":                         ClassIndex,
		"LibraryDocs/project/COMPONENT_INVENTORY.md":   ClassInventory,
		"LibraryDocs/VALIDATION.md":                    ClassValidation,
		"LibraryDocs/libraries/x/README.md":            ClassCuratedLibraryDoc,
		"LibraryDocs/project/notes.md":                 ClassCuratedProjectDoc,
		"LibraryDocs/platform/notes.md":                ClassCuratedPlatformDoc,
		"LibraryDocs/artifacts/patterns/a.md":          ClassCuratedArtifact,
		"LibraryDocs/README.md":                        ClassLibraryDocsOther,
		"LibraryDocs/odd/file.md":                      ClassLibraryDocsOther,
	}
	for path, want := range cases {
		if got := ClassifyPath(path); got != want {
			t.Errorf("ClassifyPath(%q)=%q want %q", path, got, want)
		}
	}
}

func TestNormalizeRepoRelPath(t *testing.T) {
	ok, err := NormalizeRepoRelPath("src/mqtt_client.go")
	if err != nil || ok != "src/mqtt_client.go" {
		t.Fatalf("got %q err=%v", ok, err)
	}
	if _, err := NormalizeRepoRelPath("../etc/passwd"); err == nil {
		t.Fatal("expected traversal reject")
	}
	if _, err := NormalizeRepoRelPath("/abs"); err == nil {
		t.Fatal("expected absolute reject")
	}
	clean, warns := NormalizeSourcePaths([]string{"src/ok.go", "../bad", "src/ok.go"})
	if len(clean) != 1 || clean[0] != "src/ok.go" {
		t.Fatalf("clean=%v", clean)
	}
	if len(warns) == 0 {
		t.Fatal("expected warning for bad path")
	}
}

func TestMapTrust(t *testing.T) {
	td := MapTrust(StateValidated, DocMeta{
		LibraryDocs: true, Status: "verified", EvidenceLevel: "E1",
		ContentClass: ClassCuratedLibraryDoc,
	})
	if td.Authority != "curated_internal_recipe" || td.Deprecated {
		t.Fatalf("%+v", td)
	}
	td = MapTrust(StateValidated, DocMeta{LibraryDocs: true, Status: "deprecated"})
	if !td.Deprecated {
		t.Fatal("expected deprecated")
	}
	td = MapTrust(StateInvalid, DocMeta{
		LibraryDocs: true, Status: "verified", EvidenceLevel: "E1",
	})
	if td.Authority == "curated_internal_recipe" {
		t.Fatal("invalid package must not get curated boost")
	}
}

package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRootsInfersAndPrompts(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	for _, root := range []string{"ccw_help", "creo_toolkit_help", "otk_cpp_doc"} {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI:        "project://" + root + "/doc.md",
			Title:      root,
			SourceType: SourceMarkdown,
			Path:       "doc.md",
			RootName:   root,
			Hash:       root + "-h",
			Chunks:     []Chunk{{Body: "sample body about " + root, StartLine: 1, EndLine: 2}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	inf, err := st.ResolveRoots(ctx, "Micro800 download project", nil)
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || len(inf.Roots) != 1 || inf.Roots[0] != "ccw_help" {
		t.Fatalf("ccw infer: %+v", inf)
	}

	inf, err = st.ResolveRoots(ctx, "user_initialize menubar pushbutton", nil)
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || !contains(inf.Roots, "creo_toolkit_help") {
		t.Fatalf("creo infer: %+v", inf)
	}

	inf, err = st.ResolveRoots(ctx, "create a project", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !inf.NeedsChoice {
		t.Fatalf("expected prompt for ambiguous query, got roots=%v", inf.Roots)
	}
	if !strings.Contains(inf.Message, "rootName") || !strings.Contains(inf.Message, "ccw_help") {
		t.Fatalf("prompt missing roots: %s", inf.Message)
	}

	inf, err = st.ResolveRoots(ctx, "anything", []string{"ccw_help"})
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || inf.Roots[0] != "ccw_help" {
		t.Fatalf("explicit: %+v", inf)
	}
}

func TestSearchOptsFiltersRoot(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://ccw_help/a.md", Title: "ccw", SourceType: SourceMarkdown,
		Path: "a.md", RootName: "ccw_help", Hash: "1",
		Chunks: []Chunk{{Body: "download the controller project", StartLine: 1, EndLine: 1}},
	})
	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://creo_toolkit_help/b.md", Title: "creo", SourceType: SourceMarkdown,
		Path: "b.md", RootName: "creo_toolkit_help", Hash: "2",
		Chunks: []Chunk{{Body: "download the feature project", StartLine: 1, EndLine: 1}},
	})

	hits, err := st.SearchOpts(ctx, SearchOptions{Query: "download project", Limit: 10, Roots: []string{"ccw_help"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	for _, h := range hits {
		if !strings.Contains(h.URI, "ccw_help") {
			t.Fatalf("leaked non-ccw hit: %s", h.URI)
		}
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

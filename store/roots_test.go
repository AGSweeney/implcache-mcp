// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

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

	for _, root := range []string{"example-device-sdk", "example-plugin-sdk", "example-network-sdk"} {
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

	inf, err := st.ResolveRoots(ctx, "gpio expander SpiTransfer", nil)
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || len(inf.Roots) == 0 || !contains(inf.Roots, "example-device-sdk") {
		t.Fatalf("device infer: %+v", inf)
	}

	inf, err = st.ResolveRoots(ctx, "RegisterCommand AddMenuItem", nil)
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || !contains(inf.Roots, "example-plugin-sdk") {
		t.Fatalf("plugin infer: %+v", inf)
	}

	inf, err = st.ResolveRoots(ctx, "create a project", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !inf.NeedsChoice {
		t.Fatalf("expected prompt for ambiguous query, got roots=%v", inf.Roots)
	}
	if !strings.Contains(inf.Message, "rootName") || !strings.Contains(inf.Message, "example-device-sdk") {
		t.Fatalf("prompt missing roots: %s", inf.Message)
	}

	inf, err = st.ResolveRoots(ctx, "anything", []string{"example-device-sdk"})
	if err != nil {
		t.Fatal(err)
	}
	if inf.NeedsChoice || inf.Roots[0] != "example-device-sdk" {
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
		URI: "project://example-device-sdk/a.md", Title: "device", SourceType: SourceMarkdown,
		Path: "a.md", RootName: "example-device-sdk", Hash: "1",
		Chunks: []Chunk{{Body: "download the controller project", StartLine: 1, EndLine: 1}},
	})
	_, _ = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-plugin-sdk/b.md", Title: "plugin", SourceType: SourceMarkdown,
		Path: "b.md", RootName: "example-plugin-sdk", Hash: "2",
		Chunks: []Chunk{{Body: "download the feature project", StartLine: 1, EndLine: 1}},
	})

	hits, err := st.SearchOpts(ctx, SearchOptions{Query: "download project", Limit: 10, Roots: []string{"example-device-sdk"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	for _, h := range hits {
		if !strings.Contains(h.URI, "example-device-sdk") {
			t.Fatalf("leaked non-device hit: %s", h.URI)
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

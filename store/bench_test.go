// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkSearchWarm(b *testing.B) {
	st, cleanup := benchStore(b, 2000)
	defer cleanup()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := st.SearchOpts(ctx, SearchOptions{
			Query: "topic alpha document",
			Limit: 20,
			Roots: []string{"bench"},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIngestUnchanged(b *testing.B) {
	dir := b.TempDir()
	st, err := Open(filepath.Join(dir, "b.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	in := UpsertInput{
		URI: "project://bench/x.md", Title: "x", SourceType: SourceMarkdown,
		Path: "x.md", RootName: "bench", Hash: "fixed-hash",
		Chunks: []Chunk{{Body: "unchanged body topic alpha", StartLine: 1, EndLine: 1}},
	}
	if _, err := st.UpsertDocument(ctx, in); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		written, err := st.UpsertDocument(ctx, in)
		if err != nil {
			b.Fatal(err)
		}
		if written {
			b.Fatal("expected skip")
		}
	}
}

func benchStore(b *testing.B, n int) (*Store, func()) {
	b.Helper()
	dir := b.TempDir()
	st, err := Open(filepath.Join(dir, "b.db"))
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < n; i++ {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI:        fmt.Sprintf("project://bench/doc-%d.md", i),
			Title:      fmt.Sprintf("doc-%d", i),
			SourceType: SourceMarkdown,
			Path:       fmt.Sprintf("doc-%d.md", i),
			RootName:   "bench",
			Hash:       fmt.Sprintf("h-%d", i),
			Chunks: []Chunk{{
				Body:      fmt.Sprintf("topic alpha document number %d with unique token tok%d", i, i),
				StartLine: 1,
				EndLine:   1,
			}},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	return st, func() { _ = st.Close() }
}

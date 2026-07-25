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

func BenchmarkSemanticPostingLookup(b *testing.B) {
	dir := b.TempDir()
	st, err := Open(filepath.Join(dir, "semantic.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for i := 0; i < 250; i++ {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI:   fmt.Sprintf("project://example-network-sdk/doc-%03d.md", i),
			Title: "Network guide", SourceType: SourceMarkdown,
			Path: fmt.Sprintf("doc-%03d.md", i), RootName: "example-network-sdk",
			Authority: AuthorityOfficialDocs, Hash: fmt.Sprintf("h-%d", i),
			Chunks: []Chunk{{Body: "network retry reconnect exponential backoff retry policy connection recovery", StartLine: 1, EndLine: 2}},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.SearchOpts(ctx, SearchOptions{
			Query: "network retry reconnect", Limit: 20,
			Roots: []string{"example-network-sdk"}, Semantic: true,
		}); err != nil {
			b.Fatal(err)
		}
	}
}

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
	st := seedSemanticBenchCorpus(b, 250)
	defer st.Close()
	ctx := context.Background()
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

// BenchmarkSemanticMultiTermLookup stresses IDF lookup + capped posting IN-lists
// with a long query against a larger decoy-heavy corpus.
func BenchmarkSemanticMultiTermLookup(b *testing.B) {
	st := seedSemanticBenchCorpus(b, 800)
	defer st.Close()
	ctx := context.Background()
	query := "network retry reconnect exponential backoff connection recovery RetryPolicy client session timeout handshake"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.semanticCandidates(ctx, query, []string{"example-network-sdk"}, 20); err != nil {
			b.Fatal(err)
		}
	}
}

func seedSemanticBenchCorpus(b *testing.B, n int) *Store {
	b.Helper()
	dir := b.TempDir()
	st, err := Open(filepath.Join(dir, "semantic.db"))
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < n; i++ {
		body := "network retry reconnect exponential backoff retry policy connection recovery"
		if i%7 == 0 {
			body = "commonterm unrelated deployment guide configuration session " + fmt.Sprintf("%d", i)
		}
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI:   fmt.Sprintf("project://example-network-sdk/doc-%03d.md", i),
			Title: "Network guide", SourceType: SourceMarkdown,
			Path: fmt.Sprintf("doc-%03d.md", i), RootName: "example-network-sdk",
			Authority: AuthorityOfficialDocs, Hash: fmt.Sprintf("h-%d", i),
			Chunks: []Chunk{{Body: body, StartLine: 1, EndLine: 2}},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	return st
}

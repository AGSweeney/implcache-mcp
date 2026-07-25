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

func TestBuildTermVectorStable(t *testing.T) {
	a := BuildTermVector("Reconnect", "RetryPolicy backoff Connect Disconnect network client")
	b := BuildTermVector("Reconnect", "RetryPolicy backoff Connect Disconnect network client")
	if a == "" || a != b {
		t.Fatalf("unstable vector: %q vs %q", a, b)
	}
	if !strings.Contains(a, "retrypolicy") && !strings.Contains(a, "reconnect") {
		t.Fatalf("expected key terms in %q", a)
	}
}

func TestSemanticSearchFindsRelatedChunk(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-network-sdk/retry.md", Title: "Retry guide",
		SourceType: SourceMarkdown, Path: "retry.md", RootName: "example-network-sdk",
		Authority: AuthorityOfficialDocs, Hash: "s1",
		Chunks: []Chunk{{
			Heading:   "Backoff",
			Body:      "Use RetryPolicy with exponential backoff when Connect fails. Call Disconnect before reconnect.",
			StartLine: 1, EndLine: 4,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-network-sdk/unrelated.md", Title: "Logging",
		SourceType: SourceMarkdown, Path: "unrelated.md", RootName: "example-network-sdk",
		Authority: AuthorityOfficialDocs, Hash: "s2",
		Chunks: []Chunk{{
			Heading:   "Logging",
			Body:      "Configure SetLogLevel and AttachSink for structured logs.",
			StartLine: 1, EndLine: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Query uses related wording that may be weak for exact FTS AND.
	hits, err := st.SearchOpts(ctx, SearchOptions{
		Query: "network reconnect backoff policy", Limit: 10,
		Roots: []string{"example-network-sdk"}, Semantic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected semantic/FTS hits")
	}
	foundRetry := false
	for _, h := range hits {
		if strings.Contains(h.URI, "retry.md") {
			foundRetry = true
			break
		}
	}
	if !foundRetry {
		t.Fatalf("expected retry.md among hits: %+v", hits)
	}
}

func TestSemanticOffByDefault(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/a.md", Title: "A", SourceType: SourceMarkdown, Path: "a.md",
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "a",
		Chunks: []Chunk{{Body: "RetryPolicy Connect Disconnect", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.SearchOpts(ctx, SearchOptions{Query: "RetryPolicy", Limit: 5, Roots: []string{"sdk"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.MatchKind == MatchKindSemantic {
			t.Fatal("semantic hits must not appear when Semantic=false")
		}
	}
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"strconv"
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

func TestTokenizeSemanticIdentifiers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"NetworkClient", []string{"networkclient", "network", "client"}},
		{"RegisterCommand", []string{"registercommand", "register", "command"}},
		{"RetryPolicy", []string{"retrypolicy", "retry", "policy"}},
		{"network_client", []string{"network_client", "network", "client"}},
		{"namespace::RegisterHandler", []string{"namespace", "registerhandler", "register", "handler"}},
		{"Client.Connect", []string{"client", "connect"}},
		{"HTTPServer", []string{"httpserver", "http", "server"}},
		{"reconnect", []string{"reconnect"}},
		// Tiny components and stopwords are dropped; combined token survives.
		{"toB", []string{"tob"}},
		{"the and for", nil},
	}
	for _, tc := range cases {
		got := tokenizeSemantic(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("tokenizeSemantic(%q)=%v want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("tokenizeSemantic(%q)=%v want %v", tc.in, got, tc.want)
			}
		}
		// Deterministic across calls.
		again := tokenizeSemantic(tc.in)
		if strings.Join(again, " ") != strings.Join(got, " ") {
			t.Fatalf("tokenizeSemantic(%q) unstable: %v vs %v", tc.in, got, again)
		}
	}
}

func TestIDFWeightDownranksCommonTerms(t *testing.T) {
	if idfWeight(1000, 900) >= idfWeight(1000, 2) {
		t.Fatal("rare terms must outrank common terms")
	}
	if idfWeight(100, 0) <= 1 {
		t.Fatal("unseen terms need weight above the presence floor")
	}
}

func TestSemanticIDFPrefersRareMatchOverCommonNoise(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "idf.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for i := 0; i < 40; i++ {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI: "project://sdk/noise-" + strconv.Itoa(i) + ".md", Title: "Noise",
			SourceType: SourceMarkdown, RootName: "sdk", Authority: AuthorityOfficialDocs,
			Hash:   "noise-" + strconv.Itoa(i),
			Chunks: []Chunk{{Body: "network client application deployment guide " + strconv.Itoa(i)}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/retry.md", Title: "Retry", SourceType: SourceMarkdown,
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "retry",
		Chunks: []Chunk{{Body: "Reconnect handling uses RetryPolicy and exponential backoff with network client"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.semanticCandidates(ctx, "network client RetryPolicy reconnect", []string{"sdk"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || !strings.HasSuffix(hits[0].URI, "/retry.md") {
		t.Fatalf("IDF ranking should prefer rare RetryPolicy doc, got %+v", hits)
	}
}

func TestSemanticStatsBoundedPerChunk(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/a.md", Title: "A", SourceType: SourceMarkdown,
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "a",
		Chunks: []Chunk{{Body: "RetryPolicy exponential backoff network client reconnect"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := st.SemanticStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Vectors != 1 || stats.Postings == 0 || stats.DistinctTerms == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.AvgPostingsPerVector > float64(maxTermVectorTerms) {
		t.Fatalf("avg postings per vector %.1f exceeds cap %d", stats.AvgPostingsPerVector, maxTermVectorTerms)
	}
}

func TestSelectLookupTermsCapsByIDF(t *testing.T) {
	terms := []string{"alpha", "beta", "common", "delta"}
	idf := map[string]float64{"alpha": 3, "beta": 2, "common": 1.1, "delta": 4}
	got := selectLookupTerms(terms, idf, 2)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "delta" {
		t.Fatalf("selectLookupTerms=%v want [alpha delta]", got)
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

func TestSemanticSearchEndToEndRankingLimitAndRoot(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "semantic-e2e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	docs := []UpsertInput{
		{
			URI: "project://example-network-sdk/reconnect.md", Title: "Reconnect",
			SourceType: SourceMarkdown, Path: "reconnect.md", RootName: "example-network-sdk",
			Authority: AuthorityOfficialDocs, Hash: "network",
			Chunks: []Chunk{{Body: "Reconnect handling uses exponential backoff and resets retry state after network recovery.", StartLine: 1, EndLine: 2}},
		},
		{
			URI: "project://example-network-sdk/migration.md", Title: "Migrations",
			SourceType: SourceMarkdown, Path: "migration.md", RootName: "example-network-sdk",
			Authority: AuthorityOfficialDocs, Hash: "migration",
			Chunks: []Chunk{{Body: "Database migration uses PRAGMA user_version and transactions.", StartLine: 1, EndLine: 2}},
		},
		{
			URI: "project://other-sdk/plugin.md", Title: "Plugin",
			SourceType: SourceMarkdown, Path: "plugin.md", RootName: "other-sdk",
			Authority: AuthorityOfficialDocs, Hash: "plugin",
			Chunks: []Chunk{{Body: "Plugin menu command registration and callback setup.", StartLine: 1, EndLine: 2}},
		},
	}
	for _, doc := range docs {
		if _, err := st.UpsertDocument(ctx, doc); err != nil {
			t.Fatal(err)
		}
	}

	enabled, err := st.SearchOpts(ctx, SearchOptions{
		Query: "network retry reconnect", Limit: 1, Roots: []string{"example-network-sdk"}, Semantic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(enabled) != 1 || !strings.HasSuffix(enabled[0].URI, "/reconnect.md") {
		t.Fatalf("semantic ranking/limit got %+v", enabled)
	}
	if enabled[0].RootName != "example-network-sdk" {
		t.Fatalf("root leaked: %+v", enabled[0])
	}

	disabled, err := st.SearchOpts(ctx, SearchOptions{
		Query: "network retry reconnect", Limit: 10, Roots: []string{"example-network-sdk"}, Semantic: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range disabled {
		if hit.MatchKind == MatchKindSemantic {
			t.Fatalf("semantic-disabled result: %+v", hit)
		}
		if hit.RootName != "example-network-sdk" {
			t.Fatalf("disabled root leaked: %+v", hit)
		}
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

func TestTermVectorsFollowDocumentLifecycle(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "vectors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	uri := "project://example-network-sdk/reconnect.md"

	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: uri, Title: "Reconnect Handling",
		SourceType: SourceMarkdown, Path: "reconnect.md", RootName: "example-network-sdk",
		Authority: AuthorityOfficialDocs, Hash: "v1",
		Chunks: []Chunk{
			{Heading: "Reconnect Handling", Body: "A network client reconnects with bounded exponential backoff.", StartLine: 1, EndLine: 3},
			{Heading: "Retry Policy", Body: "Use RetryPolicy and reset the retry counter after success.", StartLine: 4, EndLine: 6},
			{Heading: "", Body: "", StartLine: 7, EndLine: 7},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, err := st.TermVectorCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("term vectors=%d want 3", count)
	}
	postings, err := st.TermPostingCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if postings == 0 {
		t.Fatal("expected postings for non-empty vectors")
	}
	var matched int
	if err := st.db.QueryRow(`
		SELECT COUNT(*)
		FROM chunks c JOIN chunk_term_vectors v ON v.chunk_id = c.id
		WHERE c.document_id = (SELECT id FROM documents WHERE uri = ?) AND v.terms != ''`, uri).Scan(&matched); err != nil {
		t.Fatal(err)
	}
	if matched != 2 {
		t.Fatalf("meaningful chunks with vectors=%d want 2", matched)
	}

	// Replacement deletes obsolete chunks and their vectors, then writes the
	// vector for the replacement chunk.
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: uri, Title: "Reconnect Handling",
		SourceType: SourceMarkdown, Path: "reconnect.md", RootName: "example-network-sdk",
		Authority: AuthorityOfficialDocs, Hash: "v2",
		Chunks: []Chunk{{Heading: "Reconnect", Body: "Reconnect with RetryPolicy backoff.", StartLine: 1, EndLine: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	count, err = st.TermVectorCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("term vectors after replacement=%d want 1", count)
	}
	postings, err = st.TermPostingCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if postings != len(strings.Fields(BuildTermVector("Reconnect", "Reconnect with RetryPolicy backoff."))) {
		t.Fatalf("postings after replacement=%d", postings)
	}
	deleted, err := st.DeleteDocument(ctx, uri)
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	count, err = st.TermVectorCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("term vectors after delete=%d want 0", count)
	}
	postings, err = st.TermPostingCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if postings != 0 {
		t.Fatalf("postings after delete=%d want 0", postings)
	}
}

func TestSemanticPostingQueryPlan(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "plan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-network-sdk/retry.md", Title: "Retry",
		SourceType: SourceMarkdown, Path: "retry.md", RootName: "example-network-sdk",
		Authority: AuthorityOfficialDocs, Hash: "plan",
		Chunks: []Chunk{{Body: "network retry reconnect exponential backoff", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPostingPlanUsesIndex(t, st, `
		EXPLAIN QUERY PLAN
		SELECT p.chunk_id FROM chunk_term_postings p
		WHERE p.root_name = ? AND p.term IN (?)`,
		"example-network-sdk", "reconnect")
}

func TestSemanticProductionCandidateQueryPlan(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "prod-plan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-network-sdk/retry.md", Title: "Retry",
		SourceType: SourceMarkdown, Path: "retry.md", RootName: "example-network-sdk",
		Authority: AuthorityOfficialDocs, Hash: "prod-plan",
		Chunks: []Chunk{{Body: "network retry reconnect exponential backoff", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Mirror the production candidate subquery shape (GROUP BY / ORDER BY / LIMIT).
	assertPostingPlanUsesIndex(t, st, `
		EXPLAIN QUERY PLAN
		SELECT p.chunk_id
		FROM chunk_term_postings p
		WHERE p.term IN (?, ?)
		  AND p.root_name IN (?)
		GROUP BY p.chunk_id
		ORDER BY COUNT(*) DESC, p.chunk_id
		LIMIT ?`,
		"reconnect", "retry", "example-network-sdk", 1000)
}

func assertPostingPlanUsesIndex(t *testing.T, st *Store, sqlText string, args ...any) {
	t.Helper()
	rows, err := st.db.QueryContext(context.Background(), sqlText, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(details) == 0 {
		t.Fatal("missing semantic query plan")
	}
	for _, detail := range details {
		if strings.Contains(detail, "idx_chunk_term_postings_root_term") {
			return
		}
	}
	t.Fatalf("posting lookup did not use root/term index: %v", details)
}

func TestSemanticPostingCandidatesHandleManyTermsAndDecoys(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "candidates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for i := 0; i < 250; i++ {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI: "project://sdk/decoy-" + strconv.Itoa(i) + ".md", Title: "Decoy",
			SourceType: SourceMarkdown, RootName: "sdk", Authority: AuthorityOfficialDocs,
			Hash:   "decoy-" + strconv.Itoa(i),
			Chunks: []Chunk{{Body: "commonterm unrelated detail " + strconv.Itoa(i)}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/target.md", Title: "Target", SourceType: SourceMarkdown,
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "target",
		Chunks: []Chunk{{Body: "commonterm alphaone betatwo gammathree deltafour epsilfive zetasix eta seven"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.semanticCandidates(ctx,
		"commonterm alphaone betatwo gammathree deltafour epsilfive zetasix eta seven",
		[]string{"sdk"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.HasSuffix(hits[0].URI, "/target.md") {
		t.Fatalf("many-term candidate ranking got %+v", hits)
	}
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
)

func TestTermDFMaintainedOnIngestReplaceDelete(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "df.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	uri := "project://sdk/retry.md"
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: uri, Title: "Retry", SourceType: SourceMarkdown, RootName: "sdk",
		Authority: AuthorityOfficialDocs, Hash: "1",
		Chunks: []Chunk{{Body: "RetryPolicy exponential backoff network"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRootChunks(t, st, "sdk", 1)
	assertTermDF(t, st, "sdk", "retrypolicy", 1)

	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: uri, Title: "Retry", SourceType: SourceMarkdown, RootName: "sdk",
		Authority: AuthorityOfficialDocs, Hash: "2",
		Chunks: []Chunk{{Body: "RetryPolicy bounded linear backoff"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRootChunks(t, st, "sdk", 1)
	assertTermDF(t, st, "sdk", "retrypolicy", 1)
	assertTermDF(t, st, "sdk", "exponential", 0) // obsolete term removed
	assertTermDF(t, st, "sdk", "bounded", 1)

	ok, err := st.DeleteDocument(ctx, uri)
	if err != nil || !ok {
		t.Fatalf("delete: ok=%v err=%v", ok, err)
	}
	assertRootChunks(t, st, "sdk", 0)
	assertTermDF(t, st, "sdk", "retrypolicy", 0)
}

func TestPersistedDFUsedForIDF(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "idf-df.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		_, err := st.UpsertDocument(ctx, UpsertInput{
			URI: "project://sdk/n-" + strconv.Itoa(i) + ".md", Title: "n",
			SourceType: SourceMarkdown, RootName: "sdk", Authority: AuthorityOfficialDocs,
			Hash:   "h" + strconv.Itoa(i),
			Chunks: []Chunk{{Body: "network client guide " + strconv.Itoa(i)}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/target.md", Title: "t", SourceType: SourceMarkdown,
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "target",
		Chunks: []Chunk{{Body: "RetryPolicy reconnect network client"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	n, df, err := st.termDocumentFrequencies(ctx, []string{"network", "retrypolicy"}, []string{"sdk"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 21 {
		t.Fatalf("nChunks=%d want 21", n)
	}
	if df["network"] < 20 || df["retrypolicy"] != 1 {
		t.Fatalf("df=%v", df)
	}
}

func assertRootChunks(t *testing.T, st *Store, root string, want int) {
	t.Helper()
	var got int
	err := st.db.QueryRow(`SELECT COALESCE(SUM(chunk_count),0) FROM root_chunk_stats WHERE root_name = ?`, root).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("root_chunk_stats[%s]=%d want %d", root, got, want)
	}
}

func assertTermDF(t *testing.T, st *Store, root, term string, want int) {
	t.Helper()
	var got int
	err := st.db.QueryRow(`SELECT COALESCE(df,0) FROM term_df WHERE root_name = ? AND term = ?`, root, term).Scan(&got)
	if err != nil {
		if want == 0 {
			return
		}
		t.Fatalf("term_df[%s/%s]: %v", root, term, err)
	}
	if got != want {
		t.Fatalf("term_df[%s/%s]=%d want %d", root, term, got, want)
	}
}

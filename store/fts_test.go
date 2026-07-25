// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import "testing"

func TestToFTSQuerySafeLiterals(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"C++", `"C++"`},
		{"C#", `"C#"`},
		{`foo::bar`, `"foo::bar"`},
		{"register_command()", `"register_command()"`},
		{`quoted "text"`, `"quoted" AND "text"`},
		{"OR AND NOT", ""},
		{"hyphenated-identifier", `"hyphenated-identifier"`},
		{"snake_case_identifier", `"snake_case_identifier"`},
		{"path/to/file", `"path/to/file"`},
		{"*", ""},
		{"user initialize", `"user" AND "initialize"`},
	}
	for _, tc := range cases {
		got := toFTSQuery(tc.in)
		if got != tc.want {
			t.Fatalf("toFTSQuery(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSearchPunctuationQueries(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := t.Context()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://demo/a.md", Title: "a", SourceType: SourceMarkdown,
		Path: "a.md", RootName: "demo", Hash: "1",
		Chunks: []Chunk{{Body: "Use C++ with foo::bar and register_command() here", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"C++", "foo::bar", "register_command()"} {
		hits, err := st.SearchOpts(ctx, SearchOptions{Query: q, Limit: 5, Roots: []string{"demo"}})
		if err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		if len(hits) == 0 {
			t.Fatalf("query %q: no hits", q)
		}
	}
}

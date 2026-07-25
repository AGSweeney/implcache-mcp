// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNormalizeAndForms(t *testing.T) {
	cases := []struct {
		in, norm, unqual, ns string
	}{
		{"RegisterHandler()", "registerhandler", "RegisterHandler", ""},
		{"demo::RegisterHandler", "demo::registerhandler", "RegisterHandler", "demo"},
		{"Client.Connect", "client.connect", "Connect", "Client"},
		{"initialize_device", "initialize_device", "initialize_device", ""},
		{"CONFIG_MAX_RETRIES", "config_max_retries", "CONFIG_MAX_RETRIES", ""},
		{"template<Type>", "template<type>", "template<Type>", ""},
	}
	for _, tc := range cases {
		forms := DeriveSymbolForms(tc.in, "")
		if forms.NameNorm != tc.norm {
			t.Fatalf("%s norm=%q want %q", tc.in, forms.NameNorm, tc.norm)
		}
		if forms.UnqualifiedName != tc.unqual {
			t.Fatalf("%s unqual=%q want %q", tc.in, forms.UnqualifiedName, tc.unqual)
		}
		if forms.Namespace != tc.ns {
			t.Fatalf("%s ns=%q want %q", tc.in, forms.Namespace, tc.ns)
		}
	}
}

func TestFindSymbolsStagedMatches(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-plugin-sdk/src/commands.cpp", Title: "commands",
		SourceType: SourceSource, Path: "src/commands.cpp", RootName: "example-plugin-sdk",
		Authority: AuthorityOfficialDocs, Hash: "h1",
		Chunks: []Chunk{{Heading: "RegisterCommand", Body: "void demo::RegisterCommand() {}", StartLine: 1, EndLine: 3}},
		Symbols: []SymbolInput{
			{Name: "demo::RegisterCommand", Kind: "function", Language: "cpp", Signature: "void demo::RegisterCommand()", StartLine: 1, EndLine: 3},
			{Name: "AddMenuItem", Kind: "function", Language: "cpp", StartLine: 10, EndLine: 12},
			{Name: "Client.Connect", Kind: "function", Language: "cpp", StartLine: 20, EndLine: 22},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	check := func(query, wantMatch string) {
		t.Helper()
		syms, err := st.FindSymbols(ctx, query, []string{"example-plugin-sdk"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(syms) == 0 {
			t.Fatalf("%s: no hits", query)
		}
		if syms[0].MatchType != wantMatch && !(wantMatch == MatchExactCanonical && syms[0].MatchType == MatchExactNormalized) {
			// Accept exact or exact_normalized for bare names
			if wantMatch == MatchExactCanonical && syms[0].MatchType == MatchExactNormalized {
				return
			}
			if wantMatch == MatchExactUnqualified && (syms[0].MatchType == MatchExactNormalized || syms[0].MatchType == MatchExactUnqualified) {
				return
			}
			t.Fatalf("%s: matchType=%s want %s (name=%s)", query, syms[0].MatchType, wantMatch, syms[0].Name)
		}
		if syms[0].Confidence <= 0 {
			t.Fatalf("missing confidence")
		}
	}

	check("AddMenuItem", MatchExactCanonical)
	check("RegisterCommand", MatchExactUnqualified)
	check("demo::RegisterCommand", MatchExactCanonical)
	check("AddMenu", MatchPrefix)
	check("Connect", MatchExactUnqualified)
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
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
			{Name: "InitializeDeviceContext", Kind: "function", Language: "cpp", StartLine: 30, EndLine: 32},
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
		got := syms[0].MatchType
		ok := got == wantMatch
		// Normalized trailing parentheses land on exact/exact_normalized.
		if wantMatch == MatchExactNormalized && (got == MatchExactCanonical || got == MatchExactNormalized) {
			ok = true
		}
		if wantMatch == MatchExactCanonical && (got == MatchExactCanonical || got == MatchExactNormalized) {
			ok = true
		}
		if wantMatch == MatchExactUnqualified && (got == MatchExactUnqualified || got == MatchExactNormalized || got == MatchExactCanonical) {
			ok = true
		}
		if !ok {
			t.Fatalf("%s: matchType=%s want %s (name=%s)", query, got, wantMatch, syms[0].Name)
		}
		if syms[0].Confidence <= 0 {
			t.Fatalf("missing confidence")
		}
	}

	check("AddMenuItem", MatchExactCanonical)
	check("AddMenuItem()", MatchExactNormalized)
	check("RegisterCommand", MatchExactUnqualified)
	check("registercommand", MatchExactUnqualified)
	check("demo::RegisterCommand", MatchExactQualified)
	check("AddMenu", MatchPrefix)
	check("Connect", MatchExactUnqualified)
	check("MenuItem", MatchSuffix)
	// Token stage: needle appears inside a longer name (suffix may also match first).
	symsTok, err := st.FindSymbols(ctx, "DeviceCont", []string{"example-plugin-sdk"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(symsTok) == 0 {
		t.Fatal("token/prefix DeviceCont: no hits")
	}
	foundTok := false
	for _, s := range symsTok {
		if s.MatchType == MatchToken || s.MatchType == MatchPrefix {
			foundTok = true
			break
		}
	}
	if !foundTok {
		t.Fatalf("expected prefix/token match, got %+v", symsTok)
	}

	// Small typo → bounded fuzzy.
	syms, err := st.FindSymbols(ctx, "AddMenuItme", []string{"example-plugin-sdk"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 || syms[0].MatchType != MatchFuzzy {
		t.Fatalf("fuzzy: got %+v", syms)
	}

	// No acceptable match.
	syms, err = st.FindSymbols(ctx, "ZzzNotARealSymbol", []string{"example-plugin-sdk"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 0 {
		t.Fatalf("expected no match, got %+v", syms)
	}
}

func TestFindSymbolsRootFiltering(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for _, root := range []string{"example-plugin-sdk", "other-sdk"} {
		_, err = st.UpsertDocument(ctx, UpsertInput{
			URI: "project://" + root + "/api.cpp", Title: "api",
			SourceType: SourceSource, Path: "api.cpp", RootName: root,
			Authority: AuthorityOfficialDocs, Hash: root,
			Chunks:  []Chunk{{Body: "RegisterHandler", StartLine: 1, EndLine: 1}},
			Symbols: []SymbolInput{{Name: "RegisterHandler", Kind: "function", Language: "cpp", StartLine: 1}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	syms, err := st.FindSymbols(ctx, "RegisterHandler", []string{"example-plugin-sdk"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 || syms[0].RootName != "example-plugin-sdk" {
		t.Fatalf("root filter failed: %+v", syms)
	}
}

func TestFindSymbolsDefinitionOverDeclaration(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-plugin-sdk/src/api.cpp", Title: "api",
		SourceType: SourceSource, Path: "src/api.cpp", RootName: "example-plugin-sdk",
		Authority: AuthorityOfficialDocs, Hash: "h-decl",
		Chunks: []Chunk{{Body: "RegisterHandler", StartLine: 1, EndLine: 5}},
		Symbols: []SymbolInput{
			{Name: "RegisterHandler", Kind: "declaration", Language: "cpp", StartLine: 1, EndLine: 1},
			{Name: "RegisterHandler", Kind: "function", Language: "cpp", StartLine: 3, EndLine: 5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	syms, err := st.FindSymbols(ctx, "RegisterHandler", []string{"example-plugin-sdk"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 || syms[0].Kind != "function" {
		t.Fatalf("definition should beat declaration: %+v", syms)
	}
}

func TestFindSymbolsDefinitionOverCall(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-plugin-sdk/src/both.cpp", Title: "both",
		SourceType: SourceSource, Path: "src/both.cpp", RootName: "example-plugin-sdk",
		Authority: AuthorityOfficialExample, Hash: "h2",
		Chunks: []Chunk{{Body: "RegisterHandler", StartLine: 1, EndLine: 5}},
		Symbols: []SymbolInput{
			{Name: "RegisterHandler", Kind: "call", Language: "cpp", StartLine: 5, EndLine: 5},
			{Name: "RegisterHandler", Kind: "function", Language: "cpp", Signature: "int RegisterHandler()", StartLine: 1, EndLine: 3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	syms, err := st.FindSymbols(ctx, "RegisterHandler", []string{"example-plugin-sdk"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 || syms[0].Kind != "function" {
		t.Fatalf("definition should rank first: %+v", syms)
	}
}

func TestPersistedSymbolDerivedFields(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, UpsertInput{
		URI: "project://example-plugin-sdk/api.h", Title: "api",
		SourceType: SourceSource, Path: "api.h", RootName: "example-plugin-sdk",
		Authority: AuthorityOfficialDocs, Hash: "h3",
		Chunks: []Chunk{{Body: "int demo::RegisterHandler(const char* name);", StartLine: 1, EndLine: 1}},
		Symbols: []SymbolInput{
			{
				Name: "demo::RegisterHandler", Kind: "declaration", Language: "cpp",
				Signature: "int demo::RegisterHandler(const char* name);", StartLine: 1, EndLine: 1,
			},
			{
				Name: "Client::Connect", Kind: "declaration", Language: "cpp",
				Signature: "Status Client::Connect(const Config& config);", StartLine: 3, EndLine: 3,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var qual, unqual, ns, sigNorm string
	err = db.QueryRow(`
		SELECT qualified_name, unqualified_name, namespace, signature_norm
		FROM symbols WHERE name = 'demo::RegisterHandler'`).Scan(&qual, &unqual, &ns, &sigNorm)
	if err != nil {
		t.Fatal(err)
	}
	if qual != "demo::RegisterHandler" || unqual != "RegisterHandler" || ns != "demo" {
		t.Fatalf("forms qual=%q unqual=%q ns=%q", qual, unqual, ns)
	}
	if sigNorm == "" {
		t.Fatal("signature_norm empty")
	}

	for _, q := range []string{
		"demo::RegisterHandler", "RegisterHandler", "RegisterHandler()", "registerhandler",
		"Client::Connect", "Connect", "Connect()",
	} {
		syms, err := st.FindSymbols(ctx, q, []string{"example-plugin-sdk"}, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(syms) == 0 {
			t.Fatalf("lookup %q: no hits", q)
		}
	}
}

func TestClampSearchLimit(t *testing.T) {
	cases := []struct {
		req, cfg, want int
	}{
		{0, 20, 20},
		{-1, 20, 20},
		{5, 20, 5},
		{50, 20, 20},
		{200, 20, 20},
		{200, 0, MaxSearchLimit},
		{50, 80, 50},
	}
	for _, tc := range cases {
		if got := ClampSearchLimit(tc.req, tc.cfg); got != tc.want {
			t.Fatalf("req=%d cfg=%d got %d want %d", tc.req, tc.cfg, got, tc.want)
		}
	}
}

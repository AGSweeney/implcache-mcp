// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestSelectDiverseCitationsPrefersProjectAndOfficial(t *testing.T) {
	in := []Citation{
		{URI: "project://app/notes.md", Authority: store.AuthorityGeneratedSummary, Section: "notes"},
		{URI: "project://app/src.cpp", Authority: store.AuthorityCurrentProject, Section: "init"},
		{URI: "project://sdk/api.md", Authority: store.AuthorityOfficialDocs, Section: "API"},
		{URI: "project://other/x.md", Authority: store.AuthorityThirdParty, Section: "x"},
	}
	out := selectDiverseCitations(in, 3, nil, nil)
	// Prefer fewer strong citations over padding MaxResults with generated filler.
	if len(out) != 2 {
		t.Fatalf("len=%d want 2 (project+official, no weak pad): %+v", len(out), out)
	}
	var haveProject, haveOfficial, haveGenerated bool
	for _, c := range out {
		switch c.Authority {
		case store.AuthorityCurrentProject:
			haveProject = true
		case store.AuthorityOfficialDocs:
			haveOfficial = true
		case store.AuthorityGeneratedSummary:
			haveGenerated = true
		}
	}
	if !haveProject || !haveOfficial {
		t.Fatalf("expected project+official in %+v", out)
	}
	if haveGenerated {
		t.Fatalf("generated filler should be demoted when stronger sources exist: %+v", out)
	}
}

func TestPackageSurvivesGeneratedDecoy(t *testing.T) {
	// Recreates the Autoresearch failure mode: query-cloned generated summary
	// dominates FTS, while project + official hold the real implementation evidence.
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "decoy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	task := "Add a custom command using RegisterCommand and AddMenuItem"
	docs := []store.UpsertInput{
		{
			URI: "project://plugin-app/docs/notes.md", Title: "notes",
			SourceType: store.SourceMarkdown, Path: "docs/notes.md", RootName: "plugin-app",
			Authority: store.AuthorityGeneratedSummary, Hash: "decoy",
			Chunks: []store.Chunk{{
				Heading: "notes",
				Body:    task + " for plugin command handler menu item init sequence howto guide",
				StartLine: 1, EndLine: 20,
			}},
			Symbols: []store.SymbolInput{{Name: "RegisterCommand", Kind: "mention", Language: "cpp"}},
		},
		{
			URI: "project://plugin-app/src/commands.cpp", Title: "commands",
			SourceType: store.SourceSource, Path: "src/commands.cpp", RootName: "plugin-app",
			Authority: store.AuthorityCurrentProject, Hash: "proj",
			// Intentionally weak lexical overlap with the task so FTS prefers the decoy.
			Chunks: []store.Chunk{{
				Heading: "initialization sequence",
				Body: "Example plugin init steps:\n- RegisterCommand(name, handler)\n- AddMenuItem(menu, command)\nYou must call RegisterCommand before AddMenuItem.",
				StartLine: 1, EndLine: 40,
			}},
			Symbols: []store.SymbolInput{
				{Name: "RegisterCommand", Kind: "function", Language: "cpp", StartLine: 10, EndLine: 20},
				{Name: "AddMenuItem", Kind: "function", Language: "cpp", StartLine: 30, EndLine: 40},
			},
		},
		{
			URI: "project://plugin-sdk/api.md", Title: "API",
			SourceType: store.SourceMarkdown, Path: "api.md", RootName: "plugin-sdk",
			Authority: store.AuthorityOfficialDocs, Hash: "sdk",
			Chunks: []store.Chunk{{
				Heading: "API",
				Body:    "RegisterCommand(name, handler) AddMenuItem(menu, command). Constraint: handlers must register during init.",
				StartLine: 1, EndLine: 20,
			}},
			Symbols: []store.SymbolInput{
				{Name: "RegisterCommand", Kind: "api", Language: "cpp"},
				{Name: "AddMenuItem", Kind: "api", Language: "cpp"},
			},
		},
	}
	for _, d := range docs {
		if _, err := st.UpsertDocument(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	// Confirm FTS alone still prefers the decoy (setup validity).
	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query: task, Limit: 10, Roots: []string{"plugin-app", "plugin-sdk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected FTS hits")
	}
	if hits[0].URI != "project://plugin-app/docs/notes.md" {
		t.Fatalf("setup: expected decoy to dominate FTS, top=%s auth=%s", hits[0].URI, hits[0].Authority)
	}

	res, err := Get(ctx, st, Request{
		Task:             task,
		Language:         "cpp",
		PreferredRoots:   []string{"plugin-app", "plugin-sdk"},
		MaxContextTokens: 2500,
	})
	if err != nil {
		t.Fatal(err)
	}

	var haveProject, haveOfficial, onlyGenerated bool
	generatedCount := 0
	for _, c := range res.Citations {
		switch c.Authority {
		case store.AuthorityCurrentProject:
			haveProject = true
		case store.AuthorityOfficialDocs, store.AuthorityOfficialExample:
			haveOfficial = true
		case store.AuthorityGeneratedSummary:
			generatedCount++
		}
	}
	onlyGenerated = len(res.Citations) > 0 && generatedCount == len(res.Citations)
	if onlyGenerated {
		t.Fatalf("package citations are only generated summaries: %+v", citationSummary(res))
	}
	if !haveProject {
		t.Fatalf("expected current_project citation in package: %+v", citationSummary(res))
	}
	if !haveOfficial {
		t.Fatalf("expected official citation in package: %+v", citationSummary(res))
	}
	if len(res.Examples) == 0 {
		t.Fatalf("expected examples from project source; got citations=%+v", citationSummary(res))
	}
	if len(res.Sequence) == 0 {
		t.Fatalf("expected grounded sequence from hydrated project doc")
	}
	if len(res.Constraints) == 0 {
		t.Fatalf("expected constraints from project/official docs")
	}
}

func TestTightBudgetKeepsDiverseAuthorities(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "tight.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	for _, d := range []store.UpsertInput{
		{
			URI: "project://device/decoy.md", Title: "decoy", SourceType: store.SourceMarkdown,
			Path: "decoy.md", RootName: "device", Authority: store.AuthorityGeneratedSummary, Hash: "d",
			Chunks: []store.Chunk{{Body: "WireSpiTransfer ConfigurePin expander howto", StartLine: 1, EndLine: 5}},
		},
		{
			URI: "project://device/gpio.cpp", Title: "gpio", SourceType: store.SourceSource,
			Path: "gpio.cpp", RootName: "device", Authority: store.AuthorityCurrentProject, Hash: "p",
			Chunks: []store.Chunk{{
				Heading: "example",
				Body:    "Example: ConfigurePin then SpiTransfer. You must ConfigurePin first.",
				StartLine: 1, EndLine: 20,
			}},
			Symbols: []store.SymbolInput{
				{Name: "SpiTransfer", Kind: "function", Language: "cpp"},
				{Name: "ConfigurePin", Kind: "function", Language: "cpp"},
			},
		},
		{
			URI: "project://device/spi.md", Title: "spi", SourceType: store.SourceMarkdown,
			Path: "spi.md", RootName: "device", Authority: store.AuthorityOfficialDocs, Hash: "o",
			Chunks: []store.Chunk{{Body: "SpiTransfer API ConfigurePin mode", StartLine: 1, EndLine: 10}},
			Symbols: []store.SymbolInput{{Name: "SpiTransfer", Kind: "api", Language: "cpp"}},
		},
	} {
		if _, err := st.UpsertDocument(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	res, err := Get(ctx, st, Request{
		Task:             "SpiTransfer ConfigurePin",
		Language:         "cpp",
		PreferredRoots:   []string{"device"},
		MaxResults:       2,
		MaxContextTokens: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Citations) > 2 {
		t.Fatalf("expected <=2 citations under MaxResults=2, got %d", len(res.Citations))
	}
	auths := map[string]bool{}
	for _, c := range res.Citations {
		auths[c.Authority] = true
	}
	if !auths[store.AuthorityCurrentProject] {
		t.Fatalf("tight budget dropped project source: %+v", citationSummary(res))
	}
	if !auths[store.AuthorityOfficialDocs] && !auths[store.AuthorityOfficialExample] {
		t.Fatalf("tight budget dropped official source: %+v", citationSummary(res))
	}
	if auths[store.AuthorityGeneratedSummary] {
		t.Fatalf("tight budget kept generated filler over stronger sources: %+v", citationSummary(res))
	}
}

func citationSummary(res *Response) string {
	var parts []string
	for _, c := range res.Citations {
		parts = append(parts, c.URI+"["+c.Authority+"]")
	}
	return strings.Join(parts, ", ")
}

func TestSelectPackageSymbolsDropsGeneratedDuplicate(t *testing.T) {
	in := []store.Symbol{
		{Name: "RegisterCommand", URI: "project://app/src.cpp", Authority: store.AuthorityCurrentProject, Kind: "function"},
		{Name: "RegisterCommand", URI: "project://app/notes.md", Authority: store.AuthorityGeneratedSummary, Kind: "mention"},
		{Name: "AddMenuItem", URI: "project://sdk/api.md", Authority: store.AuthorityOfficialDocs, Kind: "api"},
	}
	out := selectPackageSymbols(in, 5, nil)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(out), out)
	}
	for _, s := range out {
		if s.Authority == store.AuthorityGeneratedSummary {
			t.Fatalf("generated duplicate survived: %+v", out)
		}
	}
}

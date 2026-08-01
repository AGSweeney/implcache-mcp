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

func TestRootGroupAllowsCrossFamilyCombine(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	for _, d := range []store.UpsertInput{
		{
			URI: "project://NetBurner Documents/tcp.md", Title: "tcp", SourceType: store.SourceMarkdown,
			Path: "tcp.md", RootName: "NetBurner Documents", Authority: store.AuthorityOfficialDocs, Hash: "d1",
			Chunks: []store.Chunk{{Heading: "TCP", Body: "Listen accept TCP server API docs", StartLine: 1, EndLine: 10}},
		},
		{
			URI: "project://NetBurner Examples/TcpServer/main.cpp", Title: "main", SourceType: store.SourceSource,
			Path: "main.cpp", RootName: "NetBurner Examples", Authority: store.AuthorityOfficialExample, Hash: "e1",
			Chunks: []store.Chunk{{
				Heading: "example",
				Body:    "Example TCP server:\n- Listen\n- Accept\nYou must call Listen before Accept.",
				StartLine: 1, EndLine: 20,
			}},
			Symbols: []store.SymbolInput{{Name: "Listen", Kind: "function", Language: "cpp"}},
		},
	} {
		if _, err := st.UpsertDocument(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UpsertKnowledgeGroup(ctx, store.RootGroup{
		ID: "netburner", Name: "NetBurner", Description: "test",
		Policies: store.DefaultKnowledgeGroupPolicies(),
		Members: []store.RootGroupMember{
			{RootName: "NetBurner Examples", Role: store.MemberRoleOfficialExample, Priority: 100},
			{RootName: "NetBurner Documents", Role: store.MemberRoleOfficialDocs, Priority: 90},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Dual preferredRoots in one knowledge group auto-expand (no needsChoice).
	respAuto, err := Get(ctx, st, Request{
		Task: "TCP Listen Accept server example", Language: "cpp",
		PreferredRoots: []string{"NetBurner Documents", "NetBurner Examples"},
		MaxContextTokens: 2500,
	})
	if err != nil {
		t.Fatalf("same knowledge group should auto-expand, got %v", err)
	}
	if respAuto.KnowledgeGroup != "netburner" {
		t.Fatalf("KnowledgeGroup=%q", respAuto.KnowledgeGroup)
	}

	resp, err := Get(ctx, st, Request{
		Task: "TCP Listen Accept server example", Language: "cpp", KnowledgeGroup: "netburner",
		MaxContextTokens: 2500,
	})
	if err != nil {
		t.Fatal(err)
	}
	var haveDocs, haveEx bool
	for _, c := range resp.Citations {
		if strings.Contains(c.URI, "Documents") {
			haveDocs = true
		}
		if strings.Contains(c.URI, "Examples") {
			haveEx = true
		}
	}
	for _, e := range resp.Examples {
		if strings.Contains(e.URI, "Examples") || e.Authority == store.AuthorityOfficialExample {
			haveEx = true
		}
	}
	if !haveDocs || !haveEx {
		t.Fatalf("rootGroup package should include docs+examples; cites=%v examples=%d trace=%v",
			resp.Citations, len(resp.Examples), resp.SelectionTrace)
	}
	if len(resp.Examples) == 0 {
		t.Fatal("official_example must be eligible for examples[]")
	}
	if len(resp.SelectionTrace) == 0 {
		t.Fatal("expected selection trace reasons")
	}
}

func TestOfficialExampleReservedInSelection(t *testing.T) {
	in := []Citation{
		{URI: "project://docs/api.md", Authority: store.AuthorityOfficialDocs, Section: "API"},
		{URI: "project://ex/main.cpp", Authority: store.AuthorityOfficialExample, Section: "example"},
		{URI: "project://gen/notes.md", Authority: store.AuthorityGeneratedSummary, Section: "notes"},
	}
	out := selectDiverseCitations(in, 2, nil, nil)
	if len(out) != 2 {
		t.Fatalf("len=%d: %+v", len(out), out)
	}
	var haveEx, haveGen bool
	for _, c := range out {
		if c.Authority == store.AuthorityOfficialExample {
			haveEx = true
		}
		if c.Authority == store.AuthorityGeneratedSummary {
			haveGen = true
		}
	}
	if !haveEx {
		t.Fatalf("expected official_example reserved: %+v", out)
	}
	if haveGen {
		t.Fatalf("generated should not pad: %+v", out)
	}
}

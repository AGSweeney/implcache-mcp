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

func TestSerializedBudgetRespected(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	body := strings.Repeat("example initialization constraint pitfall ", 80)
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://example-plugin-sdk/guide.md", Title: "guide",
		SourceType: store.SourceMarkdown, Path: "guide.md", RootName: "example-plugin-sdk",
		Authority: store.AuthorityOfficialDocs, Hash: "b1",
		Chunks: []store.Chunk{
			{Heading: "Example", Body: body, StartLine: 1, EndLine: 40},
			{Heading: "Constraints", Body: "You must call Init before Use. " + body, StartLine: 41, EndLine: 80},
			{Heading: "Pitfalls", Body: "Common error: skipping cleanup. " + body, StartLine: 81, EndLine: 120},
		},
		Symbols: []store.SymbolInput{
			{Name: "InitDevice", Kind: "function", Language: "cpp", StartLine: 1},
			{Name: "UseDevice", Kind: "function", Language: "cpp", StartLine: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	const maxTok = 400
	res, err := Get(ctx, st, Request{
		Task:             "InitDevice UseDevice example constraints pitfalls",
		Language:         "cpp",
		PreferredRoots:   []string{"example-plugin-sdk"},
		MaxContextTokens: maxTok,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, tokens, payload := serializeTokens(res)
	if tokens > maxTok+maxTok/10 {
		t.Fatalf("serialized tokens %d exceed budget %d (+10%%); payload=%d bytes", tokens, maxTok, len(payload))
	}
	if res.EstimatedTokens <= 0 {
		t.Fatalf("EstimatedTokens unset")
	}
	// Allow tiny drift from including EstimatedTokens digits in the wire payload.
	if abs(res.EstimatedTokens-tokens) > 8 {
		t.Fatalf("EstimatedTokens=%d serialize=%d", res.EstimatedTokens, tokens)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestRecipePopulatesSequence(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://example-plugin-sdk/stub.md", Title: "stub",
		SourceType: store.SourceMarkdown, Path: "stub.md", RootName: "example-plugin-sdk",
		Hash: "s1", Chunks: []store.Chunk{{Body: "stub", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertKnowledgeEntry(ctx, store.KnowledgeEntry{
		URI: "project://example-plugin-sdk/_recipes/cmd.md", Subject: "register plugin command",
		Technology: "Example Plugin SDK", Language: "cpp",
		BodyMarkdown: `# Register plugin command

## Required APIs
- ` + "`RegisterCommand`" + `
- ` + "`AddMenuItem`" + `

## Includes
- #include <Plugin.h>

## Sequence
1. Call RegisterCommand with the handler
2. Call AddMenuItem to expose the command
3. Return success from init

## Pitfalls
- Do not register after UI freeze
`,
		ReviewStatus: store.ReviewHumanReviewed,
		Authority:    store.AuthorityCuratedRecipe,
		RootName:     "example-plugin-sdk",
		Hash:         "recipe1",
		SourceURIs:   []string{"project://example-plugin-sdk/stub.md"},
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := Get(ctx, st, Request{
		Task: "register plugin command", Language: "cpp", Technology: "Example Plugin SDK",
		PreferredRoots: []string{"example-plugin-sdk"}, MaxContextTokens: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sequence) < 2 {
		t.Fatalf("expected grounded sequence from recipe, got %v", res.Sequence)
	}
	if res.RecipeReviewStatus != store.ReviewHumanReviewed {
		t.Fatalf("review status=%q", res.RecipeReviewStatus)
	}
	joined := strings.Join(res.RequiredAPIs, " ")
	if !strings.Contains(joined, "RegisterCommand") {
		t.Fatalf("apis=%v", res.RequiredAPIs)
	}
}

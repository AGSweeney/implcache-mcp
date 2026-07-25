// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"testing"

	"implcache-mcp/store"
)

func TestClassifyRecipeSectionHeadings(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"Required prerequisites", "prereqs"},
		{"Requirements", "prereqs"},
		{"Required APIs", "apis"},
		{"API reference", "apis"},
		{"Required headers", "includes"},
		{"Initialization sequence", "sequence"},
		{"Workflow", "sequence"},
		{"Cleanup requirements", "cleanup"},
		{"Known constraints", "constraints"},
		{"Known pitfalls", "pitfalls"},
		{"Example", "examples"},
		{"Version", "version"},
		{"Miscellaneous notes", "body"},
	}
	for _, tc := range cases {
		if got := classifyRecipeSection(tc.title); got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.title, got, tc.want)
		}
	}
}

func TestParseRecipeHeadingFieldsDeterministic(t *testing.T) {
	body := `# Demo recipe

## Required prerequisites
- SDK installed

## Requirements
- License accepted

## Required APIs
- ` + "`demo::RegisterHandler`" + `
- ` + "`Client::Connect`" + `

## Required headers
- #include "demo/api.h"

## Initialization sequence
1. Call RegisterHandler
2. Call Connect

## Workflow
1. Open
2. Close

## Cleanup requirements
- Release handles

## Known constraints
- Single-threaded init

## Known pitfalls
- Do not call Connect twice

## Example
` + "```" + `
RegisterHandler("x");
` + "```" + `

## Version
1.2
`
	rf := parseRecipe(store.KnowledgeEntry{
		Subject:      "Demo",
		BodyMarkdown: body,
		Version:      "1.0",
	})
	if len(rf.Prereqs) < 2 {
		t.Fatalf("prereqs=%v", rf.Prereqs)
	}
	if len(rf.APIs) < 2 {
		t.Fatalf("APIs=%v", rf.APIs)
	}
	if len(rf.Includes) == 0 {
		t.Fatal("expected includes")
	}
	if !rf.HasSequence || len(rf.Sequence) < 2 {
		t.Fatalf("sequence=%v", rf.Sequence)
	}
	if len(rf.Cleanup) == 0 {
		t.Fatal("expected cleanup")
	}
	if len(rf.Constraints) == 0 {
		t.Fatal("expected constraints")
	}
	if len(rf.Pitfalls) == 0 {
		t.Fatal("expected pitfalls")
	}
	if len(rf.Examples) == 0 {
		t.Fatal("expected examples")
	}
	if rf.Version != "1.2" {
		t.Fatalf("version=%q", rf.Version)
	}
	rf2 := parseRecipe(store.KnowledgeEntry{Subject: "Demo", BodyMarkdown: body})
	if len(rf.APIs) != len(rf2.APIs) || rf.APIs[0] != rf2.APIs[0] {
		t.Fatalf("nondeterministic APIs: %v vs %v", rf.APIs, rf2.APIs)
	}
}

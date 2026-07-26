// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestGetImplementationContextBudgeted(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://demo/menu.c", Title: "menu.c", SourceType: store.SourceSource,
		Path: "menu.c", RootName: "demo", Authority: store.AuthorityOfficialExample,
		Language: "c", Hash: "1",
		Chunks: []store.Chunk{{
			Heading:   "RegisterHandler",
			Body:      "#include <MenuBar.h>\nint RegisterHandler(){\n  RegisterCommand(\"App.Hello\", ...);\n  AddMenuItem(...);\n  return 0;\n}\n",
			StartLine: 1, EndLine: 10,
		}},
		Symbols: []store.SymbolInput{
			{Name: "RegisterCommand", Kind: "api", Language: "c", StartLine: 3},
			{Name: "AddMenuItem", Kind: "api", Language: "c", StartLine: 4},
			{Name: "RegisterHandler", Kind: "function", Language: "c", StartLine: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := Get(ctx, st, Request{
		Task:             "RegisterHandler RegisterCommand menubar pushbutton",
		Language:         "c",
		Technology:       "example-device-sdk",
		PreferredRoots:   []string{"demo"},
		MaxContextTokens: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Coverage == "low" {
		t.Fatalf("expected better coverage: %+v", res)
	}
	if len(res.Citations) == 0 {
		t.Fatal("expected citations")
	}
	joined := strings.Join(res.RequiredAPIs, " ")
	if !strings.Contains(joined, "RegisterCommand") {
		t.Fatalf("missing API in %v", res.RequiredAPIs)
	}
	if res.EstimatedTokens <= 0 || res.EstimatedTokens > 2500 {
		t.Fatalf("token estimate odd: %d", res.EstimatedTokens)
	}
	// Unknown freshness may still recommend verification even with good coverage.
	if res.Freshness == "" {
		t.Fatal("expected freshness")
	}
	// Fabricated "Use `API`" sequences are no longer allowed.
	for _, step := range res.Sequence {
		if strings.HasPrefix(step, "Use `") {
			t.Fatalf("ungrounded sequence step: %q", step)
		}
	}
}

func TestDebugTaskTokensAndVersionSignals(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "dbg.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://demo/a.md", Title: "a", SourceType: store.SourceMarkdown,
		Path: "a.md", RootName: "demo", Hash: "1", ProductVersion: "1.0",
		Chunks: []store.Chunk{{Body: "RegisterHandler notes", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Get(ctx, st, Request{
		Task: "wire RegisterHandler into the menu", PreferredRoots: []string{"demo"},
		Version: "2.0", Debug: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.DebugTaskTokens) == 0 || !strings.Contains(strings.Join(res.DebugTaskTokens, " "), "RegisterHandler") {
		t.Fatalf("debug tokens=%v", res.DebugTaskTokens)
	}
	joined := strings.Join(res.MissingInformation, " ")
	if !strings.Contains(joined, "2.0") {
		t.Fatalf("expected version missingInformation, got %v", res.MissingInformation)
	}
}

func TestPreferredRootsMultiFamilyNeedsChoice(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "mf.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	for _, root := range []string{"example-device-sdk", "example-plugin-sdk"} {
		_, err := st.UpsertDocument(ctx, store.UpsertInput{
			URI: "project://" + root + "/a.md", Title: root, SourceType: store.SourceMarkdown,
			Path: "a.md", RootName: root, Hash: root,
			Chunks: []store.Chunk{{Body: "body " + root, StartLine: 1, EndLine: 1}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = Get(ctx, st, Request{
		Task:           "do something",
		PreferredRoots: []string{"example-device-sdk", "example-plugin-sdk"},
	})
	var need *store.ErrNeedsRoot
	if err == nil || !errors.As(err, &need) || !need.Inference.NeedsChoice {
		t.Fatalf("expected needsChoice, err=%v", err)
	}
}

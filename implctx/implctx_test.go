package implctx

import (
	"context"
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
			Heading:   "user_initialize",
			Body:      "#include <ProMenuBar.h>\nint user_initialize(){\n  ProCmdActionAdd(\"App.Hello\", ...);\n  ProMenubarmenuPushbuttonAdd(...);\n  return 0;\n}\n",
			StartLine: 1, EndLine: 10,
		}},
		Symbols: []store.SymbolInput{
			{Name: "ProCmdActionAdd", Kind: "api", Language: "c", StartLine: 3},
			{Name: "ProMenubarmenuPushbuttonAdd", Kind: "api", Language: "c", StartLine: 4},
			{Name: "user_initialize", Kind: "function", Language: "c", StartLine: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := Get(ctx, st, Request{
		Task:             "user_initialize ProCmdActionAdd menubar pushbutton",
		Language:         "c",
		Technology:       "Creo TOOLKIT",
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
	if !strings.Contains(joined, "ProCmdActionAdd") {
		t.Fatalf("missing API in %v", res.RequiredAPIs)
	}
	if res.EstimatedTokens <= 0 || res.EstimatedTokens > 2500 {
		t.Fatalf("token estimate odd: %d", res.EstimatedTokens)
	}
	if res.WebSearchRecommended && res.Coverage == "high" {
		t.Fatal("high coverage should not recommend web search")
	}
}

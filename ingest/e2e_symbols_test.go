// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"implcache-mcp/store"
)

func TestIngestCPPSymbolFormsAndLookup(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `int demo::RegisterHandler(const char* name) {
  return 0;
}

Status Client::Connect(const Config& config) {
  return Status{};
}
`
	if err := os.WriteFile(filepath.Join(src, "api.cpp"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(dir, "t.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	res, err := IngestProjectOpts(ctx, st, ProjectOptions{
		Path: src, RootName: "example-plugin-sdk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Ingested < 1 {
		t.Fatalf("ingested=%d", res.Ingested)
	}

	for _, q := range []string{
		"demo::RegisterHandler",
		"RegisterHandler",
		"RegisterHandler()",
		"registerhandler",
		"Client::Connect",
		"Connect",
		"Connect()",
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

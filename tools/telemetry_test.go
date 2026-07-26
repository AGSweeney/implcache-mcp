// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"implcache-mcp/implctx"
	"implcache-mcp/store"
	"implcache-mcp/usage"
)

func TestGetImplementationContextRecordsUsage(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().Unix()
	_, err = st.UpsertDocument(context.Background(), store.UpsertInput{
		URI: "project://demo-sdk/api.md", Title: "API", SourceType: store.SourceMarkdown,
		Path: "api.md", RootName: "demo-sdk", Authority: store.AuthorityOfficialDocs,
		Hash: "h1", Mtime: now,
		Chunks: []store.Chunk{{Ordinal: 0, Heading: "Init", Body: "Call InitDevice before open."}},
	})
	if err != nil {
		t.Fatal(err)
	}

	us, err := usage.Open(filepath.Join(dir, "u.db"), usage.Config{Enabled: true, RetentionDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	defer us.Close()

	res, err := implctx.Get(context.Background(), st, implctx.Request{
		Task: "InitDevice open", PreferredRoots: []string{"demo-sdk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{Usage: us}
	opt.recordUsage(usage.FromImplementationContext("get_implementation_context", "InitDevice open", res, time.Millisecond))
	opt.recordUsage(usage.RootSelectionEvent("get_implementation_context", "ambiguous", []string{"a", "b"}, time.Millisecond))

	deadline := time.Now().Add(3 * time.Second)
	for us.Status(context.Background()).RequestCount < 2 {
		if time.Now().After(deadline) {
			t.Fatal("no usage events")
		}
		time.Sleep(40 * time.Millisecond)
	}

	// Nil usage must not panic
	Options{}.recordUsage(usage.RequestEvent{RequestID: "x", ToolName: "t", ResultStatus: usage.StatusNoLocalMatch})
}

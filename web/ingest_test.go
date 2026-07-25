// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
	"implcache-mcp/web"
)

func TestIngestURLIndexesPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Serial Port</title></head>
<body><nav>ignore</nav><div id="page_content">
<h1>Serial Port Configuration</h1>
<p>Use ConfigureUART with baud rate 115200.</p>
</div></body></html>`))
	}))
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	host := strings.TrimPrefix(srv.URL, "http://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	// httptest uses 127.0.0.1 — allow explicitly for the test allowlist.
	res, err := web.IngestURL(ctx, st, web.IngestURLOptions{
		URL:               srv.URL + "/uart.html",
		RootName:          "example-docs",
		AllowInsecureHTTP: true,
		ExtraAllowedHosts: map[string]struct{}{"127.0.0.1": {}},
		Profile:           web.ProfileGeneric,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped || res.Chunks == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query: "ConfigureUART baud", Limit: 5, Roots: []string{"example-docs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected searchable hits after url ingest")
	}
	_ = host
}

func TestFetchRejectsPrivateWithoutAllowlist(t *testing.T) {
	_, err := web.FetchURL(context.Background(), "http://127.0.0.1:1/", web.FetchOptions{
		AllowInsecureHTTP: true,
	})
	if err == nil {
		t.Fatal("expected 127.0.0.1 blocked")
	}
}

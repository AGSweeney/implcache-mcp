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

func TestCrawlSiteRespectsPrefixAndIndexes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><title>Home</title></head><body role="main">
			<div role="main"><h1>SDK Docs</h1><p>Welcome</p>
			<a href="/api/uart.html">UART</a>
			<a href="/outside.html">Outside</a>
			</div></body></html>`))
		case "/api/uart.html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><title>UART</title></head>
			<div role="main"><h1>UART API</h1><p>ConfigureUART sets baud rate.</p></div></html>`))
		case "/outside.html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><p>Should not be crawled</p></body></html>`))
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "crawl.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	prefix := strings.TrimRight(srv.URL, "/") + "/api/"
	start := strings.TrimRight(srv.URL, "/") + "/api/uart.html"
	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name: "fixture-sdk", RootName: "fixture-sdk", StartURL: start,
		Profile: web.ProfileSphinx, AllowedPrefixes: []string{prefix},
		Authority: store.AuthorityOfficialDocs, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	rep, err := web.CrawlSite(ctx, st, web.CrawlOptions{
		SourceName:        "fixture-sdk",
		MaxPages:          20,
		MaxDepth:          3,
		AllowInsecureHTTP: true,
		ExtraAllowedHosts: map[string]struct{}{"127.0.0.1": {}},
		CrawlDelay:        0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.New+rep.Changed == 0 {
		t.Fatalf("expected ingested pages: %+v", rep)
	}
	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query: "ConfigureUART", Limit: 5, Roots: []string{"fixture-sdk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits from crawl")
	}
	// Outside prefix must not be indexed under this root as outside.html
	docs, err := st.ListDocuments(ctx, store.SourceWeb)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if strings.Contains(d.URI, "outside") {
			t.Fatalf("crawled outside prefix: %s", d.URI)
		}
	}
}

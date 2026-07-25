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
	"sync/atomic"
	"testing"

	"implcache-mcp/store"
	"implcache-mcp/web"
)

func testCrawlOpts(name string) web.CrawlOptions {
	return web.CrawlOptions{
		SourceName:        name,
		MaxPages:          50,
		MaxDepth:          4,
		AllowInsecureHTTP: true,
		ExtraAllowedHosts: map[string]struct{}{"127.0.0.1": {}},
		CrawlDelay:        0,
	}
}

func TestRefreshRespects304(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/page.html", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(`<html><body role="main"><h1>Stable</h1><p>RetryPolicy backoff</p></body></html>`))
	})
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow:\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	start := strings.TrimRight(srv.URL, "/") + "/docs/page.html"
	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name: "etag-src", RootName: "etag-src", StartURL: start,
		Profile: web.ProfileGeneric, AllowedPrefixes: []string{strings.TrimRight(srv.URL, "/") + "/docs/"},
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := web.CrawlSite(ctx, st, testCrawlOpts("etag-src")); err != nil {
		t.Fatal(err)
	}
	opt := testCrawlOpts("etag-src")
	opt.RefreshOnly = true
	rep, err := web.CrawlSite(ctx, st, opt)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Unchanged < 1 {
		t.Fatalf("expected unchanged from 304: %+v", rep)
	}
	if hits.Load() < 2 {
		t.Fatalf("expected second conditional fetch, hits=%d", hits.Load())
	}
}

func TestChangedPageReindexes(t *testing.T) {
	var gen atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/a.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if gen.Load() == 0 {
			_, _ = w.Write([]byte(`<html><body><h1>A</h1><p>AlphaTokenOne unique</p></body></html>`))
			return
		}
		_, _ = w.Write([]byte(`<html><body><h1>A</h1><p>BetaTokenTwo unique</p></body></html>`))
	})
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	start := strings.TrimRight(srv.URL, "/") + "/docs/a.html"
	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name: "chg", RootName: "chg", StartURL: start,
		AllowedPrefixes: []string{strings.TrimRight(srv.URL, "/") + "/docs/"}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := web.CrawlSite(ctx, st, testCrawlOpts("chg")); err != nil {
		t.Fatal(err)
	}
	gen.Store(1)
	rep, err := web.CrawlSite(ctx, st, testCrawlOpts("chg"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Changed < 1 {
		t.Fatalf("expected changed: %+v", rep)
	}
	hits, err := st.SearchOpts(ctx, store.SearchOptions{Query: "BetaTokenTwo", Limit: 5, Roots: []string{"chg"}})
	if err != nil || len(hits) == 0 {
		t.Fatalf("expected new content indexed: hits=%d err=%v", len(hits), err)
	}
}

func TestMissingPageNotDeletedUntilPrune(t *testing.T) {
	linkGone := atomic.Bool{}
	linkGone.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/keep.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if linkGone.Load() {
			_, _ = w.Write([]byte(`<html><body><p>KeepPageToken</p><a href="gone.html">g</a></body></html>`))
			return
		}
		_, _ = w.Write([]byte(`<html><body><p>KeepPageToken</p></body></html>`))
	})
	mux.HandleFunc("/docs/gone.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>GonePageToken</p></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	base := strings.TrimRight(srv.URL, "/") + "/docs/"
	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name: "miss", RootName: "miss", StartURL: base + "keep.html",
		AllowedPrefixes: []string{base}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := web.CrawlSite(ctx, st, testCrawlOpts("miss")); err != nil {
		t.Fatal(err)
	}
	linkGone.Store(false)
	rep, err := web.CrawlSite(ctx, st, testCrawlOpts("miss"))
	if err != nil {
		t.Fatal(err)
	}
	if rep.MissingMarked < 1 {
		t.Fatalf("expected missing mark without delete: %+v", rep)
	}
	docs, err := st.ListDocuments(ctx, store.SourceWeb)
	if err != nil {
		t.Fatal(err)
	}
	foundGone := false
	for _, d := range docs {
		if strings.Contains(d.URI, "gone") {
			foundGone = true
		}
	}
	if !foundGone {
		t.Fatal("expected gone page to remain after successful crawl (no auto-delete)")
	}

	ws, err := st.GetWebSourceByName(ctx, "miss")
	if err != nil {
		t.Fatal(err)
	}
	// Second successful crawl without the link bumps missing_count again.
	if _, err := web.CrawlSite(ctx, st, testCrawlOpts("miss")); err != nil {
		t.Fatal(err)
	}
	n, err := st.PruneWebPages(ctx, ws.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected prune deleted >=1, got %d", n)
	}
	docs, err = st.ListDocuments(ctx, store.SourceWeb)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if strings.Contains(d.URI, "gone") {
			t.Fatalf("gone should be pruned: %s", d.URI)
		}
	}
}

func TestRobotsDisallowSkipped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /docs/secret/\n"))
	})
	mux.HandleFunc("/docs/ok.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>VisibleToken</p><a href="/docs/secret/x.html">s</a></body></html>`))
	})
	mux.HandleFunc("/docs/secret/x.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>SecretToken</p></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	base := strings.TrimRight(srv.URL, "/") + "/docs/"
	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name: "robots", RootName: "robots", StartURL: base + "ok.html",
		AllowedPrefixes: []string{base}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := web.CrawlSite(ctx, st, testCrawlOpts("robots")); err != nil {
		t.Fatal(err)
	}
	hits, err := st.SearchOpts(ctx, store.SearchOptions{Query: "SecretToken", Limit: 5, Roots: []string{"robots"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("robots Disallow should block secret page, hits=%d", len(hits))
	}
}

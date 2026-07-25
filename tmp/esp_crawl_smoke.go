//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"implcache-mcp/store"
	"implcache-mcp/web"
)

func main() {
	db := "./tmp/esp-idf-test.db"
	st, err := store.Open(db)
	if err != nil {
		fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	start := "https://docs.espressif.com/projects/esp-idf/en/stable/esp32/index.html"
	prefix := "https://docs.espressif.com/projects/esp-idf/en/stable/esp32/"
	_, err = st.UpsertWebSource(ctx, store.WebSource{
		Name:            "esp-idf-esp32-stable",
		RootName:        "esp-idf",
		StartURL:        start,
		Profile:         web.ProfileSphinx,
		AllowedPrefixes: []string{prefix},
		Authority:       store.AuthorityOfficialDocs,
		Product:         "ESP-IDF",
		DeclaredVersion: "stable",
		Target:          "esp32",
		Enabled:         true,
	})
	if err != nil {
		fatal(err)
	}

	t0 := time.Now()
	rep, err := web.CrawlSite(ctx, st, web.CrawlOptions{
		SourceName: "esp-idf-esp32-stable",
		MaxPages:   12,
		MaxDepth:   2,
		CrawlDelay: 150 * time.Millisecond,
	})
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
	fmt.Printf("crawl_wall=%s\n", time.Since(t0).Round(time.Millisecond))

	for _, q := range []string{"ESP-IDF", "API Reference", "Get Started", "esp32"} {
		hits, err := st.SearchOpts(ctx, store.SearchOptions{Query: q, Limit: 3, Roots: []string{"esp-idf"}})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("\nsearch %q -> %d hits\n", q, len(hits))
		for _, h := range hits {
			fmt.Printf("  - %s | %s | %s\n", h.URI, h.Title, trim(h.Snippet, 100))
		}
	}
	docs, err := st.ListDocuments(ctx, store.SourceWeb)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("\nweb documents=%d\n", len(docs))
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

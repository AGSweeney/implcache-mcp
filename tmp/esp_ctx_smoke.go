//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"implcache-mcp/implctx"
	"implcache-mcp/store"
)

func main() {
	st, err := store.Open("./tmp/esp-idf-test.db")
	if err != nil {
		fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	docs, _ := st.ListDocuments(ctx, "")
	fmt.Printf("documents=%d\n", len(docs))
	for _, d := range docs {
		fmt.Printf("  %s | %s\n", d.URI, d.Title)
	}

	for _, q := range []string{
		"install ESP-IDF",
		"Get Started ESP32",
		"API Reference",
		"gpio",
		"wifi",
		"freertos",
		"nvs",
		"RegisterHandler",
		"idf.py",
	} {
		hits, err := st.SearchOpts(ctx, store.SearchOptions{Query: q, Limit: 5, Roots: []string{"esp-idf"}})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("\n=== search %q (%d) ===\n", q, len(hits))
		for _, h := range hits {
			fmt.Printf("- %s\n  heading=%s\n  snippet=%s\n", h.URI, h.Heading, h.Snippet)
		}
	}

	res, err := implctx.Get(ctx, st, implctx.Request{
		Task:             "Get started with ESP-IDF on ESP32: install toolchain and create first project",
		Language:         "c",
		Technology:       "ESP-IDF",
		PreferredRoots:   []string{"esp-idf"},
		MaxContextTokens: 3500,
	})
	if err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	fmt.Println("\n=== implctx ===")
	_ = enc.Encode(res)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

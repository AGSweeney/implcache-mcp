// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/pdf"
	"implcache-mcp/store"
	"implcache-mcp/web"
)

func main() {
	dbPath := flag.String("db", "./implcache.db", "sqlite db path")
	path := flag.String("path", "", "path to ingest (markdown/project/pdf) or unused for delete-prefix")
	mode := flag.String("mode", "markdown", "markdown | project | delete-prefix | url | pdf-inspect | pdf-ingest | pdf-remove")
	recursive := flag.Bool("recursive", true, "recurse for markdown mode")
	rootName := flag.String("root", "", "rootName for project:// or pdf:// URIs")
	prefix := flag.String("prefix", "", "URI prefix for delete-prefix mode (e.g. file:///)")
	urlFlag := flag.String("url", "", "URL for url mode")
	uriFlag := flag.String("uri", "", "Document URI for pdf-remove mode")
	profile := flag.String("profile", "generic", "web extraction profile: generic|sphinx|doxygen")
	allowHTTP := flag.Bool("allow-http", false, "permit http:// for url mode")
	pageStart := flag.Int("page-start", 0, "1-based start page for PDF modes")
	pageEnd := flag.Int("page-end", 0, "1-based end page for PDF modes")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	start := time.Now()

	switch *mode {
	case "markdown":
		if *path == "" {
			log.Fatal("-path is required")
		}
		res, err := ingest.IngestMarkdown(ctx, st, *path, *recursive, *rootName)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=markdown root=%s ingested=%d skipped=%d errors=%d elapsed=%s\n",
			res.RootName, res.Ingested, res.Skipped, len(res.Errors), time.Since(start).Round(time.Millisecond))
		printErrs(res.Errors)
	case "project":
		if *path == "" {
			log.Fatal("-path is required")
		}
		res, err := ingest.IngestProject(ctx, st, *path, *rootName)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=project root=%s ingested=%d skipped=%d errors=%d elapsed=%s\n",
			res.RootName, res.Ingested, res.Skipped, len(res.Errors), time.Since(start).Round(time.Millisecond))
		printErrs(res.Errors)
	case "delete-prefix":
		if *prefix == "" {
			log.Fatal("-prefix is required for delete-prefix")
		}
		n, err := st.DeleteDocumentsByURIPrefix(ctx, *prefix)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=delete-prefix deleted=%d prefix=%q elapsed=%s\n",
			n, *prefix, time.Since(start).Round(time.Millisecond))
	case "url":
		if *urlFlag == "" {
			log.Fatal("-url is required for url mode")
		}
		res, err := web.IngestURL(ctx, st, web.IngestURLOptions{
			URL:               *urlFlag,
			RootName:          *rootName,
			Profile:           *profile,
			AllowInsecureHTTP: *allowHTTP,
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=url status=%s uri=%s skipped=%v chunks=%d elapsed=%s\n",
			res.Status, res.DocumentURI, res.Skipped, res.Chunks, time.Since(start).Round(time.Millisecond))
	case "pdf-inspect":
		if *path == "" {
			log.Fatal("-path is required for pdf-inspect")
		}
		rep, err := pdf.InspectPDF(*path, pdf.InspectOptions{
			PageStart: *pageStart,
			PageEnd:   *pageEnd,
		})
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			log.Fatal(err)
		}
	case "pdf-ingest":
		if *path == "" {
			log.Fatal("-path is required for pdf-ingest")
		}
		res, err := pdf.IngestPDF(ctx, st, pdf.IngestOptions{
			Path:      *path,
			RootName:  *rootName,
			PageStart: *pageStart,
			PageEnd:   *pageEnd,
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=pdf-ingest status=%s uri=%s skipped=%v chunks=%d class=%s elapsed=%s\n",
			res.Status, res.DocumentURI, res.Skipped, res.Chunks, res.Classification,
			time.Since(start).Round(time.Millisecond))
	case "pdf-remove":
		uri := *uriFlag
		if uri == "" {
			log.Fatal("-uri is required for pdf-remove")
		}
		ok, err := pdf.RemovePDF(ctx, st, uri)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=pdf-remove deleted=%v uri=%s elapsed=%s\n",
			ok, uri, time.Since(start).Round(time.Millisecond))
	default:
		log.Fatalf("unknown mode %q", *mode)
	}

	os.Exit(0)
}

func printErrs(errs []string) {
	for i, e := range errs {
		if i >= 20 {
			fmt.Printf("... and %d more errors\n", len(errs)-20)
			break
		}
		fmt.Println("error:", e)
	}
}

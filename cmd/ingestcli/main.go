// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/store"
)

func main() {
	dbPath := flag.String("db", "./implcache.db", "sqlite db path")
	path := flag.String("path", "", "path to ingest (markdown/project) or unused for delete-prefix")
	mode := flag.String("mode", "markdown", "markdown | project | delete-prefix")
	recursive := flag.Bool("recursive", true, "recurse for markdown mode")
	rootName := flag.String("root", "", "rootName for project:// URIs")
	prefix := flag.String("prefix", "", "URI prefix for delete-prefix mode (e.g. file:///)")
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
		fmt.Println("ERR:", e)
	}
}

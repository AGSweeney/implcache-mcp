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
	"strings"
	"time"

	"implcache-mcp/gitrepo"
	"implcache-mcp/ingest"
	"implcache-mcp/pdf"
	"implcache-mcp/store"
	"implcache-mcp/web"
)

func main() {
	dbPath := flag.String("db", "./implcache.db", "sqlite db path")
	path := flag.String("path", "", "path to ingest (markdown/project/pdf/local repo)")
	mode := flag.String("mode", "markdown", "markdown|project|delete-prefix|purge-empty-docs|purge-recipes|url|pdf-*|repo-*")
	recursive := flag.Bool("recursive", true, "recurse for markdown mode")
	rootName := flag.String("root", "", "rootName for project://, pdf://, or git:// URIs")
	prefix := flag.String("prefix", "", "URI prefix for delete-prefix mode")
	urlFlag := flag.String("url", "", "URL for url mode / remote for repo modes")
	uriFlag := flag.String("uri", "", "Document URI for pdf-remove")
	nameFlag := flag.String("name", "", "repo source name")
	refFlag := flag.String("ref", "", "git branch/tag/commit")
	acqMode := flag.String("acq", "", "snapshot|managed_clone|local_checkout")
	sparse := flag.String("sparse", "", "comma-separated sparse paths")
	profile := flag.String("profile", "generic", "web extraction profile")
	allowHTTP := flag.Bool("allow-http", false, "permit http:// for url mode")
	pageStart := flag.Int("page-start", 0, "1-based start page for PDF modes")
	pageEnd := flag.Int("page-end", 0, "1-based end page for PDF modes")
	workingTree := flag.Bool("working-tree", false, "index working tree for local_checkout")
	removeIndex := flag.Bool("remove-index", true, "repo-remove: delete indexed docs")
	removeClone := flag.Bool("remove-clone", false, "repo-remove: delete managed clone")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	start := time.Now()
	cache := gitrepo.CacheRootForDB(*dbPath)

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
	case "purge-empty-docs":
		n, err := st.DeleteDocumentsWithoutChunks(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=purge-empty-docs deleted=%d elapsed=%s\n",
			n, time.Since(start).Round(time.Millisecond))
	case "purge-recipes":
		n, err := st.DeleteAllKnowledgeEntries(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=purge-recipes deleted=%d elapsed=%s\n",
			n, time.Since(start).Round(time.Millisecond))
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
		rep, err := pdf.InspectPDF(*path, pdf.InspectOptions{PageStart: *pageStart, PageEnd: *pageEnd})
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	case "pdf-ingest":
		if *path == "" {
			log.Fatal("-path is required for pdf-ingest")
		}
		res, err := pdf.IngestPDF(ctx, st, pdf.IngestOptions{
			Path: *path, RootName: *rootName, PageStart: *pageStart, PageEnd: *pageEnd,
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=pdf-ingest status=%s uri=%s skipped=%v chunks=%d class=%s elapsed=%s\n",
			res.Status, res.DocumentURI, res.Skipped, res.Chunks, res.Classification,
			time.Since(start).Round(time.Millisecond))
	case "pdf-remove":
		if *uriFlag == "" {
			log.Fatal("-uri is required for pdf-remove")
		}
		ok, err := pdf.RemovePDF(ctx, st, *uriFlag)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=pdf-remove deleted=%v uri=%s elapsed=%s\n", ok, *uriFlag, time.Since(start).Round(time.Millisecond))
	case "repo-inspect":
		rep, err := gitrepo.InspectRepo(ctx, gitrepo.InspectOptions{
			RemoteURL: *urlFlag, LocalPath: *path, Ref: *refFlag,
		})
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	case "repo-add":
		if *nameFlag == "" {
			log.Fatal("-name is required")
		}
		wt := "HEAD"
		if *workingTree {
			wt = "working_tree"
		}
		rs, err := gitrepo.AddRepoSource(ctx, st, gitrepo.IngestOptions{
			Name: *nameFlag, RemoteURL: *urlFlag, LocalPath: *path, RootName: *rootName,
			AcquisitionMode: *acqMode, Ref: *refFlag, SparsePaths: splitCSV(*sparse),
			WorkingTreeMode: wt,
		})
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rs)
	case "repo-ingest":
		if *nameFlag == "" {
			*nameFlag = "repo"
		}
		modeAcq := *acqMode
		if modeAcq == "" {
			if *path != "" {
				modeAcq = "local_checkout"
			} else {
				modeAcq = "snapshot"
			}
		}
		wt := "HEAD"
		if *workingTree {
			wt = "working_tree"
		}
		res, err := gitrepo.IngestRepo(ctx, st, gitrepo.IngestOptions{
			Name: *nameFlag, RemoteURL: *urlFlag, LocalPath: *path, RootName: *rootName,
			AcquisitionMode: modeAcq, Ref: *refFlag, SparsePaths: splitCSV(*sparse),
			WorkingTreeMode: wt, PersistSource: true, CacheRoot: cache,
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=repo-ingest status=%s root=%s commit=%s ingested=%d elapsed=%s\n",
			res.Status, res.RootName, res.ResolvedCommit, res.DocumentsIngested,
			time.Since(start).Round(time.Millisecond))
	case "repo-refresh":
		if *nameFlag == "" {
			log.Fatal("-name is required")
		}
		res, err := gitrepo.RefreshRepoSource(ctx, st, *nameFlag, cache, nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=repo-refresh status=%s commit=%s ingested=%d deleted=%d elapsed=%s\n",
			res.Status, res.ResolvedCommit, res.DocumentsIngested, res.FilesDeleted,
			time.Since(start).Round(time.Millisecond))
	case "repo-list":
		list, err := st.ListRepoSources(ctx)
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(list)
	case "repo-remove":
		if *nameFlag == "" {
			log.Fatal("-name is required")
		}
		ok, err := gitrepo.RemoveRepoSource(ctx, st, *nameFlag, *removeIndex, *removeClone)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("mode=repo-remove deleted=%v name=%s\n", ok, *nameFlag)
	default:
		log.Fatalf("unknown mode %q", *mode)
	}

	os.Exit(0)
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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

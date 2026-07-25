// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// sourcevalidate runs controlled real-source ingest scenarios and writes JSON reports.
//
// Usage:
//
//	go run ./cmd/sourcevalidate -out testdata/validation/reports
//
// Network access is required for the ESP-IDF and NetBurner web scenarios.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"implcache-mcp/gitrepo"
	"implcache-mcp/pdf"
	"implcache-mcp/store"
	"implcache-mcp/web"

	_ "modernc.org/sqlite"
)

type inventory struct {
	Documents int64 `json:"documents"`
	Chunks    int64 `json:"chunks"`
	Symbols   int64 `json:"symbols"`
	DBBytes   int64 `json:"databaseBytes"`
}

type scenarioReport struct {
	Name        string         `json:"name"`
	SourceType  string         `json:"sourceType"`
	Description string         `json:"description"`
	StartedAt   time.Time      `json:"startedAt"`
	FinishedAt  time.Time      `json:"finishedAt"`
	ElapsedMS   int64          `json:"elapsedMs"`
	Before      inventory      `json:"before"`
	After       inventory      `json:"after"`
	Growth      inventory      `json:"growth"`
	Errors      []string       `json:"errors"`
	Warnings    []string       `json:"warnings"`
	Detail      map[string]any `json:"detail,omitempty"`
	OK          bool           `json:"ok"`
}

func main() {
	outDir := flag.String("out", "testdata/validation/reports", "directory for JSON reports")
	dbPath := flag.String("db", "", "sqlite path (default: <out>/validation.db)")
	repoPath := flag.String("repo", ".", "local git repo for git scenario")
	pdfPath := flag.String("pdf", "testdata/pdf/text_manual.pdf", "PDF fixture path")
	maxPages := flag.Int("max-pages", 25, "max pages per web crawl")
	maxDepth := flag.Int("max-depth", 2, "max crawl depth")
	skipWeb := flag.Bool("skip-web", false, "skip ESP-IDF and NetBurner crawls")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal(err)
	}
	if *dbPath == "" {
		*dbPath = filepath.Join(*outDir, "validation.db")
	}
	_ = os.Remove(*dbPath)
	_ = os.Remove(*dbPath + "-wal")
	_ = os.Remove(*dbPath + "-shm")

	st, err := store.Open(*dbPath)
	if err != nil {
		fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	var reports []scenarioReport

	if !*skipWeb {
		reports = append(reports, runScenario(ctx, st, *dbPath, "esp-idf-docs", "web",
			"ESP-IDF stable API-reference/system prefix (Sphinx)",
			func(ctx context.Context, st *store.Store) (map[string]any, []string, []string, error) {
				start := "https://docs.espressif.com/projects/esp-idf/en/stable/esp32/api-reference/system/"
				if _, err := st.UpsertWebSource(ctx, store.WebSource{
					Name: "esp-idf-system", RootName: "esp-idf-docs", StartURL: start,
					Profile: "sphinx", AllowedPrefixes: []string{start},
					Authority: store.AuthorityOfficialDocs, Product: "esp-idf", Enabled: true,
				}); err != nil {
					return nil, nil, nil, err
				}
				rep, err := web.CrawlSite(ctx, st, web.CrawlOptions{
					SourceName: "esp-idf-system",
					MaxPages:   *maxPages,
					MaxDepth:   *maxDepth,
					CrawlDelay: 150 * time.Millisecond,
				})
				if err != nil {
					return nil, nil, nil, err
				}
				warns := crawlWarnings(rep)
				if rep.FatalError != "" {
					return detailCrawl(rep), rep.PageErrors, warns, fmt.Errorf("%s", rep.FatalError)
				}
				return detailCrawl(rep), append([]string{}, rep.PageErrors...), warns, nil
			},
		))

		reports = append(reports, runScenario(ctx, st, *dbPath, "netburner-docs", "web",
			"NetBurner Developer Guide (Doxygen HTML)",
			func(ctx context.Context, st *store.Store) (map[string]any, []string, []string, error) {
				start := "https://www.netburner.com/NBDocs/Developer/html/index.html"
				prefix := "https://www.netburner.com/NBDocs/Developer/html/"
				if _, err := st.UpsertWebSource(ctx, store.WebSource{
					Name: "netburner-dev", RootName: "netburner-docs", StartURL: start,
					Profile: "doxygen", AllowedPrefixes: []string{prefix},
					Authority: store.AuthorityOfficialDocs, Product: "netburner", Enabled: true,
				}); err != nil {
					return nil, nil, nil, err
				}
				rep, err := web.CrawlSite(ctx, st, web.CrawlOptions{
					SourceName: "netburner-dev",
					MaxPages:   *maxPages,
					MaxDepth:   *maxDepth,
					CrawlDelay: 150 * time.Millisecond,
				})
				if err != nil {
					return nil, nil, nil, err
				}
				warns := crawlWarnings(rep)
				if rep.FatalError != "" {
					return detailCrawl(rep), rep.PageErrors, warns, fmt.Errorf("%s", rep.FatalError)
				}
				return detailCrawl(rep), append([]string{}, rep.PageErrors...), warns, nil
			},
		))
	}

	reports = append(reports, runScenario(ctx, st, *dbPath, "pdf-text-manual", "pdf",
		"Representative text PDF fixture (Stage 1)",
		func(ctx context.Context, st *store.Store) (map[string]any, []string, []string, error) {
			res, err := pdf.IngestPDF(ctx, st, pdf.IngestOptions{
				Path: *pdfPath, RootName: "validation-pdfs",
			})
			if err != nil {
				return nil, nil, nil, err
			}
			detail := map[string]any{
				"status": res.Status, "documentURI": res.DocumentURI,
				"chunks": res.Chunks, "skipped": res.Skipped,
				"classification": res.Classification, "pageCount": res.PageCount,
			}
			return detail, nil, append([]string{}, res.Warnings...), nil
		},
	))

	absRepo, err := filepath.Abs(*repoPath)
	if err != nil {
		fatal(err)
	}
	reports = append(reports, runScenario(ctx, st, *dbPath, "git-local-owned", "git",
		"Owned local Git checkout (sparse: store,ingest,gitrepo)",
		func(ctx context.Context, st *store.Store) (map[string]any, []string, []string, error) {
			cache := gitrepo.CacheRootForDB(*dbPath)
			res, err := gitrepo.IngestRepo(ctx, st, gitrepo.IngestOptions{
				Name: "implcache-local", LocalPath: absRepo, RootName: "implcache-src",
				AcquisitionMode: "local_checkout", WorkingTreeMode: "HEAD",
				SparsePaths:   []string{"store", "ingest", "gitrepo"},
				PersistSource: true, CacheRoot: cache,
			})
			if err != nil {
				return nil, nil, nil, err
			}
			detail := map[string]any{
				"status":            res.Status,
				"rootName":          res.RootName,
				"resolvedCommit":    res.ResolvedCommit,
				"documentsIngested": res.DocumentsIngested,
				"filesDiscovered":   res.FilesDiscovered,
				"filesSkipped":      res.FilesSkipped,
				"bytesProcessed":    res.BytesProcessed,
			}
			var errs []string
			if res.Status == "failed" {
				errs = append(errs, "ingest status failed")
			}
			return detail, errs, append([]string{}, res.Warnings...), nil
		},
	))

	summary := map[string]any{
		"generatedAt": time.Now().UTC(),
		"database":    *dbPath,
		"scenarios":   reports,
	}
	if v, err := st.SchemaVersion(ctx); err != nil {
		summary["schemaOK"] = false
		summary["schemaError"] = err.Error()
	} else {
		summary["schemaOK"] = v == 11
		summary["schemaVersion"] = v
	}
	final, err := takeInventory(ctx, *dbPath)
	if err != nil {
		fatal(err)
	}
	summary["finalInventory"] = final

	writeJSON(filepath.Join(*outDir, "summary.json"), summary)
	for _, r := range reports {
		writeJSON(filepath.Join(*outDir, r.Name+".json"), r)
	}

	failed := 0
	for _, r := range reports {
		status := "ok"
		if !r.OK {
			status = "FAIL"
			failed++
		}
		fmt.Printf("%s %-18s docs=%+d chunks=%+d symbols=%+d db=%+dB elapsed=%dms errors=%d warnings=%d\n",
			status, r.Name, r.Growth.Documents, r.Growth.Chunks, r.Growth.Symbols, r.Growth.DBBytes,
			r.ElapsedMS, len(r.Errors), len(r.Warnings))
	}
	fmt.Printf("wrote reports under %s (failed=%d)\n", *outDir, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func runScenario(
	ctx context.Context,
	st *store.Store,
	dbPath, name, sourceType, desc string,
	fn func(context.Context, *store.Store) (detail map[string]any, errors, warnings []string, err error),
) scenarioReport {
	rep := scenarioReport{
		Name: name, SourceType: sourceType, Description: desc,
		StartedAt: time.Now().UTC(),
		Errors:    []string{}, Warnings: []string{},
	}
	before, err := takeInventory(ctx, dbPath)
	if err != nil {
		rep.FinishedAt = time.Now().UTC()
		rep.Errors = append(rep.Errors, err.Error())
		return rep
	}
	rep.Before = before

	detail, errs, warns, runErr := fn(ctx, st)
	rep.Detail = detail
	rep.Errors = append(rep.Errors, errs...)
	rep.Warnings = append(rep.Warnings, warns...)
	fatal := false
	if runErr != nil {
		fatal = true
		rep.Errors = append(rep.Errors, runErr.Error())
	}

	after, err := takeInventory(ctx, dbPath)
	if err != nil {
		fatal = true
		rep.Errors = append(rep.Errors, err.Error())
	} else {
		rep.After = after
		rep.Growth = inventory{
			Documents: after.Documents - before.Documents,
			Chunks:    after.Chunks - before.Chunks,
			Symbols:   after.Symbols - before.Symbols,
			DBBytes:   after.DBBytes - before.DBBytes,
		}
	}
	rep.FinishedAt = time.Now().UTC()
	rep.ElapsedMS = rep.FinishedAt.Sub(rep.StartedAt).Milliseconds()
	// Per-page fetch failures are recorded in errors but do not fail the scenario
	// when the crawl still produced indexed content.
	rep.OK = !fatal && (rep.Growth.Documents > 0 || rep.Growth.Chunks > 0)
	return rep
}

func takeInventory(ctx context.Context, dbPath string) (inventory, error) {
	var inv inventory
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return inv, err
	}
	defer db.Close()
	_, _ = db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	err = db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM documents),
		(SELECT COUNT(*) FROM chunks),
		(SELECT COUNT(*) FROM symbols)`).Scan(&inv.Documents, &inv.Chunks, &inv.Symbols)
	if err != nil {
		return inv, err
	}
	fi, err := os.Stat(dbPath)
	if err == nil {
		inv.DBBytes = fi.Size()
	}
	return inv, nil
}

func crawlWarnings(rep *web.CrawlReport) []string {
	var warns []string
	if rep.LimitReached != "" {
		warns = append(warns, "limitReached="+rep.LimitReached)
	}
	if rep.Failed > 0 {
		warns = append(warns, fmt.Sprintf("failedPages=%d", rep.Failed))
	}
	return warns
}

func detailCrawl(rep *web.CrawlReport) map[string]any {
	return map[string]any{
		"sourceName": rep.SourceName, "rootName": rep.RootName,
		"generation": rep.Generation, "new": rep.New, "changed": rep.Changed,
		"unchanged": rep.Unchanged, "failed": rep.Failed, "skipped": rep.Skipped,
		"bytesDownloaded": rep.Bytes, "limitReached": rep.LimitReached,
		"durationMs": rep.DurationMS, "pageErrorCount": len(rep.PageErrors),
	}
}

func writeJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

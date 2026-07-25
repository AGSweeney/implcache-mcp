// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Command semscale measures semantic candidate latency on synthetic corpora.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"implcache-mcp/store"
)

func main() {
	chunksFlag := flag.String("chunks", "10000,100000", "comma-separated corpus sizes to measure")
	limit := flag.Int("limit", 20, "semantic result limit")
	iters := flag.Int("iters", 25, "timed iterations per size after warmup")
	query := flag.String("query", "network retry reconnect RetryPolicy exponential backoff", "search query")
	keepDB := flag.String("db-dir", "", "optional directory to keep generated DBs (default: temp)")
	flag.Parse()

	sizes, err := parseSizes(*chunksFlag)
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()
	var reports []sizeReport
	for _, n := range sizes {
		rep, err := measureSize(ctx, n, *limit, *iters, *query, *keepDB)
		if err != nil {
			fatal(err)
		}
		reports = append(reports, rep)
		fmt.Fprintf(os.Stderr, "measured %d chunks: p50=%s p95=%s candidates=%d\n",
			n, rep.TotalP50, rep.TotalP95, rep.CandidateLimit)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"query":   *query,
		"limit":   *limit,
		"iters":   *iters,
		"reports": reports,
	})
}

type sizeReport struct {
	Chunks         int    `json:"chunks"`
	SeedMS         int64  `json:"seedMs"`
	CandidateLimit int    `json:"candidateLimit"`
	CandidateRows  int    `json:"candidateRowsMedian"`
	ScoredRows     int    `json:"scoredRowsMedian"`
	Returned       int    `json:"returnedMedian"`
	IDFP50         string `json:"idfP50"`
	PostingP50     string `json:"postingFetchP50"`
	ScoreP50       string `json:"scoreP50"`
	HydrateP50     string `json:"hydrateP50"`
	TotalP50       string `json:"totalP50"`
	TotalP95       string `json:"totalP95"`
	SearchP50      string `json:"searchSemanticP50"`
	SearchP95      string `json:"searchSemanticP95"`
}

func measureSize(ctx context.Context, chunks, limit, iters int, query, keepDir string) (sizeReport, error) {
	var rep sizeReport
	rep.Chunks = chunks
	dir := keepDir
	var cleanup func()
	if dir == "" {
		tmp, err := os.MkdirTemp("", "implcache-semscale-*")
		if err != nil {
			return rep, err
		}
		dir = tmp
		cleanup = func() { _ = os.RemoveAll(tmp) }
		defer cleanup()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return rep, err
	}
	dbPath := filepath.Join(dir, fmt.Sprintf("scale-%d.db", chunks))
	_ = os.Remove(dbPath)

	seedStart := time.Now()
	st, err := store.Open(dbPath)
	if err != nil {
		return rep, err
	}
	defer st.Close()
	fmt.Fprintf(os.Stderr, "seeding %d chunks...\n", chunks)
	if err := st.SeedSyntheticSemanticCorpus(ctx, "example-scale-sdk", chunks); err != nil {
		return rep, err
	}
	rep.SeedMS = time.Since(seedStart).Milliseconds()
	fmt.Fprintf(os.Stderr, "seeded %d chunks in %dms\n", chunks, rep.SeedMS)

	roots := []string{"example-scale-sdk"}
	// Warmup
	for i := 0; i < 3; i++ {
		if _, _, err := st.SemanticCandidatesStats(ctx, query, roots, limit); err != nil {
			return rep, err
		}
	}

	var (
		totals, idfs, postings, scores, hydrates []time.Duration
		candRows, scoredRows, returned           []int
		searchTotals                             []time.Duration
		candLimit                                int
	)
	for i := 0; i < iters; i++ {
		_, stats, err := st.SemanticCandidatesStats(ctx, query, roots, limit)
		if err != nil {
			return rep, err
		}
		candLimit = stats.CandidateLimit
		totals = append(totals, stats.Total)
		idfs = append(idfs, stats.IDF)
		postings = append(postings, stats.PostingFetch)
		scores = append(scores, stats.Score)
		hydrates = append(hydrates, stats.Hydrate)
		candRows = append(candRows, stats.CandidateRows)
		scoredRows = append(scoredRows, stats.ScoredRows)
		returned = append(returned, stats.Returned)

		t0 := time.Now()
		if _, err := st.SearchOpts(ctx, store.SearchOptions{
			Query: query, Limit: limit, Roots: roots, Semantic: true,
		}); err != nil {
			return rep, err
		}
		searchTotals = append(searchTotals, time.Since(t0))
	}

	rep.CandidateLimit = candLimit
	rep.CandidateRows = medianInt(candRows)
	rep.ScoredRows = medianInt(scoredRows)
	rep.Returned = medianInt(returned)
	rep.IDFP50 = dur(percentile(idfs, 0.5))
	rep.PostingP50 = dur(percentile(postings, 0.5))
	rep.ScoreP50 = dur(percentile(scores, 0.5))
	rep.HydrateP50 = dur(percentile(hydrates, 0.5))
	rep.TotalP50 = dur(percentile(totals, 0.5))
	rep.TotalP95 = dur(percentile(totals, 0.95))
	rep.SearchP50 = dur(percentile(searchTotals, 0.5))
	rep.SearchP95 = dur(percentile(searchTotals, 0.95))
	return rep, nil
}

func parseSizes(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	var out []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil || n < 1 {
			return nil, fmt.Errorf("invalid chunk size %q", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no chunk sizes provided")
	}
	return out, nil
}

func percentile(vals []time.Duration, p float64) time.Duration {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), vals...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func medianInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]int(nil), vals...)
	sort.Ints(cp)
	return cp[len(cp)/2]
}

func dur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.3fms", float64(d)/float64(time.Millisecond))
	}
	return d.Round(100 * time.Microsecond).String()
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "semscale: %v\n", err)
	os.Exit(1)
}

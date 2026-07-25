// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"context"
	"os"

	"implcache-mcp/store"
)

// LibraryStats aggregates dashboard metrics.
type LibraryStats struct {
	Documents     int64 `json:"documents"`
	Chunks        int64 `json:"chunks"`
	Symbols       int64 `json:"symbols"`
	Recipes       int64 `json:"recipes"`
	DatabaseBytes int64 `json:"databaseBytes"`
	SourcesTotal  int   `json:"sourcesTotal"`
	SourcesOK     int   `json:"sourcesOk"`
	SourcesFailed int   `json:"sourcesFailed"`
	ActiveJobs    int   `json:"activeJobs"`
}

// GetLibraryStats returns inventory and source health counts.
func GetLibraryStats(ctx context.Context, st *store.Store, dbPath string, tracker *Tracker) (LibraryStats, error) {
	var s LibraryStats
	counts, err := st.CountLibrary(ctx)
	if err != nil {
		return s, err
	}
	s.Documents, s.Chunks, s.Symbols, s.Recipes = counts.Documents, counts.Chunks, counts.Symbols, counts.Recipes
	if fi, err := os.Stat(dbPath); err == nil {
		s.DatabaseBytes = fi.Size()
	}
	sources, err := ListSources(ctx, st)
	if err != nil {
		return s, err
	}
	s.SourcesTotal = len(sources)
	for _, src := range sources {
		switch classifyState(src.LastStatus) {
		case "ok":
			s.SourcesOK++
		case "failed", "degraded":
			s.SourcesFailed++
		}
	}
	if tracker != nil {
		for _, op := range tracker.List(100) {
			if op.State == "running" || op.State == "queued" {
				s.ActiveJobs++
			}
		}
	}
	return s, nil
}

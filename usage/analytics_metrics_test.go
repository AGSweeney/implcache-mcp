// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"implcache-mcp/implctx"
)

func waitFlush(t *testing.T, s *Store, min int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM request_events`).Scan(&n)
		if n >= min {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected >= %d events, got %d", min, n)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func TestLocalContextTokensAndReconciliation(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "u.db"), Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resp := &implctx.Response{
		Coverage:        "high",
		EstimatedTokens: 1000,
		Citations:       []implctx.Citation{{URI: "project://a/doc.md", RootName: "a"}},
		RootsUsed:       []string{"a"},
	}
	s.Record(FromImplementationContext("get_implementation_context", "task1", resp, time.Millisecond*10))
	s.Record(RootSelectionEvent("get_implementation_context", "task2", []string{"a", "b"}, time.Millisecond))
	s.Record(NoMatchEvent("search_knowledge", "task3", time.Millisecond))
	s.Record(ErrorEvent("get_document", "task4", "io", "boom", time.Millisecond))
	waitFlush(t, s, 4)

	sum, err := s.QuerySummary(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalRequests != 4 {
		t.Fatalf("total=%d", sum.TotalRequests)
	}
	if sum.LocalContextTokensServed != 1000 {
		t.Fatalf("tokens served=%d want 1000", sum.LocalContextTokensServed)
	}
	if sum.AvgPackageTokens == nil || *sum.AvgPackageTokens != 1000 {
		t.Fatalf("avg=%v", sum.AvgPackageTokens)
	}
	if !sum.ReconcileOK {
		t.Fatalf("reconcile failed: grounded=%d root=%d nomatch=%d insuff=%d err=%d sum=%d",
			sum.GroundedRequests, sum.RootSelectionRequired, sum.NoLocalMatch, sum.LocalInsufficient, sum.Errors, sum.ReconcileSum)
	}
	if sum.TokenEstimatorVersion != TokenEstimatorVersion {
		t.Fatalf("estimator=%s", sum.TokenEstimatorVersion)
	}

	eff, err := s.QueryEfficiency(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if eff.LocalContextTokensServed == nil || *eff.LocalContextTokensServed != 1000 {
		t.Fatalf("eff tokens=%v", eff.LocalContextTokensServed)
	}
	if eff.TokenEstimatorVersion != TokenEstimatorVersion {
		t.Fatalf("eff estimator=%s", eff.TokenEstimatorVersion)
	}
}

func TestCoverageUnclassifiedVsNotApplicable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "u.db"), Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Grounded without coverage -> unclassified
	ev := FromImplementationContext("get_implementation_context", "t", &implctx.Response{
		EstimatedTokens: 100,
		Citations:       []implctx.Citation{{URI: "project://x/a.md", RootName: "x"}},
		RootsUsed:       []string{"x"},
	}, time.Millisecond)
	ev.Coverage = CoverageUnclassified
	s.Record(ev)

	// Document fetch -> not applicable
	s.Record(NoMatchEvent("get_document", "doc", time.Millisecond))
	waitFlush(t, s, 2)

	sum, err := s.QuerySummary(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.UnclassifiedCoverage != 1 {
		t.Fatalf("unclassified=%d", sum.UnclassifiedCoverage)
	}
	if sum.NotApplicableCoverage < 1 {
		t.Fatalf("notApplicable=%d", sum.NotApplicableCoverage)
	}
	if !sum.UnclassifiedCoverageWarning {
		// 1/1 grounded = 100% unclassified
		t.Fatal("expected unclassified warning")
	}
	cov, err := s.QueryCoverageBreakdown(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if !cov.UnclassifiedWarning || cov.NotApplicable < 1 {
		t.Fatalf("coverage=%+v", cov)
	}
}

func TestDisabledStopsWrites(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "u.db"), Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.UpdateSettings(context.Background(), false, 90, false, false); err != nil {
		t.Fatal(err)
	}
	s.Record(ErrorEvent("get_implementation_context", "x", "x", "y", time.Millisecond))
	time.Sleep(200 * time.Millisecond)
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM request_events`).Scan(&n)
	if n != 0 {
		t.Fatalf("expected no writes when disabled, got %d", n)
	}
}

func TestExportAndMigrateV2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "u.db")
	s, err := Open(path, Config{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	st := s.Status(context.Background())
	if st.SchemaVersion != 2 {
		t.Fatalf("schema=%d", st.SchemaVersion)
	}
	s.Record(FromImplementationContext("get_implementation_context", "t", &implctx.Response{
		Coverage: "medium", EstimatedTokens: 400,
		Citations: []implctx.Citation{{URI: "project://r/a.md", RootName: "r"}},
		RootsUsed: []string{"r"},
	}, time.Millisecond))
	waitFlush(t, s, 1)
	b, err := s.ExportJSON(context.Background(), Filter{})
	if err != nil || len(b) < 10 {
		t.Fatalf("export json: %v %d", err, len(b))
	}
	csv, err := s.ExportCSV(context.Background(), Filter{})
	if err != nil || len(csv) < 10 {
		t.Fatalf("export csv: %v", err)
	}
	list, err := s.QueryRecentRequests(context.Background(), Filter{Limit: 10, Sort: "tokens", Order: "desc"})
	if err != nil || list.Total != 1 || list.Requests[0].ReturnedTokens != 400 {
		t.Fatalf("list: %v %+v", err, list)
	}
	s.Close()
}

func TestClassifyToolAndCoveragePolicy(t *testing.T) {
	if ClassifyTool("get_implementation_context") != ClassImplementationContext {
		t.Fatal("class")
	}
	if !CoverageApplicableForTool("get_implementation_context") {
		t.Fatal("impl coverage applicable")
	}
	if CoverageApplicableForTool("get_document") {
		t.Fatal("document coverage not applicable")
	}
}

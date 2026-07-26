// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenSchemaRecordQueryClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "implcache-usage.db")
	s, err := Open(path, Config{Enabled: true, RetentionDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	st := s.Status(context.Background())
	if !st.Available || !st.Enabled || !st.LocalOnly {
		t.Fatalf("status: %+v", st)
	}

	ev := RequestEvent{
		RequestID:     "req-1",
		OccurredAt:    time.Now().UTC(),
		ToolName:      "get_implementation_context",
		TaskHash:      HashTask("demo task"),
		ResultStatus:  StatusGroundedLocal,
		Coverage:      "high",
		CitationCount: 1,
		Roots:         []RootRef{{RootKey: "sdk", RootName: "sdk", Selected: true}},
		Evidence: []EvidenceEvent{{
			EvidenceType: EvidenceCitation, EvidenceKey: "project://sdk/a.md",
			RootKey: "sdk", SelectedForPackage: true, IncludedAfterTrimming: true,
		}},
	}
	s.Record(ev)
	deadline := time.Now().Add(3 * time.Second)
	for {
		var n int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM request_events`).Scan(&n)
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("event not flushed")
		}
		time.Sleep(50 * time.Millisecond)
	}

	sum, err := s.QuerySummary(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalRequests != 1 || sum.GroundedRequests != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	ts, err := s.QueryTimeseries(context.Background(), Filter{Bucket: "day"})
	if err != nil || len(ts) == 0 {
		t.Fatalf("timeseries: %v %+v", err, ts)
	}
	g, err := s.QueryGrounding(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if g.Outcomes.Total != 1 || g.Outcomes.Local != 1 {
		t.Fatalf("grounding: %+v", g)
	}
	cov, err := s.QueryCoverageBreakdown(context.Background(), Filter{})
	if err != nil || cov.High != 1 {
		t.Fatalf("coverage: %v %+v", err, cov)
	}
	recent, err := s.QueryRecentRequests(context.Background(), Filter{Limit: 10})
	if err != nil || len(recent.Requests) != 1 {
		t.Fatalf("recent: %v %+v", err, recent)
	}
	detail, err := s.QueryRequestDetail(context.Background(), "req-1")
	if err != nil || detail == nil || detail.CitationCount != 1 {
		t.Fatalf("detail: %v %+v", err, detail)
	}

	if err := s.ClearAll(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM request_events`).Scan(&n)
	if n != 0 {
		t.Fatalf("expected empty after clear, got %d", n)
	}
}

func TestWriterDropOnFullQueue(t *testing.T) {
	st := &Store{}
	w := &asyncWriter{
		store: st,
		ch:    make(chan RequestEvent, 1),
		done:  make(chan struct{}),
	}
	w.enqueue(RequestEvent{RequestID: "1", ToolName: "t", ResultStatus: StatusNoLocalMatch})
	w.enqueue(RequestEvent{RequestID: "2", ToolName: "t", ResultStatus: StatusNoLocalMatch})
	if st.drops.Load() != 1 {
		t.Fatalf("expected 1 drop, got %d", st.drops.Load())
	}
}

func TestCLIDisabledNoDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "x.db"), Config{CLIDisabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Enabled() || s.db != nil {
		t.Fatal("expected disabled store without db")
	}
	s.Record(RequestEvent{RequestID: "x", ToolName: "t", ResultStatus: StatusNoLocalMatch})
}

func TestDefaultUsageDBPath(t *testing.T) {
	got := DefaultUsageDBPath(`D:\data\implcache.db`)
	want := filepath.Join(`D:\data`, "implcache-usage.db")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNilStoreSafe(t *testing.T) {
	var s *Store
	s.Record(RequestEvent{RequestID: "a", ToolName: "t", ResultStatus: StatusNoLocalMatch})
	_ = s.Status(context.Background())
	if s.Enabled() {
		t.Fatal("nil should be disabled")
	}
}

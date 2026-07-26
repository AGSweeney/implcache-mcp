// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ExportAggregate is the aggregate analytics export payload (no sensitive diagnostics).
type ExportAggregate struct {
	ExportedAt  string            `json:"exportedAt"`
	Filters     ExportFilters     `json:"filters"`
	Summary     Summary           `json:"summary"`
	Timeseries  []TimePoint       `json:"timeseries"`
	Coverage    CoverageBreakdown `json:"coverage"`
	Outcomes    OutcomeBreakdown  `json:"outcomes"`
	Evidence    EvidenceUsage     `json:"evidence"`
	Efficiency  EfficiencyReport  `json:"efficiency"`
	Knowledge   KnowledgeReport   `json:"knowledge"`
}

// ExportFilters records active filters in the export.
type ExportFilters struct {
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Root     string `json:"root,omitempty"`
	Tool     string `json:"tool,omitempty"`
	Coverage string `json:"coverage,omitempty"`
	Status   string `json:"status,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
}

// BuildExport aggregates analytics for export.
func (s *Store) BuildExport(ctx context.Context, f Filter) (ExportAggregate, error) {
	var out ExportAggregate
	out.ExportedAt = time.Now().UTC().Format(time.RFC3339)
	out.Filters = ExportFilters{
		Root: f.Root, Tool: f.Tool, Coverage: f.Coverage, Status: f.Status, Bucket: f.Bucket,
	}
	if !f.From.IsZero() {
		out.Filters.From = f.From.UTC().Format(time.RFC3339)
	}
	if !f.To.IsZero() {
		out.Filters.To = f.To.UTC().Format(time.RFC3339)
	}
	var err error
	if out.Summary, err = s.QuerySummary(ctx, f); err != nil {
		return out, err
	}
	if out.Timeseries, err = s.QueryTimeseries(ctx, f); err != nil {
		return out, err
	}
	if out.Coverage, err = s.QueryCoverageBreakdown(ctx, f); err != nil {
		return out, err
	}
	g, err := s.QueryGrounding(ctx, f)
	if err != nil {
		return out, err
	}
	out.Outcomes = g.Outcomes
	out.Evidence = g.Evidence
	if out.Efficiency, err = s.QueryEfficiency(ctx, f); err != nil {
		return out, err
	}
	if out.Knowledge, err = s.QueryKnowledge(ctx, f); err != nil {
		return out, err
	}
	return out, nil
}

// ExportJSON returns aggregate export as JSON bytes.
func (s *Store) ExportJSON(ctx context.Context, f Filter) ([]byte, error) {
	agg, err := s.BuildExport(ctx, f)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(agg, "", "  ")
}

// ExportCSV returns a flat summary+timeseries CSV.
func (s *Store) ExportCSV(ctx context.Context, f Filter) ([]byte, error) {
	agg, err := s.BuildExport(ctx, f)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"section", "key", "value"})
	writeKV := func(section, key string, val any) {
		_ = w.Write([]string{section, key, fmt.Sprint(val)})
	}
	writeKV("meta", "exportedAt", agg.ExportedAt)
	writeKV("summary", "totalRequests", agg.Summary.TotalRequests)
	writeKV("summary", "groundedRequests", agg.Summary.GroundedRequests)
	writeKV("summary", "localContextTokensServed", agg.Summary.LocalContextTokensServed)
	writeKV("summary", "rootSelectionRequired", agg.Summary.RootSelectionRequired)
	writeKV("summary", "noLocalMatch", agg.Summary.NoLocalMatch)
	writeKV("summary", "localInsufficient", agg.Summary.LocalInsufficient)
	writeKV("summary", "errors", agg.Summary.Errors)
	if agg.Summary.AvgPackageTokens != nil {
		writeKV("summary", "avgPackageTokens", strconv.FormatFloat(*agg.Summary.AvgPackageTokens, 'f', 2, 64))
	}
	writeKV("coverage", "high", agg.Coverage.High)
	writeKV("coverage", "medium", agg.Coverage.Medium)
	writeKV("coverage", "low", agg.Coverage.Low)
	writeKV("coverage", "unclassified", agg.Coverage.Unclassified)
	writeKV("coverage", "notApplicable", agg.Coverage.NotApplicable)
	for _, p := range agg.Timeseries {
		writeKV("timeseries", p.Bucket+".total", p.Total)
		writeKV("timeseries", p.Bucket+".grounded", p.Grounded)
		writeKV("timeseries", p.Bucket+".tokensServed", p.TokensServed)
	}
	if agg.Efficiency.LocalContextTokensServed != nil {
		writeKV("efficiency", "localContextTokensServed", *agg.Efficiency.LocalContextTokensServed)
	}
	for _, b := range agg.Efficiency.SourceTypeBreakdown {
		writeKV("efficiency.sourceType", b.Type, b.Tokens)
	}
	for _, r := range agg.Knowledge.Roots {
		writeKV("knowledge.roots", r.Key, r.TimesSelected)
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

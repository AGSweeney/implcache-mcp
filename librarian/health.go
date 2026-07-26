// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"context"
	"fmt"
	"strings"

	"implcache-mcp/store"
)

// HealthIssue is one actionable library health finding.
type HealthIssue struct {
	Severity    string `json:"severity"` // info|warning|error
	Code        string `json:"code"`
	SourceKind  string `json:"sourceKind,omitempty"`
	SourceID    string `json:"sourceId,omitempty"`
	Description string `json:"description"`
	Action      string `json:"recommendedAction,omitempty"`
}

// LibraryHealth runs Stage-4 style library checks.
func LibraryHealth(ctx context.Context, st *store.Store) ([]HealthIssue, error) {
	var issues []HealthIssue

	v, err := st.SchemaVersion(ctx)
	if err != nil {
		issues = append(issues, HealthIssue{
			Severity: "error", Code: "schema_read_failed",
			Description: err.Error(), Action: "Inspect database file and reopen",
		})
	} else if v != 11 {
		issues = append(issues, HealthIssue{
			Severity: "error", Code: "schema_mismatch",
			Description: "schema version is not 11", Action: "Delete DB and re-ingest",
		})
	}

	if report, err := st.DocumentsWithoutChunksReport(ctx, 8); err != nil {
		issues = append(issues, HealthIssue{
			Severity: "warning", Code: "documents_without_chunks",
			SourceKind: "library", SourceID: "all",
			Description: "failed to inspect chunkless documents: " + err.Error(),
			Action:      "Check database integrity",
		})
	} else if report.Total > 0 {
		issues = append(issues, emptyChunksIssue(report))
	}

	sources, err := ListSources(ctx, st)
	if err != nil {
		return issues, err
	}
	for _, src := range sources {
		stt := classifyState(src.LastStatus)
		if stt == "failed" {
			issues = append(issues, HealthIssue{
				Severity: "error", Code: "source_failed",
				SourceKind: string(src.Kind), SourceID: src.ID,
				Description: "source lastStatus=" + src.LastStatus,
				Action:      "Inspect errors and refresh or remove",
			})
		} else if stt == "degraded" || strings.HasPrefix(strings.ToLower(src.LastStatus), "partial") {
			issues = append(issues, HealthIssue{
				Severity: "warning", Code: "source_degraded",
				SourceKind: string(src.Kind), SourceID: src.ID,
				Description: "source lastStatus=" + src.LastStatus,
				Action:      "Review recent errors and refresh",
			})
		}
		if src.DocumentCount == 0 && src.Kind != KindLocal {
			issues = append(issues, HealthIssue{
				Severity: "info", Code: "source_empty",
				SourceKind: string(src.Kind), SourceID: src.ID,
				Description: "source has zero documents", Action: "Run ingest",
			})
		}
	}
	return issues, nil
}

func emptyChunksIssue(report store.DocumentsWithoutChunksReport) HealthIssue {
	rootParts := make([]string, 0, len(report.ByRoot))
	rootNames := make([]string, 0, len(report.ByRoot))
	for _, r := range report.ByRoot {
		label := r.RootName
		if label == "" {
			label = "(empty root)"
		}
		if r.SourceType != "" {
			rootParts = append(rootParts, fmt.Sprintf("%s [%s]: %d", label, r.SourceType, r.Count))
		} else {
			rootParts = append(rootParts, fmt.Sprintf("%s: %d", label, r.Count))
		}
		if r.RootName != "" {
			rootNames = append(rootNames, r.RootName)
		}
	}

	desc := fmt.Sprintf("%d document(s) have no chunks", report.Total)
	if len(rootParts) > 0 {
		desc += " — by root: " + strings.Join(rootParts, "; ")
	}
	if len(report.SampleURIs) > 0 {
		desc += ". Examples: " + strings.Join(report.SampleURIs, ", ")
		if report.Total > len(report.SampleURIs) {
			desc += ", …"
		}
	}

	action := "Re-ingest or remove the listed roots (Library → filter by root, or Sources → Refresh / Remove)."
	if len(rootNames) > 0 {
		uniq := uniqueStrings(rootNames)
		action = "Purge stub docs from Librarian Health (Purge chunkless docs), " +
			"or: ingestcli -mode purge-empty-docs -db <db> " +
			"(roots: " + strings.Join(uniq, ", ") + ")."
	}

	primary := "all"
	if len(rootNames) == 1 {
		primary = rootNames[0]
	}

	return HealthIssue{
		Severity:    "warning",
		Code:        "documents_without_chunks",
		SourceKind:  "library",
		SourceID:    primary,
		Description: desc,
		Action:      action,
	}
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

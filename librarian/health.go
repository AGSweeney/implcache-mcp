// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"context"
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

	emptyDocs, _ := st.CountDocumentsWithoutChunks(ctx)
	if emptyDocs > 0 {
		issues = append(issues, HealthIssue{
			Severity: "warning", Code: "documents_without_chunks",
			Description: "documents exist with no chunks", Action: "Re-ingest affected sources",
		})
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

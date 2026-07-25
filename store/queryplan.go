// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"strings"
)

// QueryPlanStep is one row from EXPLAIN QUERY PLAN.
type QueryPlanStep struct {
	ID      int    `json:"id"`
	Parent  int    `json:"parent"`
	NotUsed int    `json:"-"`
	Detail  string `json:"detail"`
}

// ExplainSearchPlan returns EXPLAIN QUERY PLAN for a root-scoped FTS search.
func (s *Store) ExplainSearchPlan(ctx context.Context, query string, roots []string) ([]QueryPlanStep, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	ftsQuery := toFTSQuery(query)
	if ftsQuery == "" {
		return nil, fmt.Errorf("query produced no searchable terms")
	}

	sqlText := `
		EXPLAIN QUERY PLAN
		SELECT c.id
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN documents d ON d.id = c.document_id
		WHERE chunks_fts MATCH ?`
	args := []any{ftsQuery}
	if len(roots) > 0 {
		ph := make([]string, len(roots))
		for i, r := range roots {
			ph[i] = "?"
			args = append(args, r)
		}
		sqlText += `
		  AND (c.root_name IN (` + strings.Join(ph, ",") + `)
		       OR (c.root_name = '' AND d.root_name IN (` + strings.Join(ph, ",") + `)))`
		args = append(args, rootsToAny(roots)...)
	}
	sqlText += ` ORDER BY bm25(chunks_fts, 10.0, 1.0) LIMIT 20`

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueryPlanStep
	for rows.Next() {
		var step QueryPlanStep
		if err := rows.Scan(&step.ID, &step.Parent, &step.NotUsed, &step.Detail); err != nil {
			return nil, err
		}
		out = append(out, step)
	}
	return out, rows.Err()
}

func rootsToAny(roots []string) []any {
	out := make([]any, len(roots))
	for i, r := range roots {
		out[i] = r
	}
	return out
}

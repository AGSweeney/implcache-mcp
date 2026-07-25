// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// Symbol is a stored code symbol.
type Symbol struct {
	ID         int64  `json:"id"`
	DocumentID int64  `json:"documentId"`
	RootName   string `json:"rootName"`
	Name       string `json:"name"`
	NameNorm   string `json:"nameNorm"`
	Kind       string `json:"kind"`
	Language   string `json:"language"`
	Signature  string `json:"signature"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	URI        string `json:"uri,omitempty"`
	Title      string `json:"title,omitempty"`
	Authority  string `json:"authority,omitempty"`
}

// NormalizeSymbol lowercases and strips trivial call punctuation for lookup.
func NormalizeSymbol(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "()")
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// FindSymbols looks up symbols by exact normalized name within optional roots.
func (s *Store) FindSymbols(ctx context.Context, name string, roots []string, limit int) ([]Symbol, error) {
	norm := NormalizeSymbol(name)
	if norm == "" {
		return nil, fmt.Errorf("symbol name is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := `
		SELECT s.id, s.document_id, s.root_name, s.name, s.name_norm, s.kind, s.language,
		       s.signature, s.start_line, s.end_line,
		       d.uri, d.title, COALESCE(d.authority, 'unknown')
		FROM symbols s
		JOIN documents d ON d.id = s.document_id
		WHERE s.name_norm = ?`
	args := []any{norm}
	if len(roots) > 0 {
		ph := make([]string, len(roots))
		for i, r := range roots {
			ph[i] = "?"
			args = append(args, r)
		}
		q += ` AND s.root_name IN (` + strings.Join(ph, ",") + `)`
	}
	q += ` ORDER BY CASE d.authority
		WHEN 'current_project' THEN 0
		WHEN 'related_internal_project' THEN 1
		WHEN 'curated_internal_recipe' THEN 2
		WHEN 'official_example' THEN 3
		WHEN 'official_documentation' THEN 4
		ELSE 9 END, s.start_line
		LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(
			&sym.ID, &sym.DocumentID, &sym.RootName, &sym.Name, &sym.NameNorm, &sym.Kind, &sym.Language,
			&sym.Signature, &sym.StartLine, &sym.EndLine, &sym.URI, &sym.Title, &sym.Authority,
		); err != nil {
			return nil, err
		}
		out = append(out, sym)
	}
	return out, rows.Err()
}

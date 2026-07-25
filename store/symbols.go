// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Match type labels for symbol lookup (deterministic retrieval stages).
const (
	MatchExactCanonical   = "exact"
	MatchExactNormalized  = "exact_normalized"
	MatchExactQualified   = "exact_qualified"
	MatchExactUnqualified = "exact_unqualified"
	MatchPrefix           = "prefix"
	MatchSuffix           = "suffix"
	MatchToken            = "token"
	MatchFuzzy            = "fuzzy"
)

// Symbol is a stored code symbol.
type Symbol struct {
	ID              int64   `json:"id"`
	DocumentID      int64   `json:"documentId"`
	RootName        string  `json:"rootName"`
	Name            string  `json:"name"`
	QualifiedName   string  `json:"qualifiedName,omitempty"`
	UnqualifiedName string  `json:"unqualifiedName,omitempty"`
	NameNorm        string  `json:"nameNorm"`
	Kind            string  `json:"kind,omitempty"`
	Language        string  `json:"language,omitempty"`
	Namespace       string  `json:"namespace,omitempty"`
	Signature       string  `json:"signature,omitempty"`
	SignatureNorm   string  `json:"signatureNorm,omitempty"`
	StartLine       int     `json:"startLine,omitempty"`
	EndLine         int     `json:"endLine,omitempty"`
	URI             string  `json:"uri,omitempty"`
	Title           string  `json:"title,omitempty"`
	Authority       string  `json:"authority,omitempty"`
	MatchType       string  `json:"matchType,omitempty"`
	Confidence      float64 `json:"confidence,omitempty"`
}

// SymbolForms holds searchable forms derived from a displayed symbol name.
type SymbolForms struct {
	Name            string
	QualifiedName   string
	UnqualifiedName string
	Namespace       string
	NameNorm        string
	SignatureNorm   string
}

// DeriveSymbolForms preserves the original spelling and adds alternate lookup keys.
func DeriveSymbolForms(name, signature string) SymbolForms {
	name = strings.TrimSpace(name)
	sig := strings.TrimSpace(signature)
	qual := name
	unqual := UnqualifiedSymbol(name)
	ns := NamespaceOfSymbol(name)
	return SymbolForms{
		Name:            name,
		QualifiedName:   qual,
		UnqualifiedName: unqual,
		Namespace:       ns,
		NameNorm:        NormalizeSymbol(name),
		SignatureNorm:   NormalizeSymbol(sig),
	}
}

// NormalizeSymbol lowercases and strips whitespace / trailing () for lookup keys.
// It does not remove :: . _ # <> [] punctuation that distinguishes symbols.
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

// UnqualifiedSymbol returns the rightmost identifier segment.
func UnqualifiedSymbol(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "()")
	if name == "" {
		return ""
	}
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[i+2:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	if i := strings.LastIndex(name, "#"); i >= 0 {
		return name[i+1:]
	}
	return name
}

// NamespaceOfSymbol returns the qualifier prefix, if any.
func NamespaceOfSymbol(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "()")
	if i := strings.LastIndex(name, "::"); i >= 0 {
		return name[:i]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[:i]
	}
	if i := strings.LastIndex(name, "#"); i >= 0 {
		return name[:i]
	}
	return ""
}

// FindSymbols looks up symbols with staged exact → qualified → prefix/suffix matching.
func (s *Store) FindSymbols(ctx context.Context, name string, roots []string, limit int) ([]Symbol, error) {
	raw := strings.TrimSpace(name)
	norm := NormalizeSymbol(raw)
	if norm == "" {
		return nil, fmt.Errorf("symbol name is required")
	}
	limit = ClampSearchLimit(limit, DefaultSearchLimit)
	unqual := NormalizeSymbol(UnqualifiedSymbol(raw))
	qualNorm := NormalizeSymbol(raw)

	type stage struct {
		matchType  string
		confidence float64
		where      string
		arg        string
	}
	stages := []stage{
		{MatchExactNormalized, 1.0, `s.name_norm = ?`, norm},
		{MatchExactQualified, 0.98, `LOWER(s.qualified_name) = ? OR s.name_norm = ?`, qualNorm},
		{MatchExactUnqualified, 0.92, `LOWER(s.unqualified_name) = ? OR s.name_norm = ?`, unqual},
		{MatchPrefix, 0.75, `s.name_norm LIKE ? ESCAPE '\'`, escapeLike(norm) + `%`},
		{MatchSuffix, 0.7, `s.name_norm LIKE ? ESCAPE '\'`, `%` + escapeLike(unqual)},
		{MatchToken, 0.55, `s.name_norm LIKE ? ESCAPE '\'`, `%` + escapeLike(unqual) + `%`},
	}

	seen := map[int64]struct{}{}
	var out []Symbol
	for _, stg := range stages {
		if len(out) >= limit {
			break
		}
		// Skip redundant stages when query has no qualifier.
		if stg.matchType == MatchExactQualified && NamespaceOfSymbol(raw) == "" && !strings.ContainsAny(raw, ".#") {
			continue
		}
		if (stg.matchType == MatchSuffix || stg.matchType == MatchToken) && len(unqual) < 3 {
			continue
		}
		remain := limit - len(out)
		syms, err := s.querySymbols(ctx, stg.where, stg.arg, roots, remain*2, stg.matchType, stg.confidence)
		if err != nil {
			return nil, err
		}
		for _, sym := range syms {
			if _, ok := seen[sym.ID]; ok {
				continue
			}
			// Prefix/suffix/token: require the unqualified needle to appear.
			if stg.matchType == MatchPrefix || stg.matchType == MatchSuffix || stg.matchType == MatchToken {
				if !strings.Contains(sym.NameNorm, unqual) && sym.NameNorm != norm {
					continue
				}
			}
			if stg.matchType == MatchExactNormalized {
				if sym.Name == raw {
					if NamespaceOfSymbol(raw) != "" || strings.ContainsAny(raw, ".#") {
						sym.MatchType = MatchExactQualified
						sym.Confidence = 0.98
					} else {
						sym.MatchType = MatchExactCanonical
						sym.Confidence = 1.0
					}
				}
			}
			seen[sym.ID] = struct{}{}
			out = append(out, sym)
			if len(out) >= limit {
				break
			}
		}
	}
	if len(out) < limit && len(unqual) >= 4 {
		fuzzy, err := s.fuzzySymbols(ctx, unqual, roots, limit*3, seen)
		if err != nil {
			return nil, err
		}
		for _, sym := range fuzzy {
			if _, ok := seen[sym.ID]; ok {
				continue
			}
			seen[sym.ID] = struct{}{}
			out = append(out, sym)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *Store) fuzzySymbols(ctx context.Context, unqual string, roots []string, limit int, seen map[int64]struct{}) ([]Symbol, error) {
	prefix := unqual
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	where := `s.name_norm LIKE ? ESCAPE '\'`
	arg := escapeLike(prefix) + `%`
	cands, err := s.querySymbols(ctx, where, arg, roots, 50, MatchToken, 0.4)
	if err != nil {
		return nil, err
	}
	var out []Symbol
	for _, sym := range cands {
		if _, ok := seen[sym.ID]; ok {
			continue
		}
		d := levenshtein(unqual, NormalizeSymbol(sym.UnqualifiedName))
		if d == 0 || d > 2 {
			continue
		}
		sym.MatchType = MatchFuzzy
		sym.Confidence = 0.45 - 0.1*float64(d)
		if sym.Confidence < 0.25 {
			sym.Confidence = 0.25
		}
		out = append(out, sym)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = del
			if ins < cur[j] {
				cur[j] = ins
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func (s *Store) querySymbols(ctx context.Context, where, arg string, roots []string, limit int, matchType string, conf float64) ([]Symbol, error) {
	// Special-case exact_qualified / exact_unqualified which use two placeholders.
	var q string
	var args []any
	switch matchType {
	case MatchExactQualified, MatchExactUnqualified:
		q = `
		SELECT s.id, s.document_id, s.root_name, s.name, COALESCE(s.qualified_name, ''),
		       COALESCE(s.unqualified_name, ''), s.name_norm, s.kind, s.language,
		       COALESCE(s.namespace, ''), s.signature, COALESCE(s.signature_norm, ''),
		       s.start_line, s.end_line,
		       d.uri, d.title, COALESCE(d.authority, 'unknown')
		FROM symbols s
		JOIN documents d ON d.id = s.document_id
		WHERE (` + where + `)`
		args = []any{arg, arg}
	default:
		q = `
		SELECT s.id, s.document_id, s.root_name, s.name, COALESCE(s.qualified_name, ''),
		       COALESCE(s.unqualified_name, ''), s.name_norm, s.kind, s.language,
		       COALESCE(s.namespace, ''), s.signature, COALESCE(s.signature_norm, ''),
		       s.start_line, s.end_line,
		       d.uri, d.title, COALESCE(d.authority, 'unknown')
		FROM symbols s
		JOIN documents d ON d.id = s.document_id
		WHERE ` + where
		args = []any{arg}
	}
	if len(roots) > 0 {
		ph := make([]string, len(roots))
		for i, r := range roots {
			ph[i] = "?"
			args = append(args, r)
		}
		q += ` AND s.root_name IN (` + strings.Join(ph, ",") + `)`
	}
	q += ` ORDER BY CASE s.kind
		WHEN 'function' THEN 0
		WHEN 'method' THEN 1
		WHEN 'declaration' THEN 2
		WHEN 'type' THEN 3
		WHEN 'macro' THEN 3
		WHEN 'constant' THEN 4
		WHEN 'call' THEN 8
		ELSE 5 END,
		CASE d.authority
		WHEN 'current_project' THEN 0
		WHEN 'related_internal_project' THEN 1
		WHEN 'curated_internal_recipe' THEN 2
		WHEN 'official_example' THEN 3
		WHEN 'official_documentation' THEN 4
		ELSE 9 END, COALESCE(d.archived, 0), s.start_line
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
			&sym.ID, &sym.DocumentID, &sym.RootName, &sym.Name, &sym.QualifiedName,
			&sym.UnqualifiedName, &sym.NameNorm, &sym.Kind, &sym.Language,
			&sym.Namespace, &sym.Signature, &sym.SignatureNorm,
			&sym.StartLine, &sym.EndLine, &sym.URI, &sym.Title, &sym.Authority,
		); err != nil {
			return nil, err
		}
		sym.MatchType = matchType
		sym.Confidence = conf
		out = append(out, sym)
	}
	return out, rows.Err()
}

// BasenamePath returns the final path segment using slash or OS separators.
func BasenamePath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	return filepath.Base(p)
}

// ListSymbolsByDocumentIDs returns symbols for the given documents, definitions first.
func (s *Store) ListSymbolsByDocumentIDs(ctx context.Context, docIDs []int64, limit int) ([]Symbol, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	limit = ClampSearchLimit(limit, 40)
	ph := make([]string, len(docIDs))
	args := make([]any, 0, len(docIDs)+1)
	for i, id := range docIDs {
		ph[i] = "?"
		args = append(args, id)
	}
	args = append(args, limit)
	q := `
		SELECT s.id, s.document_id, s.root_name, s.name, COALESCE(s.qualified_name, ''),
		       COALESCE(s.unqualified_name, ''), s.name_norm, s.kind, s.language,
		       COALESCE(s.namespace, ''), s.signature, COALESCE(s.signature_norm, ''),
		       s.start_line, s.end_line,
		       d.uri, d.title, COALESCE(d.authority, 'unknown')
		FROM symbols s
		JOIN documents d ON d.id = s.document_id
		WHERE s.document_id IN (` + strings.Join(ph, ",") + `)
		ORDER BY CASE s.kind
			WHEN 'function' THEN 0 WHEN 'method' THEN 1 WHEN 'declaration' THEN 2
			WHEN 'type' THEN 3 WHEN 'macro' THEN 3 WHEN 'constant' THEN 4
			WHEN 'call' THEN 8 ELSE 5 END,
			CASE d.authority
			WHEN 'current_project' THEN 0 WHEN 'related_internal_project' THEN 1
			WHEN 'curated_internal_recipe' THEN 2 WHEN 'official_example' THEN 3
			WHEN 'official_documentation' THEN 4 ELSE 9 END,
			s.start_line
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Symbol
	for rows.Next() {
		var sym Symbol
		if err := rows.Scan(
			&sym.ID, &sym.DocumentID, &sym.RootName, &sym.Name, &sym.QualifiedName,
			&sym.UnqualifiedName, &sym.NameNorm, &sym.Kind, &sym.Language,
			&sym.Namespace, &sym.Signature, &sym.SignatureNorm,
			&sym.StartLine, &sym.EndLine, &sym.URI, &sym.Title, &sym.Authority,
		); err != nil {
			return nil, err
		}
		sym.MatchType = MatchToken
		sym.Confidence = 0.6
		out = append(out, sym)
	}
	return out, rows.Err()
}

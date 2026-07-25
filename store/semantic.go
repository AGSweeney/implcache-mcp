// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// MatchKindSemantic marks hits found via optional term-vector similarity.
const MatchKindSemantic = "semantic"

const maxTermVectorTerms = 48
const (
	minSemanticCandidates     = 1000
	semanticCandidateMultiple = 50
)

// BuildTermVector returns a deterministic top-N sparse term-presence vector.
// Term frequency selects the top terms, but each selected term is serialized
// once; scoring is therefore normalized set overlap, not TF-IDF.
func BuildTermVector(heading, body string) string {
	tf := map[string]int{}
	addText := func(s string) {
		for _, tok := range tokenizeSemantic(s) {
			tf[tok]++
		}
	}
	addText(heading)
	addText(body)
	if len(tf) == 0 {
		return ""
	}
	type pair struct {
		t string
		n int
	}
	var pairs []pair
	for t, n := range tf {
		pairs = append(pairs, pair{t, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].n == pairs[j].n {
			return pairs[i].t < pairs[j].t
		}
		return pairs[i].n > pairs[j].n
	})
	if len(pairs) > maxTermVectorTerms {
		pairs = pairs[:maxTermVectorTerms]
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].t < pairs[j].t })
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.t
	}
	return strings.Join(out, " ")
}

// tokenizeSemantic splits text into normalized terms. Identifiers keep their
// combined lowercase form and additionally emit component tokens split on
// camelCase/PascalCase boundaries and underscores (case boundaries are
// detected before lowercasing). Namespace (::) and member (.) separators are
// non-identifier runes, so qualified names split into their parts naturally.
// Tokens shorter than 3 runes and stopwords are dropped. Output order is
// deterministic. This is intentionally distinct from symbol normalization
// (NormalizeSymbol), which preserves qualification for exact lookup.
func tokenizeSemantic(s string) []string {
	var out []string
	var b strings.Builder
	emit := func(tok string) {
		if len(tok) < 3 || isSemanticStop(tok) {
			return
		}
		out = append(out, tok)
	}
	flush := func() {
		if b.Len() == 0 {
			return
		}
		raw := b.String()
		b.Reset()
		combined := strings.ToLower(raw)
		emit(combined)
		parts := splitIdentifierParts(raw)
		if len(parts) < 2 {
			return
		}
		for _, p := range parts {
			p = strings.ToLower(p)
			if p != combined {
				emit(p)
			}
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// splitIdentifierParts splits an identifier on underscores and camelCase /
// PascalCase boundaries, preserving acronym runs (HTTPServer → HTTP, Server).
func splitIdentifierParts(tok string) []string {
	var parts []string
	runes := []rune(tok)
	start := 0
	flushPart := func(end int) {
		if end > start {
			parts = append(parts, string(runes[start:end]))
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '_' {
			flushPart(i)
			start = i + 1
			continue
		}
		if i == start || !unicode.IsUpper(r) {
			continue
		}
		prev := runes[i-1]
		switch {
		case unicode.IsLower(prev) || unicode.IsDigit(prev):
			// lower/digit → Upper: networkClient → network | Client
			flushPart(i)
			start = i
		case unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]):
			// Acronym followed by a word: HTTPServer → HTTP | Server
			flushPart(i)
			start = i
		}
	}
	flushPart(len(runes))
	return parts
}

func isSemanticStop(t string) bool {
	switch t {
	case "the", "and", "for", "with", "that", "this", "from", "into", "your",
		"are", "was", "were", "been", "have", "has", "had", "not", "but", "you",
		"all", "any", "can", "may", "use", "using", "used", "via", "when", "then",
		"than", "also", "such", "only", "over", "after", "before", "about",
		"void", "null", "true", "false", "return", "func", "function", "class",
		"const", "var", "let", "int", "string", "bool", "error", "nil":
		return true
	}
	return false
}

func termSet(vec string) map[string]float64 {
	out := map[string]float64{}
	for _, t := range strings.Fields(vec) {
		out[t]++
	}
	// L2 normalize
	var sum float64
	for _, v := range out {
		sum += v * v
	}
	if sum == 0 {
		return out
	}
	norm := math.Sqrt(sum)
	for k, v := range out {
		out[k] = v / norm
	}
	return out
}

func cosineSparse(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot float64
	for k, av := range a {
		if bv, ok := b[k]; ok {
			dot += av * bv
		}
	}
	return dot
}

func (s *Store) upsertChunkTermVector(ctx context.Context, tx *sql.Tx, chunkID int64, rootName, heading, body string) error {
	terms := BuildTermVector(heading, body)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO chunk_term_vectors(chunk_id, terms, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET terms = excluded.terms, updated_at = excluded.updated_at`,
		chunkID, terms, time.Now().Unix(),
	)
	if err != nil {
		return err
	}
	return upsertChunkTermPostingsTx(ctx, tx, chunkID, rootName, terms)
}

// TermVectorCount returns the number of persisted vectors. It is useful for
// local integrity checks after ingestion or migration.
func (s *Store) TermVectorCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunk_term_vectors`).Scan(&n)
	return n, err
}

// TermPostingCount returns the number of indexed semantic terms. It is useful
// for local integrity checks after ingestion or migration.
func (s *Store) TermPostingCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunk_term_postings`).Scan(&n)
	return n, err
}

func upsertChunkTermPostingsTx(ctx context.Context, tx *sql.Tx, chunkID int64, rootName, terms string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_term_postings WHERE chunk_id = ?`, chunkID); err != nil {
		return err
	}
	if terms == "" {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunk_term_postings(chunk_id, root_name, term)
		VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, term := range strings.Fields(terms) {
		if _, err := stmt.ExecContext(ctx, chunkID, rootName, term); err != nil {
			return err
		}
	}
	return nil
}

// semanticCandidates returns related chunks by sparse term-presence cosine similarity.
// Used only when SearchOptions.Semantic is true — supplements FTS, does not replace it.
func (s *Store) semanticCandidates(ctx context.Context, query string, roots []string, limit int) ([]SearchHit, error) {
	qVec := termSet(BuildTermVector("", query))
	if len(qVec) == 0 {
		return nil, nil
	}
	qTerms := make([]string, 0, len(qVec))
	for term := range qVec {
		qTerms = append(qTerms, term)
	}
	sort.Strings(qTerms)
	if len(qTerms) == 0 {
		return nil, nil
	}
	candidateLimit := limit * semanticCandidateMultiple
	if candidateLimit < minSemanticCandidates {
		candidateLimit = minSemanticCandidates
	}

	placeholders := make([]string, len(qTerms))
	args := make([]any, 0, len(qTerms)+len(roots)+1)
	for i, term := range qTerms {
		placeholders[i] = "?"
		args = append(args, term)
	}
	candidates := `
		SELECT p.chunk_id
		FROM chunk_term_postings p
		WHERE p.term IN (` + strings.Join(placeholders, ",") + `)`
	if len(roots) > 0 {
		rootPlaceholders := make([]string, len(roots))
		for i, root := range roots {
			rootPlaceholders[i] = "?"
			args = append(args, root)
		}
		candidates += ` AND p.root_name IN (` + strings.Join(rootPlaceholders, ",") + `)`
	}
	candidates += `
		GROUP BY p.chunk_id
		ORDER BY COUNT(*) DESC, p.chunk_id
		LIMIT ?`
	args = append(args, candidateLimit)

	sqlText := `
		SELECT c.id, c.document_id, d.uri, d.title,
		       COALESCE(d.root_name, ''), COALESCE(d.path, ''),
		       COALESCE(d.authority, 'unknown'), COALESCE(d.language, ''),
		       COALESCE(d.technology, ''), COALESCE(d.product_version, ''),
		       COALESCE(d.archived, 0),
		       c.ordinal, c.heading, c.body, c.start_line, c.end_line, v.terms
		FROM (` + candidates + `) candidates
		JOIN chunk_term_vectors v ON v.chunk_id = candidates.chunk_id
		JOIN chunks c ON c.id = v.chunk_id
		JOIN documents d ON d.id = c.document_id
		WHERE v.terms != ''`

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		hit   SearchHit
		score float64
	}
	var ranked []scored
	for rows.Next() {
		var h SearchHit
		var archived int
		var terms string
		if err := rows.Scan(
			&h.ChunkID, &h.DocumentID, &h.URI, &h.Title, &h.RootName, &h.Path,
			&h.Authority, &h.Language, &h.Technology, &h.ProductVersion, &archived,
			&h.Ordinal, &h.Heading, &h.Snippet, &h.StartLine, &h.EndLine, &terms,
		); err != nil {
			return nil, err
		}
		h.Archived = archived != 0
		sim := cosineSparse(qVec, termSet(terms))
		if sim < 0.12 {
			continue
		}
		h.MatchKind = MatchKindSemantic
		h.Score = sim*2.0 + AuthorityBoost(h.Authority)
		h.Rank = 1.0 - sim // lower rank = better for sort ties with FTS
		h.Snippet = ClipExcerpt(h.Snippet, 240)
		ranked = append(ranked, scored{h, h.Score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].hit.ChunkID < ranked[j].hit.ChunkID
		}
		return ranked[i].score > ranked[j].score
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]SearchHit, len(ranked))
	for i, r := range ranked {
		out[i] = r.hit
	}
	return out, nil
}

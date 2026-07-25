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

// BuildTermVector returns a space-separated sparse term string for a chunk.
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

func tokenizeSemantic(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := strings.ToLower(b.String())
		b.Reset()
		if len(tok) < 3 || isSemanticStop(tok) {
			return
		}
		out = append(out, tok)
		// Split camelCase / PascalCase leftovers already lowercased — also emit
		// underscore segments.
		if strings.Contains(tok, "_") {
			for _, p := range strings.Split(tok, "_") {
				if len(p) >= 3 && !isSemanticStop(p) {
					out = append(out, p)
				}
			}
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return out
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

func (s *Store) upsertChunkTermVector(ctx context.Context, tx *sql.Tx, chunkID int64, heading, body string) error {
	terms := BuildTermVector(heading, body)
	if terms == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO chunk_term_vectors(chunk_id, terms, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET terms = excluded.terms, updated_at = excluded.updated_at`,
		chunkID, terms, time.Now().Unix(),
	)
	return err
}

// backfillChunkTermVectors populates term vectors for existing chunks (migration v6).
func backfillChunkTermVectors(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, heading, body FROM chunks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		id            int64
		heading, body string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.heading, &r.body); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`
		INSERT INTO chunk_term_vectors(chunk_id, terms, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET terms = excluded.terms, updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for _, r := range list {
		terms := BuildTermVector(r.heading, r.body)
		if terms == "" {
			continue
		}
		if _, err := stmt.Exec(r.id, terms, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// semanticCandidates returns related chunks by sparse term cosine similarity.
// Used only when SearchOptions.Semantic is true — supplements FTS, does not replace it.
func (s *Store) semanticCandidates(ctx context.Context, query string, roots []string, limit int) ([]SearchHit, error) {
	qVec := termSet(BuildTermVector("", query))
	if len(qVec) == 0 {
		return nil, nil
	}
	sqlText := `
		SELECT c.id, c.document_id, d.uri, d.title,
		       COALESCE(d.root_name, ''), COALESCE(d.path, ''),
		       COALESCE(d.authority, 'unknown'), COALESCE(d.language, ''),
		       COALESCE(d.technology, ''), COALESCE(d.product_version, ''),
		       COALESCE(d.archived, 0),
		       c.ordinal, c.heading, c.body, c.start_line, c.end_line, v.terms
		FROM chunk_term_vectors v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN documents d ON d.id = c.document_id
		WHERE v.terms != ''`
	args := []any{}
	if len(roots) > 0 {
		ph := make([]string, len(roots))
		for i, r := range roots {
			ph[i] = "?"
			args = append(args, r)
		}
		sqlText += ` AND (c.root_name IN (` + strings.Join(ph, ",") + `)
		       OR (c.root_name = '' AND d.root_name IN (` + strings.Join(ph, ",") + `)))`
		// Second IN list needs its own bound args.
		for _, r := range roots {
			args = append(args, r)
		}
	}
	// Bound candidate scan: prefer chunks sharing at least one query term via LIKE.
	var qTerms []string
	for t := range qVec {
		if len(t) >= 3 {
			qTerms = append(qTerms, t)
		}
	}
	sort.Strings(qTerms)
	var likeParts []string
	for _, t := range qTerms {
		likeParts = append(likeParts, `v.terms LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(t)+"%")
		if len(likeParts) >= 6 {
			break
		}
	}
	if len(likeParts) > 0 {
		sqlText += ` AND (` + strings.Join(likeParts, " OR ") + `)`
	}
	sqlText += ` LIMIT 200`

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

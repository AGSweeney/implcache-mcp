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
	// Semantic candidate pool: resultLimit * multiple, clamped to [min, max].
	// Bodies are hydrated only for the final resultLimit hits after scoring.
	minSemanticCandidates     = 250
	maxSemanticCandidates     = 1500
	semanticCandidateMultiple = 25
	// maxSemanticQueryTerms bounds the posting IN-list. Extra query terms still
	// participate in final IDF-weighted scoring after candidates are fetched.
	maxSemanticQueryTerms = 16
	postingInsertBatch    = 24
)

// semanticCandidateLimit returns how many posting-selected chunks to score.
func semanticCandidateLimit(resultLimit int) int {
	if resultLimit < 1 {
		resultLimit = DefaultSearchLimit
	}
	n := resultLimit * semanticCandidateMultiple
	if n < minSemanticCandidates {
		n = minSemanticCandidates
	}
	if n > maxSemanticCandidates {
		n = maxSemanticCandidates
	}
	return n
}

// BuildTermVector returns a deterministic top-N sparse term-presence vector.
// Term frequency selects the top terms, but each selected term is serialized
// once. Final scoring applies query-time corpus IDF over that presence set
// (IDF-weighted cosine); it is not classic TF-IDF with per-chunk TF weights,
// embeddings, or a vector database.
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
	return weightedTermSet(vec, nil)
}

// weightedTermSet builds an L2-normalized sparse vector. When idf is non-nil,
// each present term is weighted by its corpus IDF (missing keys use a rare-term
// fallback based on nChunks via idfWeight when df is absent — callers should
// populate every needed term).
func weightedTermSet(vec string, idf map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for _, t := range strings.Fields(vec) {
		w := 1.0
		if idf != nil {
			if iw, ok := idf[t]; ok {
				w = iw
			}
		}
		out[t] += w
	}
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

// idfWeight is smooth IDF: log(1 + N/(df+1)) + 1. Common terms approach ~1;
// rare terms get higher weight. Deterministic and cheap at query time.
func idfWeight(nChunks, df int) float64 {
	if nChunks < 1 {
		nChunks = 1
	}
	if df < 0 {
		df = 0
	}
	return math.Log(1+float64(nChunks)/float64(df+1)) + 1
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
// for local integrity checks after ingestion.
func (s *Store) TermPostingCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunk_term_postings`).Scan(&n)
	return n, err
}

// SemanticIndexStats summarizes posting-table size for operators and tests.
type SemanticIndexStats struct {
	Vectors              int     `json:"vectors"`
	Postings             int     `json:"postings"`
	DistinctTerms        int     `json:"distinctTerms"`
	AvgPostingsPerVector float64 `json:"avgPostingsPerVector"`
}

// SemanticStats returns vector/posting cardinality. Useful for watching
// posting-table growth on large corpora (bounded by maxTermVectorTerms per chunk).
func (s *Store) SemanticStats(ctx context.Context) (SemanticIndexStats, error) {
	var st SemanticIndexStats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunk_term_vectors`).Scan(&st.Vectors); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunk_term_postings`).Scan(&st.Postings); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT term) FROM chunk_term_postings`).Scan(&st.DistinctTerms); err != nil {
		return st, err
	}
	if st.Vectors > 0 {
		st.AvgPostingsPerVector = float64(st.Postings) / float64(st.Vectors)
	}
	return st, nil
}

func upsertChunkTermPostingsTx(ctx context.Context, tx *sql.Tx, chunkID int64, rootName, terms string) error {
	// Retract DF for any existing postings before replace (no-op for new chunks).
	oldRows, err := tx.QueryContext(ctx, `
		SELECT root_name, term FROM chunk_term_postings WHERE chunk_id = ?`, chunkID)
	if err != nil {
		return err
	}
	type oldPosting struct{ root, term string }
	var old []oldPosting
	for oldRows.Next() {
		var p oldPosting
		if err := oldRows.Scan(&p.root, &p.term); err != nil {
			oldRows.Close()
			return err
		}
		old = append(old, p)
	}
	if err := oldRows.Close(); err != nil {
		return err
	}
	if err := oldRows.Err(); err != nil {
		return err
	}
	for _, p := range old {
		if err := adjustTermDFTx(ctx, tx, p.root, p.term, -1); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunk_term_postings WHERE chunk_id = ?`, chunkID); err != nil {
		return err
	}
	termList := strings.Fields(terms)
	if len(termList) == 0 {
		return nil
	}
	for i := 0; i < len(termList); i += postingInsertBatch {
		end := i + postingInsertBatch
		if end > len(termList) {
			end = len(termList)
		}
		batch := termList[i:end]
		placeholders := make([]string, len(batch))
		args := make([]any, 0, len(batch)*3)
		for j, term := range batch {
			placeholders[j] = "(?, ?, ?)"
			args = append(args, chunkID, rootName, term)
		}
		sqlText := `INSERT INTO chunk_term_postings(chunk_id, root_name, term) VALUES ` +
			strings.Join(placeholders, ",")
		if _, err := tx.ExecContext(ctx, sqlText, args...); err != nil {
			return err
		}
	}
	return applyTermsDFDeltaTx(ctx, tx, rootName, terms, 1)
}

// semanticCandidates returns related chunks by IDF-weighted sparse cosine similarity.
// Used only when SearchOptions.Semantic is true — supplements FTS, does not replace it.
// limit is the desired result count (not an already-expanded FTS candidate pool).
func (s *Store) semanticCandidates(ctx context.Context, query string, roots []string, limit int) ([]SearchHit, error) {
	hits, _, err := s.semanticCandidatesDetailed(ctx, query, roots, limit)
	return hits, err
}

// SemanticCandidatesStats runs the semantic candidate path and returns timing /
// cardinality stats for scale evaluation tools.
func (s *Store) SemanticCandidatesStats(ctx context.Context, query string, roots []string, limit int) ([]SearchHit, SemanticSearchStats, error) {
	return s.semanticCandidatesDetailed(ctx, query, roots, limit)
}

// SemanticSearchStats is optional timing/cardinality detail for scale tools.
type SemanticSearchStats struct {
	CandidateLimit int           `json:"candidateLimit"`
	CandidateRows  int           `json:"candidateRows"`
	ScoredRows     int           `json:"scoredRows"`
	Returned       int           `json:"returned"`
	IDF            time.Duration `json:"idfNs"`
	PostingFetch   time.Duration `json:"postingFetchNs"`
	Score          time.Duration `json:"scoreNs"`
	Hydrate        time.Duration `json:"hydrateNs"`
	Total          time.Duration `json:"totalNs"`
}

func (s *Store) semanticCandidatesDetailed(ctx context.Context, query string, roots []string, limit int) (hits []SearchHit, stats SemanticSearchStats, err error) {
	start := time.Now()
	defer func() { stats.Total = time.Since(start) }()

	qTermsAll := strings.Fields(BuildTermVector("", query))
	if len(qTermsAll) == 0 {
		return nil, stats, nil
	}
	sort.Strings(qTermsAll)

	idfStart := time.Now()
	nChunks, df, err := s.termDocumentFrequencies(ctx, qTermsAll, roots)
	stats.IDF = time.Since(idfStart)
	if err != nil {
		return nil, stats, err
	}
	idf := make(map[string]float64, len(qTermsAll))
	for _, term := range qTermsAll {
		idf[term] = idfWeight(nChunks, df[term])
	}
	qVec := weightedTermSet(strings.Join(qTermsAll, " "), idf)
	if len(qVec) == 0 {
		return nil, stats, nil
	}

	lookupTerms := selectLookupTerms(qTermsAll, idf, df, nChunks, maxSemanticQueryTerms)
	candidateLimit := semanticCandidateLimit(limit)
	stats.CandidateLimit = candidateLimit

	placeholders := make([]string, len(lookupTerms))
	args := make([]any, 0, len(lookupTerms)+len(roots)+1)
	for i, term := range lookupTerms {
		placeholders[i] = "?"
		args = append(args, term)
	}
	// Prefer root_name first so SQLite can use idx_chunk_term_postings_root_term.
	candidates := `SELECT p.chunk_id FROM chunk_term_postings p WHERE `
	if len(roots) > 0 {
		rootPlaceholders := make([]string, len(roots))
		rootArgs := make([]any, len(roots))
		for i, root := range roots {
			rootPlaceholders[i] = "?"
			rootArgs[i] = root
		}
		candidates += `p.root_name IN (` + strings.Join(rootPlaceholders, ",") + `) AND `
		args = append(rootArgs, args...)
	}
	candidates += `p.term IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY p.chunk_id
		ORDER BY COUNT(*) DESC, p.chunk_id
		LIMIT ?`
	args = append(args, candidateLimit)

	// Phase 1: fetch only chunk_id + term vectors for scoring (no chunk bodies).
	sqlText := `
		SELECT candidates.chunk_id, v.terms
		FROM (` + candidates + `) candidates
		JOIN chunk_term_vectors v ON v.chunk_id = candidates.chunk_id
		WHERE v.terms != ''`

	phaseStart := time.Now()
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, stats, err
	}
	defer rows.Close()

	var ranked []semanticScored
	var scoreCost time.Duration
	for rows.Next() {
		var chunkID int64
		var terms string
		if err := rows.Scan(&chunkID, &terms); err != nil {
			return nil, stats, err
		}
		stats.CandidateRows++
		simStart := time.Now()
		sim := cosineSparse(qVec, termSet(terms))
		scoreCost += time.Since(simStart)
		if sim < 0.12 {
			continue
		}
		// Authority is applied after hydrate when metadata is available.
		ranked = append(ranked, semanticScored{chunkID: chunkID, sim: sim, score: sim * 2.0})
	}
	if err := rows.Err(); err != nil {
		return nil, stats, err
	}
	stats.PostingFetch = time.Since(phaseStart) - scoreCost
	if stats.PostingFetch < 0 {
		stats.PostingFetch = 0
	}
	stats.Score = scoreCost
	stats.ScoredRows = len(ranked)

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].chunkID < ranked[j].chunkID
		}
		return ranked[i].score > ranked[j].score
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	if len(ranked) == 0 {
		return nil, stats, nil
	}

	hydrateStart := time.Now()
	hits, err = s.hydrateSemanticHits(ctx, ranked)
	stats.Hydrate = time.Since(hydrateStart)
	if err != nil {
		return nil, stats, err
	}
	stats.Returned = len(hits)
	return hits, stats, nil
}

type semanticScored struct {
	chunkID int64
	score   float64
	sim     float64
}

func (s *Store) hydrateSemanticHits(ctx context.Context, ranked []semanticScored) ([]SearchHit, error) {
	ids := make([]any, len(ranked))
	byID := make(map[int64]semanticScored, len(ranked))
	placeholders := make([]string, len(ranked))
	for i, r := range ranked {
		ids[i] = r.chunkID
		byID[r.chunkID] = r
		placeholders[i] = "?"
	}
	sqlText := `
		SELECT c.id, c.document_id, d.uri, d.title,
		       COALESCE(d.root_name, ''), COALESCE(d.path, ''),
		       COALESCE(d.authority, 'unknown'), COALESCE(d.language, ''),
		       COALESCE(d.technology, ''), COALESCE(d.product_version, ''),
		       COALESCE(d.archived, 0),
		       c.ordinal, c.heading, c.body, c.start_line, c.end_line
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, sqlText, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hitsByID := make(map[int64]SearchHit, len(ranked))
	for rows.Next() {
		var h SearchHit
		var archived int
		if err := rows.Scan(
			&h.ChunkID, &h.DocumentID, &h.URI, &h.Title, &h.RootName, &h.Path,
			&h.Authority, &h.Language, &h.Technology, &h.ProductVersion, &archived,
			&h.Ordinal, &h.Heading, &h.Snippet, &h.StartLine, &h.EndLine,
		); err != nil {
			return nil, err
		}
		h.Archived = archived != 0
		r := byID[h.ChunkID]
		h.MatchKind = MatchKindSemantic
		h.Score = r.sim*2.0 + AuthorityBoost(h.Authority)
		h.Rank = 1.0 - r.sim
		h.Snippet = ClipExcerpt(h.Snippet, 240)
		hitsByID[h.ChunkID] = h
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(ranked))
	for _, r := range ranked {
		if h, ok := hitsByID[r.chunkID]; ok {
			out = append(out, h)
		}
	}
	// Re-sort after authority boost may reorder equal-sim ties across authorities.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ChunkID < out[j].ChunkID
		}
		return out[i].Score > out[j].Score
	})
	return out, nil
}

// termDocumentFrequencies returns scoped chunk cardinality and per-term DF
// from persisted term_df / root_chunk_stats (O(query terms), not a postings scan).
func (s *Store) termDocumentFrequencies(ctx context.Context, terms []string, roots []string) (int, map[string]int, error) {
	df := make(map[string]int, len(terms))
	if len(terms) == 0 {
		return 0, df, nil
	}

	var nChunks int
	if len(roots) > 0 {
		rootPlaceholders := make([]string, len(roots))
		args := make([]any, len(roots))
		for i, root := range roots {
			rootPlaceholders[i] = "?"
			args[i] = normalizeRootName(root)
		}
		err := s.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(chunk_count), 0) FROM root_chunk_stats WHERE root_name IN (`+
				strings.Join(rootPlaceholders, ",")+`)`,
			args...,
		).Scan(&nChunks)
		if err != nil {
			return 0, nil, err
		}
	} else {
		if err := s.db.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(chunk_count), 0) FROM root_chunk_stats`,
		).Scan(&nChunks); err != nil {
			return 0, nil, err
		}
	}

	placeholders := make([]string, len(terms))
	args := make([]any, 0, len(terms)+len(roots))
	sqlText := `SELECT term, COALESCE(SUM(df), 0) FROM term_df WHERE `
	if len(roots) > 0 {
		rootPlaceholders := make([]string, len(roots))
		for i, root := range roots {
			rootPlaceholders[i] = "?"
			args = append(args, normalizeRootName(root))
		}
		sqlText += `root_name IN (` + strings.Join(rootPlaceholders, ",") + `) AND `
	}
	for i, term := range terms {
		placeholders[i] = "?"
		args = append(args, term)
	}
	sqlText += `term IN (` + strings.Join(placeholders, ",") + `) GROUP BY term`

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var term string
		var count int
		if err := rows.Scan(&term, &count); err != nil {
			return 0, nil, err
		}
		df[term] = count
	}
	return nChunks, df, rows.Err()
}

// maxLookupDFRatio drops ubiquitous terms from posting candidate lookup when
// rarer query terms exist. Final IDF-weighted scoring still uses the full query.
const maxLookupDFRatio = 0.15

func selectLookupTerms(terms []string, idf map[string]float64, df map[string]int, nChunks, limit int) []string {
	type scored struct {
		term string
		w    float64
	}
	var items []scored
	for _, term := range terms {
		items = append(items, scored{term, idf[term]})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].w == items[j].w {
			return items[i].term < items[j].term
		}
		return items[i].w > items[j].w
	})

	// Prefer discriminative terms for the posting IN-list.
	var rare []string
	for _, it := range items {
		if nChunks > 0 {
			ratio := float64(df[it.term]) / float64(nChunks)
			if ratio > maxLookupDFRatio {
				continue
			}
		}
		rare = append(rare, it.term)
		if limit > 0 && len(rare) >= limit {
			break
		}
	}
	// Fallback: if every term is ubiquitous, keep the rarest few by IDF.
	if len(rare) == 0 {
		keep := 3
		if limit > 0 && limit < keep {
			keep = limit
		}
		if keep > len(items) {
			keep = len(items)
		}
		for i := 0; i < keep; i++ {
			rare = append(rare, items[i].term)
		}
	}
	sort.Strings(rare)
	return rare
}

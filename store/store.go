// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

const SourceMarkdown = "markdown"
const SourceSource = "source"
const SourceWeb = "web"
const SourcePDF = "pdf"
const SourceGit = "git"

// Authority ranks source usefulness for implementation context.
const (
	AuthorityCurrentProject   = "current_project"
	AuthorityRelatedProject   = "related_internal_project"
	AuthorityCuratedRecipe    = "curated_internal_recipe"
	AuthorityOfficialExample  = "official_example"
	AuthorityOfficialDocs     = "official_documentation"
	AuthorityGeneratedSummary = "generated_summary"
	AuthorityThirdParty       = "third_party_reference"
	AuthorityUnknown          = "unknown"
)

// Document is a stored knowledge document.
type Document struct {
	ID             int64  `json:"id"`
	URI            string `json:"uri"`
	Title          string `json:"title"`
	SourceType     string `json:"sourceType"`
	Path           string `json:"path"`
	RootName       string `json:"rootName,omitempty"`
	Authority      string `json:"authority,omitempty"`
	Technology     string `json:"technology,omitempty"`
	Language       string `json:"language,omitempty"`
	ProductVersion string `json:"productVersion,omitempty"`
	Deprecated     bool   `json:"deprecated,omitempty"`
	Archived       bool   `json:"archived,omitempty"`
	Mtime          int64  `json:"mtime"`
	Hash           string `json:"hash"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// Chunk is a section of a document.
type Chunk struct {
	ID         int64  `json:"id"`
	DocumentID int64  `json:"documentId"`
	Ordinal    int    `json:"ordinal"`
	Heading    string `json:"heading"`
	Body       string `json:"body"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	StartPage  int    `json:"startPage,omitempty"`
	EndPage    int    `json:"endPage,omitempty"`
}

// SearchHit is one FTS result with a generated snippet.
type SearchHit struct {
	ChunkID        int64   `json:"chunkId"`
	DocumentID     int64   `json:"documentId"`
	URI            string  `json:"uri"`
	Title          string  `json:"title"`
	RootName       string  `json:"rootName,omitempty"`
	Path           string  `json:"path,omitempty"`
	Authority      string  `json:"authority,omitempty"`
	Language       string  `json:"language,omitempty"`
	Technology     string  `json:"technology,omitempty"`
	ProductVersion string  `json:"productVersion,omitempty"`
	Deprecated     bool    `json:"deprecated,omitempty"`
	Archived       bool    `json:"archived,omitempty"`
	Ordinal        int     `json:"ordinal"`
	Heading        string  `json:"heading"`
	Snippet        string  `json:"snippet"`
	StartLine      int     `json:"startLine"`
	EndLine        int     `json:"endLine"`
	StartPage      int     `json:"startPage,omitempty"`
	EndPage        int     `json:"endPage,omitempty"`
	Rank           float64 `json:"rank"`
	Score          float64 `json:"score,omitempty"`     // composite score after authority/symbol boosts
	MatchKind      string  `json:"matchKind,omitempty"` // symbol|filename|path|heading|body|semantic
	// ScoreBreakdown is populated when explain scoring is requested.
	ScoreBreakdown *ScoreBreakdown `json:"scoreBreakdown,omitempty"`
}

// ScoreBreakdown explains compositeScore components (admin/debug).
type ScoreBreakdown struct {
	BM25              float64 `json:"bm25"`
	AuthorityBoost    float64 `json:"authorityBoost"`
	AuthorityRank     int     `json:"authorityRank"`
	PathTitleBias     float64 `json:"pathTitleBias"`
	SymbolBias        float64 `json:"symbolBias"`
	ArchivedPenalty   float64 `json:"archivedPenalty,omitempty"`
	DeprecatedPenalty float64 `json:"deprecatedPenalty,omitempty"`
	Total             float64 `json:"total"`
}

// Limits for query / result size (overridable via SearchOptions).
const (
	DefaultSearchLimit   = 20
	MaxSearchLimit       = 100
	MaxQueryRunes        = 512
	MaxFTSQueryTokens    = 64
	MaxSnippetTokens     = 32
	DefaultMaxPerDoc     = 3
	DefaultCandidateMult = 4
)

// ClampLimit applies: requested → default if missing/non-positive → clamp to max.
func ClampLimit(limit, def, max int) int {
	if limit <= 0 {
		limit = def
	}
	if max > 0 && limit > max {
		limit = max
	}
	return limit
}

// ClampSearchLimit clamps a search/symbol limit to the configured and hard maximums.
// requested ≤ 0 uses configuredMax (or DefaultSearchLimit). Result never exceeds
// min(configuredMax, MaxSearchLimit) when configuredMax > 0.
func ClampSearchLimit(requested, configuredMax int) int {
	def := configuredMax
	if def <= 0 {
		def = DefaultSearchLimit
	}
	limit := ClampLimit(requested, def, MaxSearchLimit)
	if configuredMax > 0 && limit > configuredMax {
		limit = configuredMax
	}
	return limit
}

// UpsertInput is a document plus chunks to store.
type UpsertInput struct {
	URI            string
	Title          string
	SourceType     string
	Path           string
	RootName       string
	Authority      string
	Technology     string
	Language       string
	ProductVersion string
	Deprecated     bool
	Archived       bool
	Mtime          int64
	Hash           string
	Chunks         []Chunk
	Symbols        []SymbolInput // optional; replaces prior symbols for the document
}

// SymbolInput is a symbol extracted during ingest.
type SymbolInput struct {
	Name            string
	QualifiedName   string
	UnqualifiedName string
	Namespace       string
	Kind            string
	Language        string
	Signature       string
	StartLine       int
	EndLine         int
}

// Store wraps the SQLite knowledge database.
type Store struct {
	db *sql.DB
}

// OpenOptions configures database open behavior.
type OpenOptions struct {
	// ReadOnly opens the DB read-only when true (ingest/delete will fail).
	ReadOnly bool
}

// Open opens (or creates) the database, applies pragmas, and migrates.
func Open(path string) (*Store, error) {
	return OpenWithOptions(path, OpenOptions{})
}

// OpenWithOptions opens the database with optional read-only mode.
//
// Connection pool is intentionally MaxOpenConns=1: SQLite + WAL works best with
// a single writer connection; PRAGMAs set here apply to that connection and are
// also embedded in the DSN so reconnects keep the same settings.
func OpenWithOptions(path string, opt OpenOptions) (*Store, error) {
	dsn := buildDSN(path, opt.ReadOnly)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Re-apply key PRAGMAs on the live connection (defense in depth).
	for _, pragma := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA temp_store = MEMORY`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if !opt.ReadOnly {
		if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
			db.Close()
			return nil, fmt.Errorf("journal_mode WAL: %w", err)
		}
		if _, err := db.Exec(`PRAGMA synchronous = NORMAL`); err != nil {
			db.Close()
			return nil, fmt.Errorf("synchronous: %w", err)
		}
	}

	if err := ensureSchema(db, path); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func buildDSN(path string, readOnly bool) string {
	// modernc.org/sqlite accepts path or file: URI with _pragma query params.
	mode := ""
	if readOnly {
		mode = "mode=ro&"
	}
	return fmt.Sprintf(
		"file:%s?%s_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=temp_store(MEMORY)",
		filepathToURIPath(path),
		mode,
	)
}

func filepathToURIPath(path string) string {
	// Keep Windows paths usable inside file: DSN.
	p := filepath.ToSlash(path)
	return p
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// FinalizeEmptyPackage checkpoints WAL and switches to DELETE journal so a
// freshly created empty database can ship as a single .db file.
func (s *Store) FinalizeEmptyPackage() error {
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("wal_checkpoint: %w", err)
	}
	if _, err := s.db.Exec(`PRAGMA journal_mode = DELETE`); err != nil {
		return fmt.Errorf("journal_mode DELETE: %w", err)
	}
	return nil
}

// GetHashByURI returns the stored hash for a URI, or "" if missing.
func (s *Store) GetHashByURI(ctx context.Context, uri string) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT hash FROM documents WHERE uri = ?`, uri).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}

// UpsertDocument replaces a document and its chunks when the hash changed.
// Returns true if content was written, false if skipped (same hash).
func (s *Store) UpsertDocument(ctx context.Context, in UpsertInput) (bool, error) {
	existing, err := s.GetHashByURI(ctx, in.URI)
	if err != nil {
		return false, err
	}
	if existing != "" && existing == in.Hash {
		return false, nil
	}

	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	authority := in.Authority
	if authority == "" {
		authority = AuthorityUnknown
	}
	var deprecated, archived int
	if in.Deprecated {
		deprecated = 1
	}
	if in.Archived {
		archived = 1
	}

	var docID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM documents WHERE uri = ?`, in.URI).Scan(&docID)
	switch {
	case err == sql.ErrNoRows:
		res, err := tx.ExecContext(ctx, `
			INSERT INTO documents (
				uri, title, source_type, path, root_name, authority, technology, language, product_version,
				deprecated, archived, mtime, hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.URI, in.Title, in.SourceType, in.Path, nullIfEmpty(in.RootName),
			authority, in.Technology, in.Language, in.ProductVersion,
			deprecated, archived, in.Mtime, in.Hash, now, now,
		)
		if err != nil {
			return false, fmt.Errorf("insert document: %w", err)
		}
		docID, err = res.LastInsertId()
		if err != nil {
			return false, err
		}
	case err != nil:
		return false, err
	default:
		if _, err := tx.ExecContext(ctx, `
			UPDATE documents
			SET title = ?, source_type = ?, path = ?, root_name = ?,
			    authority = ?, technology = ?, language = ?, product_version = ?,
			    deprecated = ?, archived = ?, mtime = ?, hash = ?, updated_at = ?
			WHERE id = ?`,
			in.Title, in.SourceType, in.Path, nullIfEmpty(in.RootName),
			authority, in.Technology, in.Language, in.ProductVersion,
			deprecated, archived, in.Mtime, in.Hash, now, docID,
		); err != nil {
			return false, fmt.Errorf("update document: %w", err)
		}
		if err := retractDocumentSemanticStats(ctx, tx, docID); err != nil {
			return false, fmt.Errorf("retract semantic stats: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, docID); err != nil {
			return false, fmt.Errorf("clear chunks: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM symbols WHERE document_id = ?`, docID); err != nil {
			return false, fmt.Errorf("clear symbols: %w", err)
		}
	}

	for i, c := range in.Chunks {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO chunks (document_id, ordinal, heading, body, start_line, end_line, start_page, end_page, root_name)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			docID, i, c.Heading, c.Body, c.StartLine, c.EndLine, c.StartPage, c.EndPage, in.RootName,
		)
		if err != nil {
			return false, fmt.Errorf("insert chunk %d: %w", i, err)
		}
		chunkID, err := res.LastInsertId()
		if err != nil {
			return false, err
		}
		if err := adjustRootChunkCountTx(ctx, tx, in.RootName, 1); err != nil {
			return false, fmt.Errorf("root chunk stats: %w", err)
		}
		if err := s.upsertChunkTermVector(ctx, tx, chunkID, in.RootName, c.Heading, c.Body); err != nil {
			return false, fmt.Errorf("term vector chunk %d: %w", i, err)
		}
	}
	for _, sym := range in.Symbols {
		forms := DeriveSymbolForms(sym.Name, sym.Signature)
		if forms.NameNorm == "" {
			continue
		}
		qual := sym.QualifiedName
		if qual == "" {
			qual = forms.QualifiedName
		}
		unqual := sym.UnqualifiedName
		if unqual == "" {
			unqual = forms.UnqualifiedName
		}
		ns := sym.Namespace
		if ns == "" {
			ns = forms.Namespace
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO symbols (
				document_id, root_name, name, name_norm, qualified_name, unqualified_name, namespace,
				kind, language, signature, signature_norm, start_line, end_line)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			docID, in.RootName, forms.Name, forms.NameNorm, qual, unqual, ns,
			sym.Kind, sym.Language, sym.Signature, forms.SignatureNorm, sym.StartLine, sym.EndLine,
		); err != nil {
			return false, fmt.Errorf("insert symbol %s: %w", sym.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// GetDocumentByURI loads a document and all chunks.
func (s *Store) GetDocumentByURI(ctx context.Context, uri string) (*Document, []Chunk, error) {
	doc, err := s.scanDocument(s.db.QueryRowContext(ctx, documentSelect+` WHERE uri = ?`, uri))
	if err != nil {
		return nil, nil, err
	}
	chunks, err := s.chunksForDocument(ctx, doc.ID)
	return doc, chunks, err
}

const documentSelect = `
		SELECT id, uri, title, source_type, path, COALESCE(root_name, ''),
		       COALESCE(authority, 'unknown'), COALESCE(technology, ''), COALESCE(language, ''),
		       COALESCE(product_version, ''), COALESCE(deprecated, 0), COALESCE(archived, 0),
		       mtime, hash, created_at, updated_at
		FROM documents`

// GetDocumentByID loads a document and all chunks.
func (s *Store) GetDocumentByID(ctx context.Context, id int64) (*Document, []Chunk, error) {
	doc, err := s.scanDocument(s.db.QueryRowContext(ctx, documentSelect+` WHERE id = ?`, id))
	if err != nil {
		return nil, nil, err
	}
	chunks, err := s.chunksForDocument(ctx, doc.ID)
	return doc, chunks, err
}

// ListDocuments returns documents, optionally filtered by source_type.
func (s *Store) ListDocuments(ctx context.Context, sourceType string) ([]Document, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if sourceType != "" {
		rows, err = s.db.QueryContext(ctx, documentSelect+` WHERE source_type = ? ORDER BY uri`, sourceType)
	} else {
		rows, err = s.db.QueryContext(ctx, documentSelect+` ORDER BY uri`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		d, err := scanDocumentRow(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *d)
	}
	return docs, rows.Err()
}

// DeleteDocument removes a document by URI.
func (s *Store) DeleteDocument(ctx context.Context, uri string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var docID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM documents WHERE uri = ?`, uri).Scan(&docID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := retractDocumentSemanticStats(ctx, tx, docID); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, docID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n > 0, nil
}

// DeleteDocumentsByURIPrefix deletes all documents whose URI starts with prefix.
// Uses SQL LIKE with escaped %/_ in the prefix, then appends '%'.
func (s *Store) DeleteDocumentsByURIPrefix(ctx context.Context, prefix string) (int64, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return 0, fmt.Errorf("prefix is required")
	}
	escaped := escapeLike(prefix)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id FROM documents WHERE uri LIKE ? ESCAPE '\'`, escaped+"%")
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := retractDocumentsSemanticStats(ctx, tx, ids); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE uri LIKE ? ESCAPE '\'`, escaped+"%")
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// SearchOptions configures an FTS search. Empty Roots searches all roots
// (callers that care about isolation should ResolveRoots first).
type SearchOptions struct {
	Query     string
	Limit     int
	Roots     []string // optional root_name filter
	MaxPerDoc int      // diversify: max chunks per document (default 3; 0 = default; <0 = unlimited)
	// MaxResults is the caller-configured soft maximum (e.g. server -max-results).
	// 0 means only DefaultSearchLimit (when Limit unset) and MaxSearchLimit apply.
	MaxResults int
	// Semantic enables optional sparse term-vector similarity to supplement FTS.
	// Uses chunk_term_vectors + chunk_term_postings with query-time IDF. Off by default.
	Semantic bool
	// PreferredVersion softly prefers matching product_version within an authority tier.
	PreferredVersion string
}

// Search runs an FTS5 query and returns ranked hits with snippets.
// Prefer SearchOpts when filtering by knowledge root.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	return s.SearchOpts(ctx, SearchOptions{Query: query, Limit: limit})
}

// SearchOpts runs an FTS5 query with optional root_name filtering.
func (s *Store) SearchOpts(ctx context.Context, opt SearchOptions) ([]SearchHit, error) {
	query := strings.TrimSpace(opt.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if utf8.RuneCountInString(query) > MaxQueryRunes {
		return nil, fmt.Errorf("query exceeds %d characters", MaxQueryRunes)
	}
	if len(tokenizeQuery(query)) > MaxFTSQueryTokens {
		return nil, fmt.Errorf("query exceeds %d searchable tokens", MaxFTSQueryTokens)
	}
	limit := ClampSearchLimit(opt.Limit, opt.MaxResults)
	maxPerDoc := opt.MaxPerDoc
	if maxPerDoc == 0 {
		maxPerDoc = DefaultMaxPerDoc
	}

	ftsQuery := toFTSQuery(query)
	if ftsQuery == "" {
		return nil, fmt.Errorf("query produced no searchable terms")
	}

	var roots []string
	for _, r := range opt.Roots {
		r = strings.TrimSpace(r)
		if r != "" {
			roots = append(roots, r)
		}
	}

	// Fetch a larger candidate set, then diversify by document.
	candidateLimit := limit
	if maxPerDoc > 0 {
		candidateLimit = limit * DefaultCandidateMult
		if candidateLimit < limit {
			candidateLimit = limit
		}
		if candidateLimit > MaxSearchLimit*DefaultCandidateMult {
			candidateLimit = MaxSearchLimit * DefaultCandidateMult
		}
	}

	candidates, err := s.searchFTS(ctx, ftsQuery, roots, candidateLimit, query)
	if err != nil {
		return nil, err
	}
	// Natural-language tasks often fail strict AND; retry with OR of content tokens.
	if len(candidates) == 0 {
		if orQ := toFTSQueryOR(query); orQ != "" && orQ != ftsQuery {
			candidates, err = s.searchFTS(ctx, orQ, roots, candidateLimit, query)
			if err != nil {
				return nil, err
			}
		}
	}

	// Path/title/filename are first-class: include docs whose path matches even if body does not.
	pathHits, err := s.pathTitleCandidates(ctx, query, roots, candidateLimit)
	if err != nil {
		return nil, err
	}
	seenChunk := map[int64]struct{}{}
	for _, h := range candidates {
		seenChunk[h.ChunkID] = struct{}{}
	}
	for _, h := range pathHits {
		if _, ok := seenChunk[h.ChunkID]; ok {
			continue
		}
		seenChunk[h.ChunkID] = struct{}{}
		candidates = append(candidates, h)
	}
	if opt.Semantic {
		// Pass the final result limit (not the FTS-enlarged candidateLimit) so
		// semantic candidate expansion is not double-multiplied.
		semHits, err := s.semanticCandidates(ctx, query, roots, limit)
		if err != nil {
			return nil, err
		}
		for _, h := range semHits {
			if _, ok := seenChunk[h.ChunkID]; ok {
				continue
			}
			seenChunk[h.ChunkID] = struct{}{}
			candidates = append(candidates, h)
		}
	}
	applyVersionPreference(candidates, opt.PreferredVersion)
	sortSearchHits(candidates)
	return diversifyHits(candidates, limit, maxPerDoc), nil
}

func applyVersionPreference(hits []SearchHit, preferred string) {
	pref := strings.ToLower(strings.TrimSpace(preferred))
	if pref == "" || pref == "unknown" {
		return
	}
	for i := range hits {
		ver := strings.ToLower(strings.TrimSpace(hits[i].ProductVersion))
		if ver == "" {
			continue
		}
		if ver == pref {
			hits[i].Score += 5
		} else {
			hits[i].Score -= 8
		}
	}
}

// sortSearchHits orders by authority tier first (lower AuthorityRank wins),
// then composite Score, then BM25 Rank. Path/title biases cannot invert tiers.
func sortSearchHits(hits []SearchHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		ri, rj := AuthorityRank(hits[i].Authority), AuthorityRank(hits[j].Authority)
		if ri != rj {
			return ri < rj
		}
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Rank < hits[j].Rank
	})
}

func (s *Store) searchFTS(ctx context.Context, ftsQuery string, roots []string, candidateLimit int, query string) ([]SearchHit, error) {
	var (
		rows *sql.Rows
		err  error
	)
	// bm25 column weights: heading (col 0) heavier than body (col 1).
	baseSelect := `
			SELECT
				c.id,
				c.document_id,
				d.uri,
				d.title,
				COALESCE(d.root_name, ''),
				COALESCE(d.path, ''),
				COALESCE(d.authority, 'unknown'),
				COALESCE(d.language, ''),
				COALESCE(d.technology, ''),
				COALESCE(d.product_version, ''),
				COALESCE(d.deprecated, 0),
				COALESCE(d.archived, 0),
				c.ordinal,
				c.heading,
				snippet(chunks_fts, 1, '<b>', '</b>', '…', 32) AS snip,
				c.start_line,
				c.end_line,
				c.start_page,
				c.end_page,
				bm25(chunks_fts, 10.0, 1.0) AS rank
			FROM chunks_fts
			JOIN chunks c ON c.id = chunks_fts.rowid
			JOIN documents d ON d.id = c.document_id
			WHERE chunks_fts MATCH ?`
	if len(roots) == 0 {
		rows, err = s.db.QueryContext(ctx, baseSelect+`
			ORDER BY rank
			LIMIT ?`, ftsQuery, candidateLimit)
	} else {
		placeholders := make([]string, len(roots))
		args := make([]any, 0, 1+len(roots)+1)
		args = append(args, ftsQuery)
		for i, r := range roots {
			placeholders[i] = "?"
			args = append(args, r)
		}
		args = append(args, candidateLimit)
		// Prefer denormalized chunks.root_name (indexed) with documents.root_name as fallback.
		q := baseSelect + `
			  AND (c.root_name IN (` + strings.Join(placeholders, ",") + `)
			       OR (c.root_name = '' AND d.root_name IN (` + strings.Join(placeholders, ",") + `)))
			ORDER BY rank
			LIMIT ?`
		// Duplicate root args for both IN lists.
		args2 := make([]any, 0, 1+2*len(roots)+1)
		args2 = append(args2, ftsQuery)
		args2 = append(args2, args[1:1+len(roots)]...)
		args2 = append(args2, args[1:1+len(roots)]...)
		args2 = append(args2, candidateLimit)
		rows, err = s.db.QueryContext(ctx, q, args2...)
	}
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var candidates []SearchHit
	for rows.Next() {
		var h SearchHit
		var deprecated, archived int
		if err := rows.Scan(
			&h.ChunkID, &h.DocumentID, &h.URI, &h.Title, &h.RootName, &h.Path,
			&h.Authority, &h.Language, &h.Technology, &h.ProductVersion, &deprecated, &archived,
			&h.Ordinal, &h.Heading, &h.Snippet, &h.StartLine, &h.EndLine, &h.StartPage, &h.EndPage, &h.Rank,
		); err != nil {
			return nil, err
		}
		h.Deprecated = deprecated != 0
		h.Archived = archived != 0
		h.Score, h.MatchKind = compositeScore(h, query)
		candidates = append(candidates, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

// pathTitleCandidates finds documents whose path, basename, or title matches the query.
func (s *Store) pathTitleCandidates(ctx context.Context, query string, roots []string, limit int) ([]SearchHit, error) {
	q := strings.TrimSpace(query)
	if q == "" || limit <= 0 {
		return nil, nil
	}
	like := "%" + escapeLike(q) + "%"
	sqlText := `
		SELECT c.id, c.document_id, d.uri, d.title, COALESCE(d.root_name, ''), COALESCE(d.path, ''),
		       COALESCE(d.authority, 'unknown'), COALESCE(d.language, ''), COALESCE(d.technology, ''),
		       COALESCE(d.product_version, ''), COALESCE(d.deprecated, 0), COALESCE(d.archived, 0),
		       c.ordinal, c.heading, substr(c.body, 1, 240), c.start_line, c.end_line, c.start_page, c.end_page
		FROM documents d
		JOIN chunks c ON c.document_id = d.id AND c.ordinal = 0
		WHERE (d.path LIKE ? ESCAPE '\' OR d.title LIKE ? ESCAPE '\' OR d.uri LIKE ? ESCAPE '\')`
	args := []any{like, like, like}
	if len(roots) > 0 {
		ph := make([]string, len(roots))
		for i, r := range roots {
			ph[i] = "?"
			args = append(args, r)
		}
		sqlText += ` AND d.root_name IN (` + strings.Join(ph, ",") + `)`
	}
	sqlText += ` LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		var deprecated, archived int
		if err := rows.Scan(
			&h.ChunkID, &h.DocumentID, &h.URI, &h.Title, &h.RootName, &h.Path,
			&h.Authority, &h.Language, &h.Technology, &h.ProductVersion, &deprecated, &archived,
			&h.Ordinal, &h.Heading, &h.Snippet, &h.StartLine, &h.EndLine, &h.StartPage, &h.EndPage,
		); err != nil {
			return nil, err
		}
		h.Deprecated = deprecated != 0
		h.Archived = archived != 0
		h.Rank = 0
		h.Score, h.MatchKind = compositeScore(h, query)
		if h.MatchKind == "body" {
			h.MatchKind = "path"
			h.Score += 8
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func compositeScore(h SearchHit, query string) (float64, string) {
	bd := explainCompositeScore(h, query)
	return bd.Total, bd.matchKind
}

func explainCompositeScore(h SearchHit, query string) scoreParts {
	// BM25: more negative/lower is better in SQLite; invert for a positive score.
	bm25 := -h.Rank
	authBoost := AuthorityBoost(h.Authority)
	q := strings.ToLower(strings.TrimSpace(query))
	pathLower := strings.ToLower(strings.ReplaceAll(h.Path, `\`, `/`))
	base := strings.ToLower(BasenamePath(h.Path))
	titleLower := strings.ToLower(h.Title)
	headingLower := strings.ToLower(h.Heading)
	matchKind := "body"
	var pathBias, symbolBias float64

	if base != "" && (base == q || strings.TrimSuffix(base, filepath.Ext(base)) == q) {
		pathBias += 28
		matchKind = "filename"
	} else if base != "" && strings.Contains(base, q) {
		pathBias += 18
		matchKind = "filename"
	} else if pathLower != "" && strings.Contains(pathLower, q) {
		pathBias += 12
		matchKind = "path"
	}
	if titleLower != "" && strings.Contains(titleLower, q) {
		pathBias += 6
		if matchKind == "body" {
			matchKind = "heading"
		}
	}
	if headingLower != "" && strings.Contains(headingLower, q) {
		pathBias += 10
		if matchKind == "body" {
			matchKind = "heading"
		}
	}
	for _, tok := range tokenizeQuery(query) {
		if !looksLikeSymbol(tok) {
			continue
		}
		tl := strings.ToLower(tok)
		if strings.Contains(strings.ToLower(h.Snippet), tl) || strings.Contains(headingLower, tl) || strings.Contains(titleLower, tl) {
			symbolBias += 14
			matchKind = "symbol"
		}
		if base != "" && strings.Contains(base, tl) {
			symbolBias += 8
		}
	}
	var archivedPen, deprecatedPen float64
	if h.ArchivedHint() {
		archivedPen = 25
	}
	if h.Deprecated {
		deprecatedPen = 20
	}
	total := bm25 + authBoost + pathBias + symbolBias - archivedPen - deprecatedPen
	return scoreParts{
		BM25: bm25, AuthorityBoost: authBoost, AuthorityRank: AuthorityRank(h.Authority),
		PathTitleBias: pathBias, SymbolBias: symbolBias,
		ArchivedPenalty: archivedPen, DeprecatedPenalty: deprecatedPen,
		Total: total, matchKind: matchKind,
	}
}

type scoreParts struct {
	BM25              float64
	AuthorityBoost    float64
	AuthorityRank     int
	PathTitleBias     float64
	SymbolBias        float64
	ArchivedPenalty   float64
	DeprecatedPenalty float64
	Total             float64
	matchKind         string
}

// AttachScoreBreakdown fills ScoreBreakdown on each hit (admin explain path).
func AttachScoreBreakdown(hits []SearchHit, query string) {
	for i := range hits {
		p := explainCompositeScore(hits[i], query)
		hits[i].ScoreBreakdown = &ScoreBreakdown{
			BM25: p.BM25, AuthorityBoost: p.AuthorityBoost, AuthorityRank: p.AuthorityRank,
			PathTitleBias: p.PathTitleBias, SymbolBias: p.SymbolBias,
			ArchivedPenalty: p.ArchivedPenalty, DeprecatedPenalty: p.DeprecatedPenalty,
			Total: p.Total,
		}
	}
}

// ArchivedHint is true when the document is marked archived or path suggests obsolete material.
func (h SearchHit) ArchivedHint() bool {
	if h.Archived {
		return true
	}
	a := strings.ToLower(h.Authority)
	return a == "archived" || strings.Contains(strings.ToLower(h.Path), "/archive/")
}

// AuthorityRank returns the sort tier for an authority class (0 = highest).
// Lower rank always sorts before higher rank, regardless of path/title bias.
func AuthorityRank(authority string) int {
	switch authority {
	case AuthorityCurrentProject:
		return 0
	case AuthorityRelatedProject:
		return 1
	case AuthorityCuratedRecipe:
		return 2
	case AuthorityOfficialExample:
		return 3
	case AuthorityOfficialDocs:
		return 4
	case AuthorityGeneratedSummary:
		return 5
	case AuthorityThirdParty:
		return 6
	default:
		return 7
	}
}

// AuthorityBoost returns a ranking bonus for a source authority class.
// Values match ARCHITECTURE.md order (generated above third_party).
func AuthorityBoost(authority string) float64 {
	switch authority {
	case AuthorityCurrentProject:
		return 40
	case AuthorityRelatedProject:
		return 30
	case AuthorityCuratedRecipe:
		return 28
	case AuthorityOfficialExample:
		return 22
	case AuthorityOfficialDocs:
		return 14
	case AuthorityGeneratedSummary:
		return 8
	case AuthorityThirdParty:
		return 4
	default:
		return 0
	}
}

func looksLikeSymbol(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, ":.#()/<>") {
		return true
	}
	hasUpper, hasLower, hasUnder := false, false, false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r == '_':
			hasUnder = true
		}
	}
	return (hasUpper && hasLower) || hasUnder || (hasUpper && len(s) > 2)
}

func diversifyHits(hits []SearchHit, limit, maxPerDoc int) []SearchHit {
	if limit <= 0 {
		return nil
	}
	if maxPerDoc < 0 {
		if len(hits) > limit {
			return hits[:limit]
		}
		return hits
	}
	perDoc := map[int64]int{}
	out := make([]SearchHit, 0, limit)
	for _, h := range hits {
		if perDoc[h.DocumentID] >= maxPerDoc {
			continue
		}
		perDoc[h.DocumentID]++
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Store) chunksForDocument(ctx context.Context, docID int64) ([]Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, document_id, ordinal, heading, body, start_line, end_line, start_page, end_page
		FROM chunks WHERE document_id = ? ORDER BY ordinal`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Ordinal, &c.Heading, &c.Body, &c.StartLine, &c.EndLine, &c.StartPage, &c.EndPage); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

func (s *Store) scanDocument(row *sql.Row) (*Document, error) {
	var d Document
	var root sql.NullString
	var dep, arch int
	err := row.Scan(
		&d.ID, &d.URI, &d.Title, &d.SourceType, &d.Path, &root,
		&d.Authority, &d.Technology, &d.Language, &d.ProductVersion, &dep, &arch,
		&d.Mtime, &d.Hash, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, err
	}
	if root.Valid {
		d.RootName = root.String
	}
	d.Deprecated = dep != 0
	d.Archived = arch != 0
	return &d, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDocumentRow(row rowScanner) (*Document, error) {
	var d Document
	var root sql.NullString
	var dep, arch int
	if err := row.Scan(
		&d.ID, &d.URI, &d.Title, &d.SourceType, &d.Path, &root,
		&d.Authority, &d.Technology, &d.Language, &d.ProductVersion, &dep, &arch,
		&d.Mtime, &d.Hash, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if root.Valid {
		d.RootName = root.String
	}
	d.Deprecated = dep != 0
	d.Archived = arch != 0
	return &d, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// toFTSQuery turns free-text into a safe FTS5 AND-of-phrases query.
// Tokens are quoted so FTS operators (AND/OR/NOT/NEAR) and punctuation
// like C++, C#, and snake_case are treated as literals. Empty/wildcard-only
// input yields "".
func toFTSQuery(q string) string {
	tokens := tokenizeQuery(q)
	if len(tokens) == 0 {
		return ""
	}
	// Prefer content-bearing tokens; fall back to all tokens if everything was a stopword.
	content := filterStopwords(tokens)
	if len(content) > 0 {
		tokens = content
	}
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ReplaceAll(t, `"`, ``)
		t = strings.TrimSpace(t)
		if t == "" || t == "*" {
			continue
		}
		// Strip characters that break FTS phrase quoting even inside quotes.
		t = strings.Map(func(r rune) rune {
			switch r {
			case '"', '\x00':
				return -1
			default:
				return r
			}
		}, t)
		if t == "" {
			continue
		}
		parts = append(parts, `"`+t+`"`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}

func tokenizeQuery(q string) []string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	// Split on whitespace; keep punctuation attached so C++ / foo::bar stay whole.
	fields := strings.Fields(q)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		// Drop pure FTS operator tokens when they appear alone (still searchable if quoted by user text).
		upper := strings.ToUpper(f)
		switch upper {
		case "AND", "OR", "NOT", "NEAR":
			continue
		}
		out = append(out, f)
	}
	return out
}

var ftsStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "to": {}, "of": {}, "in": {}, "on": {},
	"for": {}, "with": {}, "from": {}, "into": {}, "by": {}, "as": {}, "is": {}, "are": {},
	"be": {}, "this": {}, "that": {}, "it": {}, "at": {}, "add": {}, "use": {}, "using": {},
	"make": {}, "create": {}, "please": {}, "how": {}, "do": {}, "does": {}, "can": {},
	"should": {}, "must": {}, "need": {}, "needed": {}, "new": {},
}

func filterStopwords(tokens []string) []string {
	var out []string
	for _, t := range tokens {
		key := strings.ToLower(strings.Trim(t, ".,;:()[]{}\"'"))
		if len(key) < 3 {
			continue
		}
		if _, stop := ftsStopwords[key]; stop {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toFTSQueryOR(q string) string {
	tokens := filterStopwords(tokenizeQuery(q))
	if len(tokens) == 0 {
		tokens = tokenizeQuery(q)
	}
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ReplaceAll(t, `"`, ``)
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		parts = append(parts, `"`+t+`"`)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
}

// SchemaVersion returns PRAGMA user_version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v)
	return v, err
}

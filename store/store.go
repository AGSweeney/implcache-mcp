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
}

// SearchHit is one FTS result with a generated snippet.
type SearchHit struct {
	ChunkID    int64   `json:"chunkId"`
	DocumentID int64   `json:"documentId"`
	URI        string  `json:"uri"`
	Title      string  `json:"title"`
	RootName   string  `json:"rootName,omitempty"`
	Path       string  `json:"path,omitempty"`
	Authority  string  `json:"authority,omitempty"`
	Language   string  `json:"language,omitempty"`
	Technology string  `json:"technology,omitempty"`
	Ordinal    int     `json:"ordinal"`
	Heading    string  `json:"heading"`
	Snippet    string  `json:"snippet"`
	StartLine  int     `json:"startLine"`
	EndLine    int     `json:"endLine"`
	Rank       float64 `json:"rank"`
	Score      float64 `json:"score,omitempty"` // composite score after authority/symbol boosts
}

// Limits for query / result size (overridable via SearchOptions).
const (
	DefaultSearchLimit   = 20
	MaxSearchLimit       = 100
	MaxQueryRunes        = 512
	MaxSnippetTokens     = 32
	DefaultMaxPerDoc     = 3
	DefaultCandidateMult = 4
)

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
	Name      string
	Kind      string
	Language  string
	Signature string
	StartLine int
	EndLine   int
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

	if err := migrate(db); err != nil {
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
		if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, docID); err != nil {
			return false, fmt.Errorf("clear chunks: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM symbols WHERE document_id = ?`, docID); err != nil {
			return false, fmt.Errorf("clear symbols: %w", err)
		}
	}

	for i, c := range in.Chunks {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chunks (document_id, ordinal, heading, body, start_line, end_line)
			VALUES (?, ?, ?, ?, ?, ?)`,
			docID, i, c.Heading, c.Body, c.StartLine, c.EndLine,
		); err != nil {
			return false, fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}
	for _, sym := range in.Symbols {
		norm := NormalizeSymbol(sym.Name)
		if norm == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO symbols (
				document_id, root_name, name, name_norm, kind, language, signature, start_line, end_line)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			docID, in.RootName, sym.Name, norm, sym.Kind, sym.Language, sym.Signature, sym.StartLine, sym.EndLine,
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
	res, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE uri = ?`, uri)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// DeleteDocumentsByURIPrefix deletes all documents whose URI starts with prefix.
// Uses SQL LIKE with escaped %/_ in the prefix, then appends '%'.
func (s *Store) DeleteDocumentsByURIPrefix(ctx context.Context, prefix string) (int64, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return 0, fmt.Errorf("prefix is required")
	}
	escaped := escapeLike(prefix)
	res, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE uri LIKE ? ESCAPE '\'`, escaped+"%")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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
	limit := opt.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}
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

	var (
		rows *sql.Rows
		err  error
	)
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
				c.ordinal,
				c.heading,
				snippet(chunks_fts, 1, '<b>', '</b>', '…', 32) AS snip,
				c.start_line,
				c.end_line,
				bm25(chunks_fts) AS rank
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
		q := baseSelect + `
			  AND d.root_name IN (` + strings.Join(placeholders, ",") + `)
			ORDER BY rank
			LIMIT ?`
		rows, err = s.db.QueryContext(ctx, q, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var candidates []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(
			&h.ChunkID, &h.DocumentID, &h.URI, &h.Title, &h.RootName, &h.Path,
			&h.Authority, &h.Language, &h.Technology,
			&h.Ordinal, &h.Heading, &h.Snippet, &h.StartLine, &h.EndLine, &h.Rank,
		); err != nil {
			return nil, err
		}
		h.Score = compositeScore(h, query)
		candidates = append(candidates, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Rank < candidates[j].Rank // bm25: lower is better
		}
		return candidates[i].Score > candidates[j].Score
	})
	return diversifyHits(candidates, limit, maxPerDoc), nil
}

func compositeScore(h SearchHit, query string) float64 {
	// BM25: more negative/lower is better in SQLite; invert for a positive score.
	score := -h.Rank
	score += AuthorityBoost(h.Authority)
	q := strings.ToLower(query)
	if h.Path != "" && strings.Contains(strings.ToLower(h.Path), strings.TrimSpace(q)) {
		score += 8
	}
	if strings.Contains(strings.ToLower(h.Title), q) {
		score += 4
	}
	// Exact-ish symbol tokens in snippet/heading
	for _, tok := range tokenizeQuery(query) {
		if looksLikeSymbol(tok) && (strings.Contains(h.Snippet, tok) || strings.Contains(h.Heading, tok) || strings.Contains(h.Title, tok)) {
			score += 12
		}
	}
	if h.ArchivedHint() {
		score -= 20
	}
	return score
}

// ArchivedHint is true when authority/path suggests obsolete material.
func (h SearchHit) ArchivedHint() bool {
	a := strings.ToLower(h.Authority)
	return a == "archived" || strings.Contains(strings.ToLower(h.Path), "/archive/")
}

// AuthorityBoost returns a ranking bonus for a source authority class.
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
		return 4
	case AuthorityThirdParty:
		return 6
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
		SELECT id, document_id, ordinal, heading, body, start_line, end_line
		FROM chunks WHERE document_id = ? ORDER BY ordinal`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Ordinal, &c.Heading, &c.Body, &c.StartLine, &c.EndLine); err != nil {
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

// SchemaVersion returns PRAGMA user_version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v)
	return v, err
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
)

// RootCounts is document/chunk/symbol cardinality for one root_name.
type RootCounts struct {
	Documents int64 `json:"documents"`
	Chunks    int64 `json:"chunks"`
	Symbols   int64 `json:"symbols"`
}

// LibraryCounts is global inventory cardinality.
type LibraryCounts struct {
	Documents int64 `json:"documents"`
	Chunks    int64 `json:"chunks"`
	Symbols   int64 `json:"symbols"`
	Recipes   int64 `json:"recipes"`
}

// CountByRoot returns inventory counts for a knowledge root.
func (s *Store) CountByRoot(ctx context.Context, rootName string) (RootCounts, error) {
	var c RootCounts
	if rootName == "" {
		return c, fmt.Errorf("rootName is required")
	}
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM documents WHERE root_name = ?),
			(SELECT COUNT(*) FROM chunks WHERE root_name = ?),
			(SELECT COUNT(*) FROM symbols WHERE root_name = ?)`,
		rootName, rootName, rootName).Scan(&c.Documents, &c.Chunks, &c.Symbols)
	return c, err
}

// CountLibrary returns global document/chunk/symbol/recipe counts.
func (s *Store) CountLibrary(ctx context.Context) (LibraryCounts, error) {
	var c LibraryCounts
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM documents),
			(SELECT COUNT(*) FROM chunks),
			(SELECT COUNT(*) FROM symbols),
			(SELECT COUNT(*) FROM knowledge_entries)`).
		Scan(&c.Documents, &c.Chunks, &c.Symbols, &c.Recipes)
	return c, err
}

// CountDocumentsWithoutChunks returns documents that have no chunk rows.
func (s *Store) CountDocumentsWithoutChunks(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents d
		WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.document_id = d.id)`).Scan(&n)
	return n, err
}

// DeleteDocumentsWithoutChunks removes documents that have no chunk rows
// (ingest stubs / binary-ish files that never produced searchable text).
func (s *Store) DeleteDocumentsWithoutChunks(ctx context.Context) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT d.id FROM documents d
		WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.document_id = d.id)`)
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
	if len(ids) == 0 {
		return 0, nil
	}
	if err := retractDocumentsSemanticStats(ctx, tx, ids); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM documents
		WHERE id IN (
			SELECT d.id FROM documents d
			WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.document_id = d.id)
		)`)
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

// EmptyChunkRootCount is how many chunkless documents sit under one root.
type EmptyChunkRootCount struct {
	RootName   string `json:"rootName"`
	Count      int    `json:"count"`
	SourceType string `json:"sourceType,omitempty"`
}

// DocumentsWithoutChunksReport summarizes orphan documents for health UX.
type DocumentsWithoutChunksReport struct {
	Total      int                   `json:"total"`
	ByRoot     []EmptyChunkRootCount `json:"byRoot"`
	SampleURIs []string              `json:"sampleUris"`
}

const emptyChunkDocSQL = `
FROM documents d
WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.document_id = d.id)`

// DocumentsWithoutChunksReport returns totals, per-root breakdown, and sample URIs.
func (s *Store) DocumentsWithoutChunksReport(ctx context.Context, sampleLimit int) (DocumentsWithoutChunksReport, error) {
	var out DocumentsWithoutChunksReport
	if sampleLimit <= 0 {
		sampleLimit = 8
	}
	if sampleLimit > 25 {
		sampleLimit = 25
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+emptyChunkDocSQL).Scan(&out.Total); err != nil {
		return out, err
	}
	if out.Total == 0 {
		return out, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT d.root_name, COALESCE(d.source_type, ''), COUNT(*)
		`+emptyChunkDocSQL+`
		GROUP BY d.root_name, d.source_type
		ORDER BY COUNT(*) DESC, d.root_name ASC
		LIMIT 20`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r EmptyChunkRootCount
		if err := rows.Scan(&r.RootName, &r.SourceType, &r.Count); err != nil {
			return out, err
		}
		out.ByRoot = append(out.ByRoot, r)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	uriRows, err := s.db.QueryContext(ctx, `
		SELECT d.uri
		`+emptyChunkDocSQL+`
		ORDER BY d.root_name ASC, d.uri ASC
		LIMIT ?`, sampleLimit)
	if err != nil {
		return out, err
	}
	defer uriRows.Close()
	for uriRows.Next() {
		var uri string
		if err := uriRows.Scan(&uri); err != nil {
			return out, err
		}
		out.SampleURIs = append(out.SampleURIs, uri)
	}
	return out, uriRows.Err()
}

// ListDocumentsPage returns a page of documents with optional filters.
func (s *Store) ListDocumentsPage(ctx context.Context, rootName, sourceType string, limit, offset int) ([]Document, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	where := []string{"1=1"}
	args := []any{}
	if rootName != "" {
		where = append(where, "root_name = ?")
		args = append(args, rootName)
	}
	if sourceType != "" {
		where = append(where, "source_type = ?")
		args = append(args, sourceType)
	}
	clause := " WHERE " + joinAND(where)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := documentSelect + clause + ` ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		d, err := scanDocumentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		docs = append(docs, *d)
	}
	return docs, total, rows.Err()
}

func joinAND(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

// ListWebPageErrors returns recent non-empty last_error rows for a web source.
func (s *Store) ListWebPageErrors(ctx context.Context, webSourceID int64, limit int) ([]WebPage, error) {
	if webSourceID == 0 {
		return nil, fmt.Errorf("webSourceId is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, web_source_id, COALESCE(document_id, 0), source_url, canonical_url, relative_path,
			page_title, etag, last_modified, content_hash, http_status, content_type, content_length,
			fetched_at, verified_at, crawl_generation, crawl_depth, last_seen_generation,
			missing_count, last_error
		FROM web_pages
		WHERE web_source_id = ? AND TRIM(COALESCE(last_error, '')) != ''
		ORDER BY verified_at DESC, id DESC
		LIMIT ?`, webSourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebPage
	for rows.Next() {
		var p WebPage
		if err := rows.Scan(
			&p.ID, &p.WebSourceID, &p.DocumentID, &p.SourceURL, &p.CanonicalURL,
			&p.RelativePath, &p.PageTitle, &p.ETag, &p.LastModified, &p.ContentHash,
			&p.HTTPStatus, &p.ContentType, &p.ContentLength, &p.FetchedAt, &p.VerifiedAt,
			&p.CrawlGeneration, &p.CrawlDepth, &p.LastSeenGeneration, &p.MissingCount, &p.LastError,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

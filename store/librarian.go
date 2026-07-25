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

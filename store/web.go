// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// WebSource is a registered documentation site mirror configuration.
type WebSource struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	RootName          string   `json:"rootName"`
	StartURL          string   `json:"startUrl"`
	Profile           string   `json:"profile"`
	AllowedPrefixes   []string `json:"allowedPrefixes"`
	Authority         string   `json:"authority"`
	Product           string   `json:"product"`
	DeclaredVersion   string   `json:"declaredVersion"`
	DetectedVersion   string   `json:"detectedVersion"`
	Target            string   `json:"target"`
	Language          string   `json:"language"`
	Enabled           bool     `json:"enabled"`
	ConfigurationJSON string   `json:"configurationJson,omitempty"`
	LastAttemptAt     int64    `json:"lastAttemptAt"`
	LastSuccessAt     int64    `json:"lastSuccessAt"`
	LastStatus        string   `json:"lastStatus"`
	CreatedAt         int64    `json:"createdAt"`
	UpdatedAt         int64    `json:"updatedAt"`
}

// WebPage tracks one mirrored URL under a web source.
type WebPage struct {
	ID                 int64  `json:"id"`
	WebSourceID        int64  `json:"webSourceId"`
	DocumentID         int64  `json:"documentId,omitempty"`
	SourceURL          string `json:"sourceUrl"`
	CanonicalURL       string `json:"canonicalUrl"`
	RelativePath       string `json:"relativePath"`
	PageTitle          string `json:"pageTitle"`
	ETag               string `json:"etag"`
	LastModified       string `json:"lastModified"`
	ContentHash        string `json:"contentHash"`
	HTTPStatus         int    `json:"httpStatus"`
	ContentType        string `json:"contentType"`
	ContentLength      int64  `json:"contentLength"`
	FetchedAt          int64  `json:"fetchedAt"`
	VerifiedAt         int64  `json:"verifiedAt"`
	CrawlGeneration    int64  `json:"crawlGeneration"`
	CrawlDepth         int    `json:"crawlDepth"`
	LastSeenGeneration int64  `json:"lastSeenGeneration"`
	MissingCount       int    `json:"missingCount"`
	LastError          string `json:"lastError"`
}

// UpsertWebSource creates or updates a web source by name.
func (s *Store) UpsertWebSource(ctx context.Context, in WebSource) (int64, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return 0, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(in.StartURL) == "" {
		return 0, fmt.Errorf("startUrl is required")
	}
	if strings.TrimSpace(in.RootName) == "" {
		return 0, fmt.Errorf("rootName is required")
	}
	if in.Profile == "" {
		in.Profile = "generic"
	}
	if in.Authority == "" {
		in.Authority = AuthorityOfficialDocs
	}
	if len(in.AllowedPrefixes) == 0 {
		in.AllowedPrefixes = []string{in.StartURL}
	}
	prefixes, err := json.Marshal(in.AllowedPrefixes)
	if err != nil {
		return 0, err
	}
	cfg := in.ConfigurationJSON
	if cfg == "" {
		cfg = "{}"
	}
	now := time.Now().Unix()
	enabled := 1
	if in.ID != 0 && !in.Enabled {
		enabled = 0
	}

	var id int64
	err = s.db.QueryRowContext(ctx, `SELECT id FROM web_sources WHERE name = ?`, name).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO web_sources(
				name, root_name, start_url, profile, allowed_prefixes, authority, product,
				declared_version, detected_version, target, language, enabled, configuration_json,
				created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name, in.RootName, in.StartURL, in.Profile, string(prefixes), in.Authority, in.Product,
			in.DeclaredVersion, in.DetectedVersion, in.Target, in.Language, enabled, cfg, now, now,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	case err != nil:
		return 0, err
	default:
		_, err = s.db.ExecContext(ctx, `
			UPDATE web_sources SET
				root_name = ?, start_url = ?, profile = ?, allowed_prefixes = ?, authority = ?,
				product = ?, declared_version = ?, detected_version = ?, target = ?, language = ?,
				enabled = ?, configuration_json = ?, updated_at = ?
			WHERE id = ?`,
			in.RootName, in.StartURL, in.Profile, string(prefixes), in.Authority,
			in.Product, in.DeclaredVersion, in.DetectedVersion, in.Target, in.Language,
			enabled, cfg, now, id,
		)
		return id, err
	}
}

// GetWebSourceByName loads a web source.
func (s *Store) GetWebSourceByName(ctx context.Context, name string) (*WebSource, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, root_name, start_url, profile, allowed_prefixes, authority, product,
			declared_version, detected_version, target, language, enabled, configuration_json,
			last_attempt_at, last_success_at, last_status, created_at, updated_at
		FROM web_sources WHERE name = ?`, name)
	return scanWebSource(row)
}

// GetWebSourceByID loads a web source by id.
func (s *Store) GetWebSourceByID(ctx context.Context, id int64) (*WebSource, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, root_name, start_url, profile, allowed_prefixes, authority, product,
			declared_version, detected_version, target, language, enabled, configuration_json,
			last_attempt_at, last_success_at, last_status, created_at, updated_at
		FROM web_sources WHERE id = ?`, id)
	return scanWebSource(row)
}

// ListWebSources returns all registered sources.
func (s *Store) ListWebSources(ctx context.Context) ([]WebSource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, root_name, start_url, profile, allowed_prefixes, authority, product,
			declared_version, detected_version, target, language, enabled, configuration_json,
			last_attempt_at, last_success_at, last_status, created_at, updated_at
		FROM web_sources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebSource
	for rows.Next() {
		ws, err := scanWebSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ws)
	}
	return out, rows.Err()
}

// DeleteWebSource removes a web source, its pages, and only the documents linked
// from those pages. It does not wipe other corpora that happen to share root_name.
func (s *Store) DeleteWebSource(ctx context.Context, name string) (bool, error) {
	ws, err := s.GetWebSourceByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT document_id FROM web_pages
		WHERE web_source_id = ? AND document_id IS NOT NULL AND document_id > 0`, ws.ID)
	if err != nil {
		return false, err
	}
	var docIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		docIDs = append(docIDs, id)
	}
	if err := rows.Close(); err != nil {
		return false, err
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	if err := retractDocumentsSemanticStats(ctx, tx, docIDs); err != nil {
		return false, err
	}
	for _, id := range docIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id); err != nil {
			return false, err
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM web_sources WHERE id = ?`, ws.ID)
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

// SetWebSourceDetectedVersion records a version inferred from crawled page titles.
func (s *Store) SetWebSourceDetectedVersion(ctx context.Context, id int64, version string) error {
	version = strings.TrimSpace(version)
	if id == 0 || version == "" {
		return nil
	}
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE web_sources SET detected_version = ?, updated_at = ? WHERE id = ?`,
		version, now, id)
	return err
}

// SetWebSourceStatus updates crawl status timestamps.
func (s *Store) SetWebSourceStatus(ctx context.Context, id int64, status string, success bool) error {
	now := time.Now().Unix()
	if success {
		_, err := s.db.ExecContext(ctx, `
			UPDATE web_sources SET last_attempt_at = ?, last_success_at = ?, last_status = ?, updated_at = ?
			WHERE id = ?`, now, now, status, now, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE web_sources SET last_attempt_at = ?, last_status = ?, updated_at = ?
		WHERE id = ?`, now, status, now, id)
	return err
}

// UpsertWebPage inserts or updates a mirrored page row.
func (s *Store) UpsertWebPage(ctx context.Context, p WebPage) (int64, error) {
	if p.WebSourceID == 0 || strings.TrimSpace(p.SourceURL) == "" {
		return 0, fmt.Errorf("webSourceId and sourceUrl are required")
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM web_pages WHERE web_source_id = ? AND source_url = ?`,
		p.WebSourceID, p.SourceURL).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO web_pages(
				web_source_id, document_id, source_url, canonical_url, relative_path, page_title,
				etag, last_modified, content_hash, http_status, content_type, content_length,
				fetched_at, verified_at, crawl_generation, crawl_depth, last_seen_generation,
				missing_count, last_error)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.WebSourceID, nullInt64(p.DocumentID), p.SourceURL, p.CanonicalURL, p.RelativePath, p.PageTitle,
			p.ETag, p.LastModified, p.ContentHash, p.HTTPStatus, p.ContentType, p.ContentLength,
			p.FetchedAt, p.VerifiedAt, p.CrawlGeneration, p.CrawlDepth, p.LastSeenGeneration,
			p.MissingCount, p.LastError,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	case err != nil:
		return 0, err
	default:
		_, err = s.db.ExecContext(ctx, `
			UPDATE web_pages SET
				document_id = ?, canonical_url = ?, relative_path = ?, page_title = ?,
				etag = ?, last_modified = ?, content_hash = ?, http_status = ?, content_type = ?,
				content_length = ?, fetched_at = ?, verified_at = ?, crawl_generation = ?,
				crawl_depth = ?, last_seen_generation = ?, missing_count = ?, last_error = ?
			WHERE id = ?`,
			nullInt64(p.DocumentID), p.CanonicalURL, p.RelativePath, p.PageTitle,
			p.ETag, p.LastModified, p.ContentHash, p.HTTPStatus, p.ContentType,
			p.ContentLength, p.FetchedAt, p.VerifiedAt, p.CrawlGeneration,
			p.CrawlDepth, p.LastSeenGeneration, p.MissingCount, p.LastError, id,
		)
		return id, err
	}
}

// ListWebPages returns pages for a source.
func (s *Store) ListWebPages(ctx context.Context, sourceID int64) ([]WebPage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, web_source_id, COALESCE(document_id, 0), source_url, canonical_url, relative_path,
			page_title, etag, last_modified, content_hash, http_status, content_type, content_length,
			fetched_at, verified_at, crawl_generation, crawl_depth, last_seen_generation,
			missing_count, last_error
		FROM web_pages WHERE web_source_id = ? ORDER BY source_url`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebPage
	for rows.Next() {
		var p WebPage
		if err := rows.Scan(
			&p.ID, &p.WebSourceID, &p.DocumentID, &p.SourceURL, &p.CanonicalURL, &p.RelativePath,
			&p.PageTitle, &p.ETag, &p.LastModified, &p.ContentHash, &p.HTTPStatus, &p.ContentType, &p.ContentLength,
			&p.FetchedAt, &p.VerifiedAt, &p.CrawlGeneration, &p.CrawlDepth, &p.LastSeenGeneration,
			&p.MissingCount, &p.LastError,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkMissingWebPages increments missing_count for pages not seen in generation.
func (s *Store) MarkMissingWebPages(ctx context.Context, sourceID, generation int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE web_pages SET missing_count = missing_count + 1
		WHERE web_source_id = ? AND last_seen_generation < ?`, sourceID, generation)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneWebPages deletes pages with missing_count >= threshold and their documents.
func (s *Store) PruneWebPages(ctx context.Context, sourceID int64, threshold int) (int64, error) {
	if threshold < 1 {
		threshold = 2
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(document_id, 0) FROM web_pages
		WHERE web_source_id = ? AND missing_count >= ?`, sourceID, threshold)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type pair struct{ pageID, docID int64 }
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.pageID, &p.docID); err != nil {
			return 0, err
		}
		pairs = append(pairs, p)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var deleted int64
	for _, p := range pairs {
		if p.docID > 0 {
			if _, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, p.docID); err != nil {
				return deleted, err
			}
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM web_pages WHERE id = ?`, p.pageID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

// NextCrawlGeneration returns max(generation)+1 for a source.
func (s *Store) NextCrawlGeneration(ctx context.Context, sourceID int64) (int64, error) {
	var g sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(crawl_generation) FROM web_pages WHERE web_source_id = ?`, sourceID).Scan(&g)
	if err != nil {
		return 0, err
	}
	if !g.Valid {
		return 1, nil
	}
	return g.Int64 + 1, nil
}

func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

type scannable interface {
	Scan(dest ...any) error
}

func scanWebSource(row scannable) (*WebSource, error) {
	var ws WebSource
	var prefixes string
	var enabled int
	err := row.Scan(
		&ws.ID, &ws.Name, &ws.RootName, &ws.StartURL, &ws.Profile, &prefixes, &ws.Authority, &ws.Product,
		&ws.DeclaredVersion, &ws.DetectedVersion, &ws.Target, &ws.Language, &enabled, &ws.ConfigurationJSON,
		&ws.LastAttemptAt, &ws.LastSuccessAt, &ws.LastStatus, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	ws.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(prefixes), &ws.AllowedPrefixes)
	return &ws, nil
}

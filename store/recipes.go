// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Review statuses for knowledge entries.
const (
	ReviewGenerated     = "generated"
	ReviewHumanReviewed = "human_reviewed"
)

// KnowledgeEntry is a curated or generated implementation recipe.
type KnowledgeEntry struct {
	ID           int64    `json:"id"`
	URI          string   `json:"uri"`
	Subject      string   `json:"subject"`
	Technology   string   `json:"technology"`
	Language     string   `json:"language"`
	Version      string   `json:"version"`
	BodyMarkdown string   `json:"bodyMarkdown"`
	ReviewStatus string   `json:"reviewStatus"`
	Authority    string   `json:"authority"`
	Confidence   string   `json:"confidence"`
	RootName     string   `json:"rootName"`
	CreatedAt    int64    `json:"createdAt"`
	VerifiedAt   int64    `json:"verifiedAt"`
	Hash         string   `json:"hash"`
	SourceURIs   []string `json:"sourceUris,omitempty"`
}

// DeleteAllKnowledgeEntries removes every recipe (knowledge_entries).
// Source lineage rows cascade via FK.
func (s *Store) DeleteAllKnowledgeEntries(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_entries`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// normalizeRecipeAuthority enforces review_status ↔ authority invariants.
func normalizeRecipeAuthority(e *KnowledgeEntry) error {
	switch e.ReviewStatus {
	case ReviewGenerated:
		e.Authority = AuthorityGeneratedSummary
	case ReviewHumanReviewed:
		if e.Authority == "" || e.Authority == AuthorityGeneratedSummary || e.Authority == AuthorityUnknown {
			e.Authority = AuthorityCuratedRecipe
		}
		if AuthorityRank(e.Authority) > AuthorityRank(AuthorityCuratedRecipe) {
			return fmt.Errorf("human_reviewed recipes require authority at least curated_internal_recipe, got %q", e.Authority)
		}
	default:
		return fmt.Errorf("invalid review_status %q (want generated|human_reviewed)", e.ReviewStatus)
	}
	return nil
}

// UpsertKnowledgeEntry stores a recipe with source lineage.
func (s *Store) UpsertKnowledgeEntry(ctx context.Context, e KnowledgeEntry) (int64, error) {
	if strings.TrimSpace(e.URI) == "" || strings.TrimSpace(e.BodyMarkdown) == "" {
		return 0, fmt.Errorf("uri and bodyMarkdown are required")
	}
	if e.ReviewStatus == "" {
		e.ReviewStatus = ReviewGenerated
	}
	if err := normalizeRecipeAuthority(&e); err != nil {
		return 0, err
	}
	if e.Confidence == "" {
		e.Confidence = "medium"
	}
	now := time.Now().Unix()
	if e.CreatedAt == 0 {
		e.CreatedAt = now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM knowledge_entries WHERE uri = ?`, e.URI).Scan(&id)
	switch {
	case err == nil:
		if _, err := tx.ExecContext(ctx, `
			UPDATE knowledge_entries SET
				subject=?, technology=?, language=?, version=?, body_markdown=?,
				review_status=?, authority=?, confidence=?, root_name=?, verified_at=?, hash=?
			WHERE id=?`,
			e.Subject, e.Technology, e.Language, e.Version, e.BodyMarkdown,
			e.ReviewStatus, e.Authority, e.Confidence, e.RootName, e.VerifiedAt, e.Hash, id,
		); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_entry_sources WHERE entry_id=?`, id); err != nil {
			return 0, err
		}
	default:
		res, err := tx.ExecContext(ctx, `
			INSERT INTO knowledge_entries (
				uri, subject, technology, language, version, body_markdown,
				review_status, authority, confidence, root_name, created_at, verified_at, hash)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.URI, e.Subject, e.Technology, e.Language, e.Version, e.BodyMarkdown,
			e.ReviewStatus, e.Authority, e.Confidence, e.RootName, e.CreatedAt, e.VerifiedAt, e.Hash,
		)
		if err != nil {
			return 0, err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
	}
	for _, src := range e.SourceURIs {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO knowledge_entry_sources (entry_id, source_uri) VALUES (?, ?)`,
			id, src); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// SearchKnowledgeEntries finds recipes by subject/technology keywords.
func (s *Store) SearchKnowledgeEntries(ctx context.Context, task, technology, language string, roots []string, limit int) ([]KnowledgeEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	where := []string{"1=1"}
	args := []any{}
	if technology != "" {
		where = append(where, `technology LIKE ?`)
		args = append(args, "%"+technology+"%")
	}
	if language != "" {
		where = append(where, `language LIKE ?`)
		args = append(args, "%"+language+"%")
	}
	if task != "" {
		where = append(where, `(subject LIKE ? OR body_markdown LIKE ?)`)
		args = append(args, "%"+task+"%", "%"+task+"%")
	}
	if len(roots) > 0 {
		ph := make([]string, len(roots))
		for i, r := range roots {
			ph[i] = "?"
			args = append(args, r)
		}
		where = append(where, `root_name IN (`+strings.Join(ph, ",")+`)`)
	}
	args = append(args, limit)
	q := `SELECT id, uri, subject, technology, language, version, body_markdown,
		review_status, authority, confidence, root_name, created_at, verified_at, hash
		FROM knowledge_entries WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY CASE review_status WHEN 'human_reviewed' THEN 0 ELSE 1 END, created_at DESC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KnowledgeEntry
	for rows.Next() {
		var e KnowledgeEntry
		if err := rows.Scan(
			&e.ID, &e.URI, &e.Subject, &e.Technology, &e.Language, &e.Version, &e.BodyMarkdown,
			&e.ReviewStatus, &e.Authority, &e.Confidence, &e.RootName, &e.CreatedAt, &e.VerifiedAt, &e.Hash,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		uris, err := s.listKnowledgeEntrySources(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].SourceURIs = uris
	}
	return out, nil
}

func (s *Store) listKnowledgeEntrySources(ctx context.Context, entryID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_uri FROM knowledge_entry_sources WHERE entry_id = ? ORDER BY source_uri`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uri string
		if err := rows.Scan(&uri); err != nil {
			return nil, err
		}
		out = append(out, uri)
	}
	return out, rows.Err()
}

// SetKnowledgeEntryReviewStatus promotes or demotes a recipe's review status.
// human_reviewed sets authority to curated_internal_recipe and verified_at=now.
func (s *Store) SetKnowledgeEntryReviewStatus(ctx context.Context, uri, status string) error {
	uri = strings.TrimSpace(uri)
	status = strings.TrimSpace(status)
	if uri == "" {
		return fmt.Errorf("uri is required")
	}
	e := KnowledgeEntry{ReviewStatus: status}
	if err := normalizeRecipeAuthority(&e); err != nil {
		return err
	}
	now := time.Now().Unix()
	verified := int64(0)
	if status == ReviewHumanReviewed {
		verified = now
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE knowledge_entries
		SET review_status = ?, authority = ?, verified_at = ?
		WHERE uri = ?`, status, e.Authority, verified, uri)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("knowledge entry not found: %s", uri)
	}
	return nil
}

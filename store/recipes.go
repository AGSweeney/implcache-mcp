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

// UpsertKnowledgeEntry stores a recipe with source lineage.
func (s *Store) UpsertKnowledgeEntry(ctx context.Context, e KnowledgeEntry) (int64, error) {
	if strings.TrimSpace(e.URI) == "" || strings.TrimSpace(e.BodyMarkdown) == "" {
		return 0, fmt.Errorf("uri and bodyMarkdown are required")
	}
	if e.ReviewStatus == "" {
		e.ReviewStatus = ReviewGenerated
	}
	if e.Authority == "" {
		if e.ReviewStatus == ReviewHumanReviewed {
			e.Authority = AuthorityCuratedRecipe
		} else {
			e.Authority = AuthorityGeneratedSummary
		}
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
	return out, rows.Err()
}

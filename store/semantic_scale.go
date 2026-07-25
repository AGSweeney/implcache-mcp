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

// SeedSyntheticSemanticCorpus bulk-inserts n compact chunks for offline scale
// measurement. It bypasses the normal document-replacement upsert path for
// speed and is not a production ingest API.
func (s *Store) SeedSyntheticSemanticCorpus(ctx context.Context, root string, n int) error {
	if n < 1 {
		return fmt.Errorf("n must be positive")
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("root is required")
	}
	const commitEvery = 5000
	now := time.Now().Unix()

	for start := 0; start < n; {
		end := start + commitEvery
		if end > n {
			end = n
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		docStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO documents(uri, title, source_type, path, root_name, mtime, hash,
				authority, technology, language, product_version, deprecated, archived, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', '', '', 0, 0, ?, ?)`)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		chunkStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO chunks(document_id, ordinal, heading, body, start_line, end_line, root_name)
			VALUES (?, 0, ?, ?, 1, 2, ?)`)
		if err != nil {
			docStmt.Close()
			_ = tx.Rollback()
			return err
		}
		for i := start; i < end; i++ {
			body := "network client configuration session deployment guide"
			if i%11 == 0 {
				body = "Reconnect handling uses RetryPolicy and exponential backoff for network client recovery"
			} else if i%5 == 0 {
				body = "retry reconnect timeout handshake policy network recovery procedures"
			}
			uri := fmt.Sprintf("project://%s/doc-%06d.md", root, i)
			path := fmt.Sprintf("doc-%06d.md", i)
			res, err := docStmt.ExecContext(ctx, uri, path, SourceMarkdown, path, root, now,
				fmt.Sprintf("h-%d", i), AuthorityOfficialDocs, now, now)
			if err != nil {
				docStmt.Close()
				chunkStmt.Close()
				_ = tx.Rollback()
				return err
			}
			docID, err := res.LastInsertId()
			if err != nil {
				docStmt.Close()
				chunkStmt.Close()
				_ = tx.Rollback()
				return err
			}
			cres, err := chunkStmt.ExecContext(ctx, docID, "Overview", body, root)
			if err != nil {
				docStmt.Close()
				chunkStmt.Close()
				_ = tx.Rollback()
				return err
			}
			chunkID, err := cres.LastInsertId()
			if err != nil {
				docStmt.Close()
				chunkStmt.Close()
				_ = tx.Rollback()
				return err
			}
			if err := s.upsertChunkTermVector(ctx, tx, chunkID, root, "Overview", body); err != nil {
				docStmt.Close()
				chunkStmt.Close()
				_ = tx.Rollback()
				return err
			}
		}
		docStmt.Close()
		chunkStmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}
		start = end
	}
	return nil
}

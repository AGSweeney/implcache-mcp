// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"strings"
)

func normalizeRootName(root string) string {
	return strings.TrimSpace(root)
}

func adjustRootChunkCountTx(ctx context.Context, tx *sql.Tx, root string, delta int) error {
	if delta == 0 {
		return nil
	}
	root = normalizeRootName(root)
	if delta > 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO root_chunk_stats(root_name, chunk_count)
			VALUES (?, ?)
			ON CONFLICT(root_name) DO UPDATE SET
				chunk_count = root_chunk_stats.chunk_count + excluded.chunk_count`,
			root, delta)
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE root_chunk_stats
		SET chunk_count = chunk_count + ?
		WHERE root_name = ? AND chunk_count + ? > 0`, delta, root, delta)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		DELETE FROM root_chunk_stats WHERE root_name = ?`, root)
	return err
}

func adjustTermDFTx(ctx context.Context, tx *sql.Tx, root, term string, delta int) error {
	if delta == 0 || term == "" {
		return nil
	}
	root = normalizeRootName(root)
	if delta > 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO term_df(root_name, term, df)
			VALUES (?, ?, ?)
			ON CONFLICT(root_name, term) DO UPDATE SET
				df = term_df.df + excluded.df`,
			root, term, delta)
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE term_df SET df = df + ?
		WHERE root_name = ? AND term = ? AND df + ? > 0`, delta, root, term, delta)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// Row would reach zero or below — remove it.
	_, err = tx.ExecContext(ctx, `
		DELETE FROM term_df WHERE root_name = ? AND term = ?`, root, term)
	return err
}

func applyTermsDFDeltaTx(ctx context.Context, tx *sql.Tx, root, terms string, delta int) error {
	if delta == 0 || terms == "" {
		return nil
	}
	for _, term := range strings.Fields(terms) {
		if err := adjustTermDFTx(ctx, tx, root, term, delta); err != nil {
			return err
		}
	}
	return nil
}

// retractDocumentSemanticStats decrements persisted DF / chunk counts for all
// chunks belonging to docID. Must run before those chunks are deleted.
func retractDocumentSemanticStats(ctx context.Context, tx *sql.Tx, docID int64) error {
	return retractDocumentsSemanticStats(ctx, tx, []int64{docID})
}

// retractDocumentsSemanticStats batch-retracts semantic stats for many documents
// in a few grouped queries (much faster than per-document loops on large roots).
func retractDocumentsSemanticStats(ctx context.Context, tx *sql.Tx, docIDs []int64) error {
	if len(docIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(docIDs))
	args := make([]any, len(docIDs))
	for i, id := range docIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	inList := strings.Join(placeholders, ",")

	type delta struct {
		root string
		term string
		n    int
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT p.root_name, p.term, COUNT(*)
		FROM chunk_term_postings p
		JOIN chunks c ON c.id = p.chunk_id
		WHERE c.document_id IN (`+inList+`)
		GROUP BY p.root_name, p.term`, args...)
	if err != nil {
		return err
	}
	var termDeltas []delta
	for rows.Next() {
		var d delta
		if err := rows.Scan(&d.root, &d.term, &d.n); err != nil {
			rows.Close()
			return err
		}
		termDeltas = append(termDeltas, d)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, d := range termDeltas {
		if err := adjustTermDFTx(ctx, tx, d.root, d.term, -d.n); err != nil {
			return err
		}
	}

	rows, err = tx.QueryContext(ctx, `
		SELECT root_name, COUNT(*)
		FROM chunks
		WHERE document_id IN (`+inList+`)
		GROUP BY root_name`, args...)
	if err != nil {
		return err
	}
	var chunkDeltas []delta
	for rows.Next() {
		var d delta
		if err := rows.Scan(&d.root, &d.n); err != nil {
			rows.Close()
			return err
		}
		chunkDeltas = append(chunkDeltas, d)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, d := range chunkDeltas {
		if err := adjustRootChunkCountTx(ctx, tx, d.root, -d.n); err != nil {
			return err
		}
	}
	return nil
}

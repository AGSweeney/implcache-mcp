// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Record enqueues a request event (best-effort, never blocks long).
func (s *Store) Record(ev RequestEvent) {
	defer func() { _ = recover() }()
	if s == nil || !s.Enabled() || s.writer == nil {
		return
	}
	if ev.RequestID == "" {
		return
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}
	cfg := s.Config()
	if !cfg.StoreTaskText {
		ev.TaskSummary = ""
	}
	s.writer.enqueue(ev)
}

// HashTask returns a stable SHA-256 hex of the normalized task text.
func HashTask(task string) string {
	norm := strings.ToLower(strings.TrimSpace(task))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// SessionHash HMAC-hashes a session identifier with the install salt.
func (s *Store) SessionHash(ctx context.Context, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || s == nil {
		return ""
	}
	salt := s.InstallSalt(ctx)
	if salt == "" {
		sum := sha256.Sum256([]byte(raw))
		return hex.EncodeToString(sum[:8])
	}
	mac := hmac.New(sha256.New, []byte(salt))
	_, _ = mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

func (s *Store) insertBatch(ctx context.Context, batch []RequestEvent) error {
	if s.db == nil || len(batch) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := range batch {
		if err := insertOne(ctx, tx, &batch[i]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertOne(ctx context.Context, tx *sql.Tx, ev *RequestEvent) error {
	var redPct any
	if ev.ReductionPct != nil {
		redPct = *ev.ReductionPct
	}
	rootSel, addRet := 0, 0
	if ev.RootSelectionRequired {
		rootSel = 1
	}
	if ev.AdditionalRetrievalRecommended {
		addRet = 1
	}
	returned := ev.ReturnedTokens
	if returned == 0 {
		returned = ev.EstimatedTokens
	}
	if ev.EstimatedTokens == 0 && returned > 0 {
		ev.EstimatedTokens = returned
	}
	estVer := ev.TokenEstimatorVersion
	if estVer == "" && returned > 0 {
		estVer = TokenEstimatorVersion
	}
	var covApp any
	if ev.CoverageApplicable != nil {
		if *ev.CoverageApplicable {
			covApp = 1
		} else {
			covApp = 0
		}
	}
	_, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO request_events (
			request_id, occurred_at, session_hash, client_name, model_name, tool_name,
			task_hash, task_summary, result_status, coverage, freshness,
			latency_ms, estimated_tokens, returned_tokens, structured_tokens, raw_document_tokens,
			estimated_source_tokens, estimated_tokens_avoided,
			context_reduction_percent, token_estimator_version, coverage_applicable, request_class,
			context_fingerprint,
			root_selection_required, additional_retrieval_recommended,
			root_count, source_count, citation_count, curated_count, recipe_count, symbol_count,
			error_category, error_message
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ev.RequestID, ev.OccurredAt.UTC().Format(time.RFC3339Nano),
		nullStr(ev.SessionHash), nullStr(ev.ClientName), nullStr(ev.ModelName), ev.ToolName,
		nullStr(ev.TaskHash), nullStr(ev.TaskSummary), ev.ResultStatus,
		nullStr(ev.Coverage), nullStr(ev.Freshness),
		ev.LatencyMS, ev.EstimatedTokens, returned, ev.StructuredTokens, ev.RawDocumentTokens,
		ev.EstimatedSource, ev.TokensAvoided,
		redPct, nullStr(estVer), covApp, nullStr(ev.RequestClass),
		nullStr(ev.ContextFingerprint),
		rootSel, addRet,
		ev.RootCount, ev.SourceCount, ev.CitationCount, ev.CuratedCount, ev.RecipeCount, ev.SymbolCount,
		nullStr(ev.ErrorCategory), nullStr(ev.ErrorMessage),
	)
	if err != nil {
		return fmt.Errorf("request_events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM request_roots WHERE request_id=?`, ev.RequestID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM evidence_events WHERE request_id=?`, ev.RequestID); err != nil {
		return err
	}
	for _, r := range ev.Roots {
		sel := 0
		if r.Selected {
			sel = 1
		}
		name := r.RootName
		if name == "" {
			name = r.RootKey
		}
		key := r.RootKey
		if key == "" {
			key = name
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO request_roots(request_id, root_key, root_name, root_group_key, root_role, selected)
			VALUES(?,?,?,?,?,?)`,
			ev.RequestID, nullStr(key), nullStr(name), nullStr(r.RootGroupKey), nullStr(r.RootRole), sel,
		); err != nil {
			return err
		}
	}
	for _, e := range ev.Evidence {
		sel, trim := 0, 0
		if e.SelectedForPackage {
			sel = 1
		}
		if e.IncludedAfterTrimming {
			trim = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evidence_events(
				request_id, evidence_type, evidence_key, root_key, source_uri, authority,
				rank_position, selected_for_package, included_after_trimming, estimated_tokens, source_hash)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			ev.RequestID, e.EvidenceType, nullStr(e.EvidenceKey), nullStr(e.RootKey), nullStr(e.SourceURI),
			nullStr(e.Authority), e.RankPosition, sel, trim, e.EstimatedTokens, nullStr(e.SourceHash),
		); err != nil {
			return err
		}
	}
	return nil
}

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

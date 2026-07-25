// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RepoSource is a registered Git repository knowledge source.
type RepoSource struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	RootName            string   `json:"rootName"`
	RemoteURL           string   `json:"remoteUrl"`
	LocalPath           string   `json:"localPath,omitempty"`
	Provider            string   `json:"provider,omitempty"`
	Owner               string   `json:"owner,omitempty"`
	Repository          string   `json:"repository,omitempty"`
	AcquisitionMode     string   `json:"acquisitionMode"`
	RequestedRef        string   `json:"requestedRef"`
	ResolvedCommitSHA   string   `json:"resolvedCommitSha"`
	DefaultBranch       string   `json:"defaultBranch,omitempty"`
	Authority           string   `json:"authority"`
	Product             string   `json:"product,omitempty"`
	Version             string   `json:"version,omitempty"`
	CredentialReference string   `json:"credentialReference,omitempty"`
	IncludePatterns     []string `json:"includePatterns,omitempty"`
	ExcludePatterns     []string `json:"excludePatterns,omitempty"`
	SparsePaths         []string `json:"sparsePaths,omitempty"`
	SubmodulePolicy     string   `json:"submodulePolicy"`
	SymlinkPolicy       string   `json:"symlinkPolicy"`
	WorkingTreeMode     string   `json:"workingTreeMode"`
	CloneDepth          int      `json:"cloneDepth"`
	PartialCloneFilter  string   `json:"partialCloneFilter,omitempty"`
	CheckoutPath        string   `json:"checkoutPath,omitempty"`
	Enabled             bool     `json:"enabled"`
	LastAttemptAt       int64    `json:"lastAttemptAt"`
	LastSuccessAt       int64    `json:"lastSuccessAt"`
	LastStatus          string   `json:"lastStatus"`
	CreatedAt           int64    `json:"createdAt"`
	UpdatedAt           int64    `json:"updatedAt"`
}

// RepoFile tracks one indexed path under a repo source.
type RepoFile struct {
	ID                 int64  `json:"id"`
	RepoSourceID       int64  `json:"repoSourceId"`
	DocumentID         int64  `json:"documentId,omitempty"`
	RelativePath       string `json:"relativePath"`
	BlobHash           string `json:"blobHash,omitempty"`
	ContentHash        string `json:"contentHash"`
	Language           string `json:"language,omitempty"`
	ContentClass       string `json:"contentClass"`
	FileSize           int64  `json:"fileSize"`
	ResolvedCommitSHA  string `json:"resolvedCommitSha"`
	LastSeenGeneration int64  `json:"lastSeenGeneration"`
}

// UpsertRepoSource creates or updates a repo source by name.
func (s *Store) UpsertRepoSource(ctx context.Context, in RepoSource) (int64, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return 0, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(in.RootName) == "" {
		return 0, fmt.Errorf("rootName is required")
	}
	if strings.TrimSpace(in.RemoteURL) == "" && strings.TrimSpace(in.LocalPath) == "" {
		return 0, fmt.Errorf("remoteUrl or localPath is required")
	}
	if in.AcquisitionMode == "" {
		in.AcquisitionMode = "snapshot"
	}
	if in.Authority == "" {
		in.Authority = AuthorityCurrentProject
	}
	if in.SubmodulePolicy == "" {
		in.SubmodulePolicy = "ignore"
	}
	if in.SymlinkPolicy == "" {
		in.SymlinkPolicy = "ignore"
	}
	if in.WorkingTreeMode == "" {
		in.WorkingTreeMode = "HEAD"
	}
	if in.CloneDepth <= 0 {
		in.CloneDepth = 1
	}
	inc, err := json.Marshal(in.IncludePatterns)
	if err != nil {
		return 0, err
	}
	exc, err := json.Marshal(in.ExcludePatterns)
	if err != nil {
		return 0, err
	}
	sparse, err := json.Marshal(in.SparsePaths)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	enabled := 1
	if in.ID != 0 && !in.Enabled {
		enabled = 0
	}

	var id int64
	err = s.db.QueryRowContext(ctx, `SELECT id FROM repo_sources WHERE name = ?`, name).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO repo_sources(
				name, root_name, remote_url, local_path, provider, owner, repository,
				acquisition_mode, requested_ref, resolved_commit_sha, default_branch,
				authority, product, version, credential_reference,
				include_patterns, exclude_patterns, sparse_paths,
				submodule_policy, symlink_policy, working_tree_mode,
				clone_depth, partial_clone_filter, checkout_path, enabled,
				created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			name, in.RootName, in.RemoteURL, in.LocalPath, in.Provider, in.Owner, in.Repository,
			in.AcquisitionMode, in.RequestedRef, in.ResolvedCommitSHA, in.DefaultBranch,
			in.Authority, in.Product, in.Version, in.CredentialReference,
			string(inc), string(exc), string(sparse),
			in.SubmodulePolicy, in.SymlinkPolicy, in.WorkingTreeMode,
			in.CloneDepth, in.PartialCloneFilter, in.CheckoutPath, enabled,
			now, now,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	case err != nil:
		return 0, err
	default:
		_, err = s.db.ExecContext(ctx, `
			UPDATE repo_sources SET
				root_name = ?, remote_url = ?, local_path = ?, provider = ?, owner = ?, repository = ?,
				acquisition_mode = ?, requested_ref = ?, resolved_commit_sha = ?, default_branch = ?,
				authority = ?, product = ?, version = ?, credential_reference = ?,
				include_patterns = ?, exclude_patterns = ?, sparse_paths = ?,
				submodule_policy = ?, symlink_policy = ?, working_tree_mode = ?,
				clone_depth = ?, partial_clone_filter = ?, checkout_path = ?, enabled = ?,
				updated_at = ?
			WHERE id = ?`,
			in.RootName, in.RemoteURL, in.LocalPath, in.Provider, in.Owner, in.Repository,
			in.AcquisitionMode, in.RequestedRef, in.ResolvedCommitSHA, in.DefaultBranch,
			in.Authority, in.Product, in.Version, in.CredentialReference,
			string(inc), string(exc), string(sparse),
			in.SubmodulePolicy, in.SymlinkPolicy, in.WorkingTreeMode,
			in.CloneDepth, in.PartialCloneFilter, in.CheckoutPath, enabled,
			now, id,
		)
		return id, err
	}
}

// GetRepoSourceByName loads a repo source.
func (s *Store) GetRepoSourceByName(ctx context.Context, name string) (*RepoSource, error) {
	row := s.db.QueryRowContext(ctx, repoSourceSelect+` WHERE name = ?`, name)
	return scanRepoSource(row)
}

// GetRepoSourceByID loads a repo source by id.
func (s *Store) GetRepoSourceByID(ctx context.Context, id int64) (*RepoSource, error) {
	row := s.db.QueryRowContext(ctx, repoSourceSelect+` WHERE id = ?`, id)
	return scanRepoSource(row)
}

// ListRepoSources returns all registered sources.
func (s *Store) ListRepoSources(ctx context.Context) ([]RepoSource, error) {
	rows, err := s.db.QueryContext(ctx, repoSourceSelect+` ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RepoSource
	for rows.Next() {
		rs, err := scanRepoSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rs)
	}
	return out, rows.Err()
}

// SetRepoSourceStatus updates attempt/success timestamps and status.
func (s *Store) SetRepoSourceStatus(ctx context.Context, id int64, status string, success bool) error {
	now := time.Now().Unix()
	if success {
		_, err := s.db.ExecContext(ctx, `
			UPDATE repo_sources SET last_attempt_at = ?, last_success_at = ?, last_status = ?, updated_at = ?
			WHERE id = ?`, now, now, status, now, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE repo_sources SET last_attempt_at = ?, last_status = ?, updated_at = ?
		WHERE id = ?`, now, status, now, id)
	return err
}

// UpdateRepoSourceCommit updates resolved commit and checkout path after successful ingest.
func (s *Store) UpdateRepoSourceCommit(ctx context.Context, id int64, sha, checkoutPath, status string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE repo_sources SET
			resolved_commit_sha = ?, checkout_path = ?,
			last_attempt_at = ?, last_success_at = ?, last_status = ?, updated_at = ?
		WHERE id = ?`,
		sha, checkoutPath, now, now, status, now, id)
	return err
}

// UpsertRepoFile inserts or updates a repo file row.
func (s *Store) UpsertRepoFile(ctx context.Context, f RepoFile) (int64, error) {
	if f.RepoSourceID == 0 || strings.TrimSpace(f.RelativePath) == "" {
		return 0, fmt.Errorf("repoSourceId and relativePath are required")
	}
	if f.ContentClass == "" {
		f.ContentClass = "unknown"
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM repo_files WHERE repo_source_id = ? AND relative_path = ?`,
		f.RepoSourceID, f.RelativePath).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO repo_files(
				repo_source_id, document_id, relative_path, blob_hash, content_hash,
				language, content_class, file_size, resolved_commit_sha, last_seen_generation)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.RepoSourceID, nullInt64(f.DocumentID), f.RelativePath, f.BlobHash, f.ContentHash,
			f.Language, f.ContentClass, f.FileSize, f.ResolvedCommitSHA, f.LastSeenGeneration,
		)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	case err != nil:
		return 0, err
	default:
		_, err = s.db.ExecContext(ctx, `
			UPDATE repo_files SET
				document_id = ?, blob_hash = ?, content_hash = ?, language = ?, content_class = ?,
				file_size = ?, resolved_commit_sha = ?, last_seen_generation = ?
			WHERE id = ?`,
			nullInt64(f.DocumentID), f.BlobHash, f.ContentHash, f.Language, f.ContentClass,
			f.FileSize, f.ResolvedCommitSHA, f.LastSeenGeneration, id,
		)
		return id, err
	}
}

// ListRepoFiles returns files for a source.
func (s *Store) ListRepoFiles(ctx context.Context, sourceID int64) ([]RepoFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo_source_id, COALESCE(document_id, 0), relative_path, blob_hash, content_hash,
			language, content_class, file_size, resolved_commit_sha, last_seen_generation
		FROM repo_files WHERE repo_source_id = ? ORDER BY relative_path`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RepoFile
	for rows.Next() {
		var f RepoFile
		if err := rows.Scan(
			&f.ID, &f.RepoSourceID, &f.DocumentID, &f.RelativePath, &f.BlobHash, &f.ContentHash,
			&f.Language, &f.ContentClass, &f.FileSize, &f.ResolvedCommitSHA, &f.LastSeenGeneration,
		); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DeleteRepoFilesNotSeen deletes files (and documents) not seen in generation.
func (s *Store) DeleteRepoFilesNotSeen(ctx context.Context, sourceID, generation int64) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(document_id, 0), relative_path FROM repo_files
		WHERE repo_source_id = ? AND last_seen_generation < ?`, sourceID, generation)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type row struct {
		id, docID int64
		path      string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.docID, &r.path); err != nil {
			return 0, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var n int64
	for _, r := range list {
		if r.docID > 0 {
			if _, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, r.docID); err != nil {
				return n, err
			}
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM repo_files WHERE id = ?`, r.id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// DeleteRepoFileByPath removes one file row and its document.
func (s *Store) DeleteRepoFileByPath(ctx context.Context, sourceID int64, rel string) error {
	var docID int64
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(document_id, 0) FROM repo_files
		WHERE repo_source_id = ? AND relative_path = ?`, sourceID, rel).Scan(&id, &docID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if docID > 0 {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, docID); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM repo_files WHERE id = ?`, id)
	return err
}

// DeleteRepoSource removes config, files, and documents under the root URI prefix.
func (s *Store) DeleteRepoSource(ctx context.Context, name string, removeIndex, removeCloneMeta bool) (bool, error) {
	rs, err := s.GetRepoSourceByName(ctx, name)
	if err != nil {
		return false, err
	}
	if removeIndex {
		prefix := "git://" + rs.RootName + "/"
		if _, err := s.DeleteDocumentsByURIPrefix(ctx, prefix); err != nil {
			return false, err
		}
	}
	if removeCloneMeta || removeIndex {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM repo_sources WHERE id = ?`, rs.ID); err != nil {
			return false, err
		}
		return true, nil
	}
	// Config-only soft disable
	_, err = s.db.ExecContext(ctx, `UPDATE repo_sources SET enabled = 0, updated_at = ? WHERE id = ?`,
		time.Now().Unix(), rs.ID)
	return err == nil, err
}

// NextRepoGeneration returns max(generation)+1.
func (s *Store) NextRepoGeneration(ctx context.Context, sourceID int64) (int64, error) {
	var g sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(last_seen_generation) FROM repo_files WHERE repo_source_id = ?`, sourceID).Scan(&g)
	if err != nil {
		return 0, err
	}
	if !g.Valid {
		return 1, nil
	}
	return g.Int64 + 1, nil
}

const repoSourceSelect = `
	SELECT id, name, root_name, remote_url, local_path, provider, owner, repository,
		acquisition_mode, requested_ref, resolved_commit_sha, default_branch,
		authority, product, version, credential_reference,
		include_patterns, exclude_patterns, sparse_paths,
		submodule_policy, symlink_policy, working_tree_mode,
		clone_depth, partial_clone_filter, checkout_path, enabled,
		last_attempt_at, last_success_at, last_status, created_at, updated_at
	FROM repo_sources`

func scanRepoSource(row scannable) (*RepoSource, error) {
	var rs RepoSource
	var inc, exc, sparse string
	var enabled int
	err := row.Scan(
		&rs.ID, &rs.Name, &rs.RootName, &rs.RemoteURL, &rs.LocalPath, &rs.Provider, &rs.Owner, &rs.Repository,
		&rs.AcquisitionMode, &rs.RequestedRef, &rs.ResolvedCommitSHA, &rs.DefaultBranch,
		&rs.Authority, &rs.Product, &rs.Version, &rs.CredentialReference,
		&inc, &exc, &sparse,
		&rs.SubmodulePolicy, &rs.SymlinkPolicy, &rs.WorkingTreeMode,
		&rs.CloneDepth, &rs.PartialCloneFilter, &rs.CheckoutPath, &enabled,
		&rs.LastAttemptAt, &rs.LastSuccessAt, &rs.LastStatus, &rs.CreatedAt, &rs.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repo source not found")
		}
		return nil, err
	}
	rs.Enabled = enabled != 0
	_ = json.Unmarshal([]byte(inc), &rs.IncludePatterns)
	_ = json.Unmarshal([]byte(exc), &rs.ExcludePatterns)
	_ = json.Unmarshal([]byte(sparse), &rs.SparsePaths)
	return &rs, nil
}

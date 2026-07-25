// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/store"
)

// IngestOptions configures repository ingestion.
type IngestOptions struct {
	Name               string
	RemoteURL          string
	LocalPath          string
	RootName           string
	AcquisitionMode    string
	Ref                string
	Authority          string
	Product            string
	Version            string
	CredentialRef      string
	IncludePatterns    []string
	ExcludePatterns    []string
	SparsePaths        []string
	SubmodulePolicy    string
	SymlinkPolicy      string
	WorkingTreeMode    string
	CloneDepth         int
	PartialCloneFilter string
	CacheRoot          string
	MaxFiles           int
	MaxDocumentBytes   int64
	MaxTotalBytes      int64
	PersistSource      bool // upsert repo_sources
	Runner             *Runner
}

// IngestReport is returned from ingest/refresh.
type IngestReport struct {
	SourceName        string   `json:"sourceName"`
	RootName          string   `json:"rootName"`
	RemoteURL         string   `json:"remoteUrl,omitempty"`
	AcquisitionMode   string   `json:"acquisitionMode"`
	RequestedRef      string   `json:"requestedRef"`
	PreviousCommit    string   `json:"previousCommit,omitempty"`
	ResolvedCommit    string   `json:"resolvedCommit"`
	CheckoutPath      string   `json:"checkoutPath"`
	FilesDiscovered   int      `json:"filesDiscovered"`
	DocumentsIngested int      `json:"documentsIngested"`
	FilesSkipped      int      `json:"filesSkipped"`
	FilesAdded        int      `json:"filesAdded,omitempty"`
	FilesModified     int      `json:"filesModified,omitempty"`
	FilesDeleted      int      `json:"filesDeleted,omitempty"`
	SymbolsHint       string   `json:"symbolsNote,omitempty"`
	BytesProcessed    int64    `json:"bytesProcessed"`
	DurationMS        int64    `json:"durationMs"`
	Warnings          []string `json:"warnings,omitempty"`
	Status            string   `json:"status"`
	WorkingTreeDirty  bool     `json:"workingTreeDirty,omitempty"`
}

// IngestRepo acquires a repo state and indexes it via the local-tree ingester.
func IngestRepo(ctx context.Context, st *store.Store, opt IngestOptions) (*IngestReport, error) {
	start := time.Now()
	if opt.Name == "" {
		opt.Name = "repo"
	}
	if opt.RootName == "" {
		opt.RootName = opt.Name
	}
	if opt.AcquisitionMode == "" {
		if opt.LocalPath != "" {
			opt.AcquisitionMode = "local_checkout"
		} else {
			opt.AcquisitionMode = "snapshot"
		}
	}
	if opt.Authority == "" {
		opt.Authority = store.AuthorityCurrentProject
	}
	if opt.SubmodulePolicy == "" {
		opt.SubmodulePolicy = "ignore"
	}
	if opt.SymlinkPolicy == "" {
		opt.SymlinkPolicy = "ignore"
	}

	var prevCommit string
	var sourceID int64
	if opt.PersistSource {
		existing, err := st.GetRepoSourceByName(ctx, opt.Name)
		if err == nil {
			prevCommit = existing.ResolvedCommitSHA
			sourceID = existing.ID
		}
		id, err := st.UpsertRepoSource(ctx, store.RepoSource{
			Name: opt.Name, RootName: opt.RootName, RemoteURL: opt.RemoteURL, LocalPath: opt.LocalPath,
			AcquisitionMode: opt.AcquisitionMode, RequestedRef: opt.Ref, ResolvedCommitSHA: prevCommit,
			Authority: opt.Authority, Product: opt.Product, Version: opt.Version,
			CredentialReference: opt.CredentialRef, IncludePatterns: opt.IncludePatterns,
			ExcludePatterns: opt.ExcludePatterns, SparsePaths: opt.SparsePaths,
			SubmodulePolicy: opt.SubmodulePolicy, SymlinkPolicy: opt.SymlinkPolicy,
			WorkingTreeMode: opt.WorkingTreeMode, CloneDepth: opt.CloneDepth,
			PartialCloneFilter: opt.PartialCloneFilter, Enabled: true,
		})
		if err != nil {
			return nil, err
		}
		sourceID = id
		_ = st.SetRepoSourceStatus(ctx, sourceID, "running", false)
	}

	co, err := PrepareCheckout(ctx, SnapshotOptions{
		RemoteURL: opt.RemoteURL, LocalPath: opt.LocalPath, Ref: opt.Ref,
		CacheRoot: opt.CacheRoot, SourceName: opt.Name, AcquisitionMode: opt.AcquisitionMode,
		CloneDepth: opt.CloneDepth, PartialCloneFilter: opt.PartialCloneFilter,
		SparsePaths: opt.SparsePaths, WorkingTreeMode: opt.WorkingTreeMode, Runner: opt.Runner,
	})
	if err != nil {
		if sourceID != 0 {
			_ = st.SetRepoSourceStatus(ctx, sourceID, "failed:"+err.Error(), false)
		}
		return nil, err
	}
	cleanupTemp := opt.AcquisitionMode == "snapshot" || (opt.AcquisitionMode == "local_checkout" && opt.WorkingTreeMode != "working_tree" && co.Path != opt.LocalPath)
	if cleanupTemp && !co.Managed {
		defer func() {
			if strings.Contains(co.Path, "implcache-git-head-") || strings.Contains(co.Path, opt.Name+"-") {
				_ = os.RemoveAll(co.Path)
			}
		}()
	}

	inc, exc := opt.IncludePatterns, opt.ExcludePatterns
	if len(exc) == 0 {
		exc = DefaultExcludePatterns
	}
	filter := func(rel string) bool {
		return PathAllowed(rel, inc, exc)
	}

	pres, err := ingest.IngestProjectOpts(ctx, st, ingest.ProjectOptions{
		Path: co.Path, RootName: opt.RootName,
		MaxFiles: opt.MaxFiles, MaxDocumentBytes: opt.MaxDocumentBytes, MaxTotalBytes: opt.MaxTotalBytes,
		PathFilter: filter, URIScheme: "git", SourceType: store.SourceGit, Authority: opt.Authority,
		SkipDirNames: map[string]struct{}{".git": {}},
	})
	if err != nil {
		if sourceID != 0 {
			_ = st.SetRepoSourceStatus(ctx, sourceID, "failed:"+err.Error(), false)
		}
		return nil, err
	}

	gen := int64(1)
	if sourceID != 0 {
		if g, err := st.NextRepoGeneration(ctx, sourceID); err == nil {
			gen = g
		}
	}
	for _, f := range pres.Files {
		if sourceID == 0 {
			break
		}
		docID := int64(0)
		if d, _, err := st.GetDocumentByURI(ctx, f.URI); err == nil {
			docID = d.ID
		}
		_, _ = st.UpsertRepoFile(ctx, store.RepoFile{
			RepoSourceID: sourceID, DocumentID: docID, RelativePath: f.RelativePath,
			ContentHash: f.ContentHash, Language: f.Language, ContentClass: ClassifyPath(f.RelativePath),
			FileSize: f.FileSize, ResolvedCommitSHA: co.ResolvedCommitSHA, LastSeenGeneration: gen,
		})
	}
	if sourceID != 0 {
		if _, err := st.DeleteRepoFilesNotSeen(ctx, sourceID, gen); err != nil {
			return nil, err
		}
		n, _ := NormalizeRemoteURL(co.CloneURL)
		if co.CloneURL == "" && opt.RemoteURL != "" {
			n, _ = NormalizeRemoteURL(opt.RemoteURL)
		}
		provider, owner, repo := "", "", ""
		if n != nil {
			provider, owner, repo = n.Provider, n.Owner, n.Repository
		}
		_, _ = st.UpsertRepoSource(ctx, store.RepoSource{
			Name: opt.Name, RootName: opt.RootName, RemoteURL: redactSecrets(firstNonEmpty(co.CloneURL, opt.RemoteURL)),
			LocalPath: opt.LocalPath, Provider: provider, Owner: owner, Repository: repo,
			AcquisitionMode: opt.AcquisitionMode, RequestedRef: co.RequestedRef,
			ResolvedCommitSHA: co.ResolvedCommitSHA, Authority: opt.Authority, Product: opt.Product,
			Version: opt.Version, CredentialReference: opt.CredentialRef,
			IncludePatterns: opt.IncludePatterns, ExcludePatterns: opt.ExcludePatterns,
			SparsePaths: opt.SparsePaths, SubmodulePolicy: opt.SubmodulePolicy,
			SymlinkPolicy: opt.SymlinkPolicy, WorkingTreeMode: opt.WorkingTreeMode,
			CloneDepth: opt.CloneDepth, PartialCloneFilter: opt.PartialCloneFilter,
			CheckoutPath: co.Path, Enabled: true,
		})
		_ = st.UpdateRepoSourceCommit(ctx, sourceID, co.ResolvedCommitSHA, co.Path, "ok")
	}

	dirty := false
	if opt.AcquisitionMode == "local_checkout" && opt.WorkingTreeMode == "working_tree" {
		dirty = true
	}

	return &IngestReport{
		SourceName: opt.Name, RootName: opt.RootName,
		RemoteURL:       redactSecrets(firstNonEmpty(co.CloneURL, opt.RemoteURL)),
		AcquisitionMode: opt.AcquisitionMode, RequestedRef: co.RequestedRef,
		PreviousCommit: prevCommit, ResolvedCommit: co.ResolvedCommitSHA,
		CheckoutPath: co.Path, FilesDiscovered: len(pres.Files),
		DocumentsIngested: pres.Ingested, FilesSkipped: pres.Skipped,
		BytesProcessed: pres.BytesProcessed, DurationMS: time.Since(start).Milliseconds(),
		Warnings: append([]string{}, pres.Errors...), Status: "ingested",
		WorkingTreeDirty: dirty,
	}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// AddRepoSource persists configuration without necessarily ingesting.
func AddRepoSource(ctx context.Context, st *store.Store, opt IngestOptions) (*store.RepoSource, error) {
	opt.PersistSource = true
	if opt.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if opt.RootName == "" {
		opt.RootName = opt.Name
	}
	mode := opt.AcquisitionMode
	if mode == "" {
		if opt.LocalPath != "" {
			mode = "local_checkout"
		} else {
			mode = "managed_clone"
		}
	}
	var provider, owner, repo string
	remote := opt.RemoteURL
	if remote != "" {
		n, err := NormalizeRemoteURL(remote)
		if err != nil {
			return nil, err
		}
		remote = n.CloneURL
		provider, owner, repo = n.Provider, n.Owner, n.Repository
	}
	id, err := st.UpsertRepoSource(ctx, store.RepoSource{
		Name: opt.Name, RootName: opt.RootName, RemoteURL: redactSecrets(remote), LocalPath: opt.LocalPath,
		Provider: provider, Owner: owner, Repository: repo, AcquisitionMode: mode,
		RequestedRef: opt.Ref, Authority: firstNonEmpty(opt.Authority, store.AuthorityCurrentProject),
		Product: opt.Product, Version: opt.Version, CredentialReference: opt.CredentialRef,
		IncludePatterns: opt.IncludePatterns, ExcludePatterns: opt.ExcludePatterns,
		SparsePaths: opt.SparsePaths, SubmodulePolicy: firstNonEmpty(opt.SubmodulePolicy, "ignore"),
		SymlinkPolicy:   firstNonEmpty(opt.SymlinkPolicy, "ignore"),
		WorkingTreeMode: firstNonEmpty(opt.WorkingTreeMode, "HEAD"),
		CloneDepth:      opt.CloneDepth, PartialCloneFilter: opt.PartialCloneFilter, Enabled: true,
	})
	if err != nil {
		return nil, err
	}
	return st.GetRepoSourceByID(ctx, id)
}

// CacheRootForDB returns a default cache directory beside the database file.
func CacheRootForDB(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		dir = "."
	}
	return filepath.Join(dir, ".implcache", "repos")
}

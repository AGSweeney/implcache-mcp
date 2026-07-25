// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/store"
)

// RefreshRepoSource fetches and incrementally reindexes a managed (or local) source.
func RefreshRepoSource(ctx context.Context, st *store.Store, name string, cacheRoot string, runner *Runner) (*IngestReport, error) {
	start := time.Now()
	rs, err := st.GetRepoSourceByName(ctx, name)
	if err != nil {
		return nil, err
	}
	_ = st.SetRepoSourceStatus(ctx, rs.ID, "refreshing", false)
	prev := rs.ResolvedCommitSHA

	switch rs.AcquisitionMode {
	case "snapshot":
		return &IngestReport{
			SourceName: rs.Name, RootName: rs.RootName, RemoteURL: rs.RemoteURL,
			AcquisitionMode: rs.AcquisitionMode, RequestedRef: rs.RequestedRef,
			PreviousCommit: prev, ResolvedCommit: prev, Status: "unchanged",
			DurationMS: time.Since(start).Milliseconds(),
			Warnings:   []string{"snapshot sources tied to a ref report unchanged on refresh unless re-ingested"},
		}, nil
	case "local_checkout":
		return refreshLocal(ctx, st, rs, start, runner)
	default:
		return refreshManaged(ctx, st, rs, cacheRoot, prev, start, runner)
	}
}

func refreshLocal(ctx context.Context, st *store.Store, rs *store.RepoSource, start time.Time, runner *Runner) (*IngestReport, error) {
	rep, err := IngestRepo(ctx, st, IngestOptions{
		Name: rs.Name, LocalPath: rs.LocalPath, RootName: rs.RootName,
		AcquisitionMode: "local_checkout", Ref: rs.RequestedRef, Authority: rs.Authority,
		Product: rs.Product, Version: rs.Version, IncludePatterns: rs.IncludePatterns,
		ExcludePatterns: rs.ExcludePatterns, WorkingTreeMode: rs.WorkingTreeMode,
		PersistSource: true, Runner: runner,
	})
	if err != nil {
		_ = st.SetRepoSourceStatus(ctx, rs.ID, "failed:"+err.Error(), false)
		return nil, err
	}
	rep.DurationMS = time.Since(start).Milliseconds()
	return rep, nil
}

func refreshManaged(ctx context.Context, st *store.Store, rs *store.RepoSource, cacheRoot, prev string, start time.Time, runner *Runner) (*IngestReport, error) {
	if runner == nil {
		runner = &Runner{}
	}
	if cacheRoot == "" {
		cacheRoot = filepath.Join(".", ".implcache", "repos")
	}
	co, err := PrepareCheckout(ctx, SnapshotOptions{
		RemoteURL: rs.RemoteURL, Ref: rs.RequestedRef, CacheRoot: cacheRoot,
		SourceName: rs.Name, AcquisitionMode: "managed_clone",
		CloneDepth: rs.CloneDepth, PartialCloneFilter: rs.PartialCloneFilter,
		SparsePaths: rs.SparsePaths, Runner: runner,
	})
	if err != nil {
		_ = st.SetRepoSourceStatus(ctx, rs.ID, "failed:"+err.Error(), false)
		// prior root remains
		return nil, err
	}
	if prev != "" && prev == co.ResolvedCommitSHA {
		_ = st.SetRepoSourceStatus(ctx, rs.ID, "ok", true)
		return &IngestReport{
			SourceName: rs.Name, RootName: rs.RootName, RemoteURL: rs.RemoteURL,
			AcquisitionMode: rs.AcquisitionMode, RequestedRef: rs.RequestedRef,
			PreviousCommit: prev, ResolvedCommit: prev, CheckoutPath: co.Path,
			Status: "unchanged", DurationMS: time.Since(start).Milliseconds(),
		}, nil
	}

	inc, exc := rs.IncludePatterns, rs.ExcludePatterns
	if len(exc) == 0 {
		exc = DefaultExcludePatterns
	}
	filter := func(rel string) bool { return PathAllowed(rel, inc, exc) }

	var only []string
	var deleted []string
	if prev != "" {
		added, modified, del, err := DiffNameStatus(ctx, runner, co.Path, prev, co.ResolvedCommitSHA)
		if err != nil {
			// full rescan fallback
			only = nil
		} else {
			only = append(added, modified...)
			deleted = del
		}
	}

	projOpt := ingest.ProjectOptions{
		Path: co.Path, RootName: rs.RootName, PathFilter: filter,
		URIScheme: "git", SourceType: store.SourceGit, Authority: rs.Authority,
		SkipDirNames: map[string]struct{}{".git": {}},
	}
	if len(only) > 0 {
		projOpt.OnlyRelativePaths = only
	}
	pres, err := ingest.IngestProjectOpts(ctx, st, projOpt)
	if err != nil {
		_ = st.SetRepoSourceStatus(ctx, rs.ID, "failed:"+err.Error(), false)
		return nil, err
	}

	for _, p := range deleted {
		_ = st.DeleteRepoFileByPath(ctx, rs.ID, filepath.ToSlash(p))
		uri := ingest.GitURI(rs.RootName, p)
		_, _ = st.DeleteDocument(ctx, uri)
	}

	gen, _ := st.NextRepoGeneration(ctx, rs.ID)
	for _, f := range pres.Files {
		docID := int64(0)
		if d, _, err := st.GetDocumentByURI(ctx, f.URI); err == nil {
			docID = d.ID
		}
		_, _ = st.UpsertRepoFile(ctx, store.RepoFile{
			RepoSourceID: rs.ID, DocumentID: docID, RelativePath: f.RelativePath,
			ContentHash: f.ContentHash, Language: f.Language, ContentClass: ClassifyPath(f.RelativePath),
			FileSize: f.FileSize, ResolvedCommitSHA: co.ResolvedCommitSHA, LastSeenGeneration: gen,
		})
	}
	// Mark all still-present files as seen when full scan
	if len(only) == 0 {
		files, _ := st.ListRepoFiles(ctx, rs.ID)
		for _, f := range files {
			f.LastSeenGeneration = gen
			f.ResolvedCommitSHA = co.ResolvedCommitSHA
			_, _ = st.UpsertRepoFile(ctx, f)
		}
		_, _ = st.DeleteRepoFilesNotSeen(ctx, rs.ID, gen)
	}

	if err := st.UpdateRepoSourceCommit(ctx, rs.ID, co.ResolvedCommitSHA, co.Path, "ok"); err != nil {
		return nil, err
	}

	return &IngestReport{
		SourceName: rs.Name, RootName: rs.RootName, RemoteURL: rs.RemoteURL,
		AcquisitionMode: rs.AcquisitionMode, RequestedRef: rs.RequestedRef,
		PreviousCommit: prev, ResolvedCommit: co.ResolvedCommitSHA, CheckoutPath: co.Path,
		FilesDiscovered: len(pres.Files), DocumentsIngested: pres.Ingested, FilesSkipped: pres.Skipped,
		FilesAdded: len(only), FilesDeleted: len(deleted), BytesProcessed: pres.BytesProcessed,
		DurationMS: time.Since(start).Milliseconds(), Status: "refreshed",
		Warnings: append([]string{}, pres.Errors...),
	}, nil
}

// RemoveRepoSource deletes source data per flags.
func RemoveRepoSource(ctx context.Context, st *store.Store, name string, removeIndex, removeClone bool) (bool, error) {
	rs, err := st.GetRepoSourceByName(ctx, name)
	if err != nil {
		return false, err
	}
	if removeClone && rs.CheckoutPath != "" && rs.AcquisitionMode != "local_checkout" {
		_ = os.RemoveAll(rs.CheckoutPath)
	}
	return st.DeleteRepoSource(ctx, name, removeIndex, true)
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"implcache-mcp/gitrepo"
	"implcache-mcp/librarian"
)

type gitIngestRequest struct {
	Name                string   `json:"name"`
	RemoteURL           string   `json:"remoteUrl,omitempty"`
	LocalPath           string   `json:"localPath,omitempty"`
	RootName            string   `json:"rootName,omitempty"`
	AcquisitionMode     string   `json:"acquisitionMode,omitempty"`
	Ref                 string   `json:"ref,omitempty"`
	Authority           string   `json:"authority,omitempty"`
	Product             string   `json:"product,omitempty"`
	Version             string   `json:"version,omitempty"`
	CredentialReference string   `json:"credentialReference,omitempty"`
	IncludePatterns     []string `json:"includePatterns,omitempty"`
	ExcludePatterns     []string `json:"excludePatterns,omitempty"`
	SparsePaths         []string `json:"sparsePaths,omitempty"`
	SubmodulePolicy     string   `json:"submodulePolicy,omitempty"`
	SymlinkPolicy       string   `json:"symlinkPolicy,omitempty"`
	WorkingTreeMode     string   `json:"workingTreeMode,omitempty"`
	CloneDepth          int      `json:"cloneDepth,omitempty"`
	PartialCloneFilter  string   `json:"partialCloneFilter,omitempty"`
	LibraryDocsHandling string   `json:"libraryDocsHandling,omitempty"` // auto|normal|exclude
}

// handleGitIngest acquires and indexes a Git repository as a tracked async job.
func (h *handler) handleGitIngest(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	var req gitIngestRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	mode := req.AcquisitionMode
	if mode == "" {
		if req.LocalPath != "" {
			mode = "local_checkout"
		} else {
			mode = "snapshot"
		}
	}

	src := librarian.SourceRef{Kind: librarian.KindRepo, ID: req.Name, RootName: firstNonEmpty(req.RootName, req.Name)}
	opID := h.opt.Tracker.Start(src, "ingest")
	jobCtx, cancel := context.WithCancel(context.Background())
	h.opt.Tracker.SetCancel(opID, cancel)

	go func() {
		defer cancel()
		res, err := gitrepo.IngestRepo(jobCtx, h.opt.Store, gitrepo.IngestOptions{
			Name: req.Name, RemoteURL: req.RemoteURL, LocalPath: req.LocalPath,
			RootName: req.RootName, AcquisitionMode: mode, Ref: req.Ref,
			Authority: req.Authority, Product: req.Product, Version: req.Version,
			CredentialRef: req.CredentialReference, IncludePatterns: req.IncludePatterns,
			ExcludePatterns: req.ExcludePatterns, SparsePaths: req.SparsePaths,
			SubmodulePolicy: req.SubmodulePolicy, SymlinkPolicy: req.SymlinkPolicy,
			WorkingTreeMode: req.WorkingTreeMode, CloneDepth: req.CloneDepth,
			PartialCloneFilter: req.PartialCloneFilter, PersistSource: true,
			LibraryDocsHandling: req.LibraryDocsHandling,
			MaxFiles: h.opt.MaxIngestFiles, MaxDocumentBytes: h.opt.MaxDocumentBytes,
			CacheRoot: gitrepo.CacheRootForDB(h.opt.DBPath),
			Progress: h.gitProgress(opID, src),
		})
		h.finishGitJob(opID, res, err)
	}()

	WriteJSON(w, http.StatusAccepted, map[string]any{"opId": opID})
}

// handleGitRefresh fetches and incrementally reindexes a registered Git
// source as a tracked async job.
func (h *handler) handleGitRefresh(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	name := r.PathValue("name")
	src := librarian.SourceRef{Kind: librarian.KindRepo, ID: name}
	if rs, err := h.opt.Store.GetRepoSourceByName(r.Context(), name); err == nil {
		src.RootName = rs.RootName
		src.Title = firstNonEmpty(rs.RemoteURL, rs.LocalPath)
	}

	opID := h.opt.Tracker.Start(src, "refresh")
	jobCtx, cancel := context.WithCancel(context.Background())
	h.opt.Tracker.SetCancel(opID, cancel)

	go func() {
		defer cancel()
		res, err := gitrepo.RefreshRepoSource(jobCtx, h.opt.Store, name, gitrepo.CacheRootForDB(h.opt.DBPath), nil, h.gitProgress(opID, src))
		h.finishGitJob(opID, res, err)
	}()

	WriteJSON(w, http.StatusAccepted, map[string]any{"opId": opID})
}

func (h *handler) gitProgress(opID string, src librarian.SourceRef) gitrepo.ProgressFunc {
	var mu sync.Mutex
	var last time.Time
	return func(phase string, done, total int, bytes int64, current, message string) {
		mu.Lock()
		// Throttle noisy per-file index updates so SSE stays responsive.
		if phase == "index" && time.Since(last) < 150*time.Millisecond {
			mu.Unlock()
			return
		}
		last = time.Now()
		mu.Unlock()
		h.opt.Tracker.Update(opID, librarian.ProgressEvent{
			Source: src, Phase: phase, Done: done, Total: total, Bytes: bytes,
			Current: current, Message: message, UpdatedAt: time.Now().Unix(),
		})
	}
}

func (h *handler) finishGitJob(opID string, res *gitrepo.IngestReport, err error) {
	state := "ok"
	var errs []string
	report := map[string]any{}
	if err != nil {
		state = "failed"
		errs = append(errs, err.Error())
	}
	if res != nil {
		report["filesDiscovered"] = res.FilesDiscovered
		report["documentsIngested"] = res.DocumentsIngested
		report["filesSkipped"] = res.FilesSkipped
		report["resolvedCommit"] = res.ResolvedCommit
		report["status"] = res.Status
		if res.LibraryDocs != nil {
			report["libraryDocs"] = res.LibraryDocs
		}
		errs = append(errs, res.Warnings...)
	}
	h.opt.Tracker.Finish(opID, state, report, errs)
}

func (h *handler) handleDeleteGitSource(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "delete") {
		return
	}
	name := r.PathValue("name")
	removeIndex := queryBool(r, "removeIndex", true)
	removeClone := queryBool(r, "removeClone", false)
	ok, err := gitrepo.RemoveRepoSource(r.Context(), h.opt.Store, name, removeIndex, removeClone)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"name": name, "deleted": ok})
}

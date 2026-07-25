// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"implcache-mcp/gitrepo"
)

type localPreviewRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
	Mode      string `json:"mode,omitempty"` // markdown|project
	Limit     int    `json:"limit,omitempty"`
}

// handleLocalPreview walks a path without writing to the store (preview-before-ingest).
func (h *handler) handleLocalPreview(w http.ResponseWriter, r *http.Request) {
	var req localPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	path := strings.TrimSpace(req.Path)
	if path == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "path is required")
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	st, err := os.Stat(abs)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 200
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "project"
	}

	type fileHit struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	var files []fileHit
	var skipped int
	var totalBytes int64

	if !st.IsDir() {
		files = append(files, fileHit{Path: abs, Size: st.Size()})
		totalBytes = st.Size()
	} else {
		walkErr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				skipped++
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" || name == ".venv" {
					return filepath.SkipDir
				}
				if !req.Recursive && p != abs {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				skipped++
				return nil
			}
			rel, _ := filepath.Rel(abs, p)
			ext := strings.ToLower(filepath.Ext(p))
			if mode == "markdown" && ext != ".md" && ext != ".markdown" && ext != ".html" && ext != ".htm" {
				skipped++
				return nil
			}
			if len(files) < limit {
				files = append(files, fileHit{Path: filepath.ToSlash(rel), Size: info.Size()})
			}
			totalBytes += info.Size()
			return nil
		})
		if walkErr != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", walkErr.Error())
			return
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"path":           abs,
		"mode":           mode,
		"isDirectory":    st.IsDir(),
		"filesPreviewed": files,
		"fileCount":      len(files),
		"truncated":      len(files) >= limit,
		"skipped":        skipped,
		"totalBytes":     totalBytes,
		"note":           "Preview only; no index writes.",
	})
}

type gitInspectRequest struct {
	RemoteURL string `json:"remoteUrl,omitempty"`
	LocalPath string `json:"localPath,omitempty"`
	Ref       string `json:"ref,omitempty"`
}

func (h *handler) handleGitInspect(w http.ResponseWriter, r *http.Request) {
	var req gitInspectRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rep, err := gitrepo.InspectRepo(r.Context(), gitrepo.InspectOptions{
		RemoteURL: req.RemoteURL,
		LocalPath: req.LocalPath,
		Ref:       req.Ref,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, rep)
}

type webPreviewRequest struct {
	StartURL          string   `json:"startUrl"`
	AllowedPrefixes   []string `json:"allowedPrefixes,omitempty"`
	MaxPages          int      `json:"maxPages,omitempty"`
	AllowInsecureHTTP bool     `json:"allowInsecureHttp,omitempty"`
}

// handleWebPreview returns a dry-run crawl plan (URL + prefix validation) without ingest.
func (h *handler) handleWebPreview(w http.ResponseWriter, r *http.Request) {
	var req webPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	start := strings.TrimSpace(req.StartURL)
	if start == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "startUrl is required")
		return
	}
	prefixes := req.AllowedPrefixes
	if len(prefixes) == 0 {
		prefixes = []string{start}
	}
	maxPages := req.MaxPages
	if maxPages <= 0 {
		maxPages = 10
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"startUrl":          start,
		"allowedPrefixes":   prefixes,
		"maxPages":          maxPages,
		"allowInsecureHttp": req.AllowInsecureHTTP,
		"dryRun":            true,
		"note":              "Preview plan only. Register the source and call ingest to crawl.",
		"warnings":          []string{},
	})
}

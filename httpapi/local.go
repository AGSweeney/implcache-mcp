// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/internal/safePath"
	"implcache-mcp/librarian"
)

type localIngestRequest struct {
	Path      string `json:"path"`
	RootName  string `json:"rootName,omitempty"`
	Mode      string `json:"mode,omitempty"` // markdown|project
	Recursive bool   `json:"recursive,omitempty"`
}

// handleDeleteLocalSource removes a synthesized local root's indexed documents
// (project://{root}/…). Refuses roots owned by web/pdf/repo source rows.
func (h *handler) handleDeleteLocalSource(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "delete") {
		return
	}
	root := strings.TrimSpace(r.PathValue("name"))
	if root == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "root name is required")
		return
	}
	src, err := librarian.GetSource(r.Context(), h.opt.Store, librarian.KindLocal, root)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if src.Kind != librarian.KindLocal {
		WriteError(w, http.StatusBadRequest, "bad_request", "use the typed delete endpoint for "+string(src.Kind)+" sources")
		return
	}
	prefix := "project://" + root + "/"
	n, err := h.opt.Store.DeleteDocumentsByURIPrefix(r.Context(), prefix)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"rootName": root, "deletedDocuments": n, "prefix": prefix})
}

func (h *handler) handleLocalIngest(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	var req localIngestRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "project"
	}

	switch mode {
	case "markdown":
		res, err := ingest.IngestMarkdownOpts(r.Context(), h.opt.Store, ingest.MarkdownOptions{
			Path: req.Path, Recursive: req.Recursive, RootName: req.RootName,
			MaxFiles: h.opt.MaxIngestFiles, MaxDocumentBytes: h.opt.MaxDocumentBytes,
		})
		if err != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		WriteJSON(w, http.StatusOK, res)
	case "project":
		res, err := ingest.IngestProjectOpts(r.Context(), h.opt.Store, ingest.ProjectOptions{
			Path: req.Path, RootName: req.RootName,
			MaxFiles: h.opt.MaxIngestFiles, MaxDocumentBytes: h.opt.MaxDocumentBytes,
		})
		if err != nil {
			WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		WriteJSON(w, http.StatusOK, res)
	default:
		WriteError(w, http.StatusBadRequest, "bad_request", "mode must be markdown or project")
	}
}

const maxUploadBytes = 128 << 20 // 128 MiB

// handleUploads accepts a multipart "file" field and saves it under
// opt.UploadDir with a collision-resistant name, for later PDF ingest.
func (h *handler) handleUploads(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	if strings.TrimSpace(h.opt.UploadDir) == "" {
		WriteError(w, http.StatusInternalServerError, "internal", "upload directory is not configured")
		return
	}
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	defer file.Close()

	absDir, err := filepath.Abs(h.opt.UploadDir)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	safeName := sanitizeUploadName(header.Filename)
	dest, err := safePath.ResolveUnderRoot(absDir, safeName)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	out, err := os.Create(dest)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	defer out.Close()

	n, err := io.Copy(out, file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"path": dest, "fileName": filepath.Base(dest), "size": n,
	})
}

// sanitizeUploadName strips path separators and prefixes a time-based token
// so concurrent uploads with the same original name never collide or overwrite.
func sanitizeUploadName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		name = "upload"
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), name)
}

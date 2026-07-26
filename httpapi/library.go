// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"
	"strconv"

	"implcache-mcp/librarian"
)

const maxDocumentPreviewChunks = 20

func (h *handler) handleLibraryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := librarian.GetLibraryStats(r.Context(), h.opt.Store, h.opt.DBPath, h.opt.Tracker)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, stats)
}

// handlePurgeEmptyDocs deletes documents that have no chunk rows (ingest stubs).
func (h *handler) handlePurgeEmptyDocs(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "delete") {
		return
	}
	report, err := h.opt.Store.DocumentsWithoutChunksReport(r.Context(), 8)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	n, err := h.opt.Store.DeleteDocumentsWithoutChunks(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"deleted":    n,
		"before":     report.Total,
		"byRoot":     nonNilSlice(report.ByRoot),
		"sampleUris": nonNilSlice(report.SampleURIs),
	})
}

func (h *handler) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	root := q.Get("root")
	sourceType := q.Get("sourceType")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	docs, total, err := h.opt.Store.ListDocumentsPage(r.Context(), root, sourceType, limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"documents": nonNilSlice(docs), "total": total, "limit": limit, "offset": offset,
	})
}

func (h *handler) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "id must be numeric")
		return
	}
	doc, chunks, err := h.opt.Store.GetDocumentByID(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	total := len(chunks)
	truncated := false
	if total > maxDocumentPreviewChunks {
		chunks = chunks[:maxDocumentPreviewChunks]
		truncated = true
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"document": doc, "chunks": nonNilSlice(chunks), "totalChunks": total, "truncated": truncated,
	})
}

func (h *handler) handleDocumentSymbols(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", "id must be numeric")
		return
	}
	limit := queryInt(r, "limit", 40)
	syms, err := h.opt.Store.ListSymbolsByDocumentIDs(r.Context(), []int64{id}, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"symbols": nonNilSlice(syms), "count": len(syms)})
}

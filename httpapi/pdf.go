// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"
	"strings"

	"implcache-mcp/pdf"
)

type pdfInspectRequest struct {
	Path      string `json:"path"`
	PageStart int    `json:"pageStart,omitempty"`
	PageEnd   int    `json:"pageEnd,omitempty"`
	MaxPages  int    `json:"maxPages,omitempty"`
}

func (h *handler) handlePDFInspect(w http.ResponseWriter, r *http.Request) {
	var req pdfInspectRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	rep, err := pdf.InspectPDF(req.Path, pdf.InspectOptions{
		MaxFileBytes: h.opt.MaxDocumentBytes,
		MaxPages:     req.MaxPages,
		PageStart:    req.PageStart,
		PageEnd:      req.PageEnd,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, rep)
}

type pdfIngestRequest struct {
	Path      string `json:"path"`
	RootName  string `json:"rootName,omitempty"`
	Authority string `json:"authority,omitempty"`
	Product   string `json:"product,omitempty"`
	Version   string `json:"version,omitempty"`
	Language  string `json:"language,omitempty"`
	OCRMode   string `json:"ocrMode,omitempty"`
	PageStart int    `json:"pageStart,omitempty"`
	PageEnd   int    `json:"pageEnd,omitempty"`
	MaxPages  int    `json:"maxPages,omitempty"`
	Force     bool   `json:"force,omitempty"`
}

// handlePDFIngest runs synchronously; Stage 1 PDF ingest is bounded enough
// that a blocking request/response is acceptable (per product requirements).
func (h *handler) handlePDFIngest(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	var req pdfIngestRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	res, err := pdf.IngestPDF(r.Context(), h.opt.Store, pdf.IngestOptions{
		Path: req.Path, RootName: req.RootName, Authority: req.Authority,
		Product: req.Product, Version: req.Version, Language: req.Language,
		OCRMode: req.OCRMode, PageStart: req.PageStart, PageEnd: req.PageEnd,
		MaxFileBytes: h.opt.MaxDocumentBytes, MaxPages: req.MaxPages, Force: req.Force,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, res)
}

func (h *handler) handleDeletePDFSource(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "delete") {
		return
	}
	uri := strings.TrimSpace(r.URL.Query().Get("uri"))
	if uri == "" {
		WriteError(w, http.StatusBadRequest, "bad_request", "uri query parameter is required")
		return
	}
	ok, err := pdf.RemovePDF(r.Context(), h.opt.Store, uri)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"uri": uri, "deleted": ok})
}

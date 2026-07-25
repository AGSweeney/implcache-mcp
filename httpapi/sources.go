// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"

	"implcache-mcp/librarian"
)

func (h *handler) handleListSources(w http.ResponseWriter, r *http.Request) {
	list, err := librarian.ListSources(r.Context(), h.opt.Store)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	list = nonNilSlice(list)
	WriteJSON(w, http.StatusOK, map[string]any{"sources": list, "count": len(list)})
}

func (h *handler) handleGetSource(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	id := r.PathValue("id")
	sum, err := librarian.GetSource(r.Context(), h.opt.Store, librarian.SourceKind(kind), id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, APIError{Code: "not_found", Message: err.Error(), SourceID: id})
		return
	}
	WriteJSON(w, http.StatusOK, sum)
}

func (h *handler) handleSourceHealth(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	id := r.PathValue("id")
	health, err := librarian.GetSourceHealth(r.Context(), h.opt.Store, librarian.SourceKind(kind), id)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, APIError{Code: "not_found", Message: err.Error(), SourceID: id})
		return
	}
	WriteJSON(w, http.StatusOK, health)
}

func (h *handler) handleSourceErrors(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	id := r.PathValue("id")
	limit := queryInt(r, "limit", 20)
	errs, err := librarian.RecentErrors(r.Context(), h.opt.Store, librarian.SourceKind(kind), id, limit)
	if err != nil {
		WriteAPIError(w, http.StatusNotFound, APIError{Code: "not_found", Message: err.Error(), SourceID: id})
		return
	}
	errs = nonNilSlice(errs)
	WriteJSON(w, http.StatusOK, map[string]any{"kind": kind, "id": id, "errors": errs, "count": len(errs)})
}

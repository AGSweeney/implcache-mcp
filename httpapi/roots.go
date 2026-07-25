// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"

	"implcache-mcp/store"
)

func (h *handler) handleListRoots(w http.ResponseWriter, r *http.Request) {
	roots, err := h.opt.Store.ListRootNames(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	roots = nonNilSlice(roots)
	WriteJSON(w, http.StatusOK, map[string]any{"roots": roots, "count": len(roots)})
}

func (h *handler) handleListRootGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.opt.Store.ListRootGroups(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	groups = nonNilSlice(groups)
	WriteJSON(w, http.StatusOK, map[string]any{"rootGroups": groups, "groups": groups, "count": len(groups)})
}

type upsertRootGroupRequest struct {
	Description string                  `json:"description"`
	Members     []store.RootGroupMember `json:"members"`
}

func (h *handler) handleUpsertRootGroup(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "ingest") {
		return
	}
	name := r.PathValue("name")
	var req upsertRootGroupRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := h.opt.Store.UpsertRootGroup(r.Context(), name, req.Description, req.Members); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"name": name, "ok": true})
}

func (h *handler) handleDeleteRootGroup(w http.ResponseWriter, r *http.Request) {
	if !h.allowMutation(w, r, "delete") {
		return
	}
	name := r.PathValue("name")
	ok, err := h.opt.Store.DeleteRootGroup(r.Context(), name)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"name": name, "deleted": ok})
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"

	"implcache-mcp/implctx"
	"implcache-mcp/librarian"
)

type searchPlaygroundRequest struct {
	Query    string   `json:"query"`
	Roots    []string `json:"roots,omitempty"`
	RootName string   `json:"rootName,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Semantic bool     `json:"semantic,omitempty"`
	Explain  bool     `json:"explain,omitempty"`
}

func (h *handler) handleSearchPlayground(w http.ResponseWriter, r *http.Request) {
	var req searchPlaygroundRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	res, err := librarian.SearchPlayground(r.Context(), h.opt.Store, librarian.SearchPlaygroundOptions{
		Query:    req.Query,
		Roots:    req.Roots,
		RootName: req.RootName,
		Limit:    req.Limit,
		Semantic: req.Semantic || h.opt.EnableSemantic,
		Explain:  req.Explain,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, res)
}

type findSymbolsRequest struct {
	Name  string   `json:"name"`
	Roots []string `json:"roots,omitempty"`
	Limit int      `json:"limit,omitempty"`
}

func (h *handler) handleSearchSymbols(w http.ResponseWriter, r *http.Request) {
	var req findSymbolsRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	syms, err := h.opt.Store.FindSymbols(r.Context(), req.Name, req.Roots, req.Limit)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"symbols": nonNilSlice(syms), "count": len(syms)})
}

func (h *handler) handleLibraryHealth(w http.ResponseWriter, r *http.Request) {
	issues, err := librarian.LibraryHealth(r.Context(), h.opt.Store)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	issues = nonNilSlice(issues)
	WriteJSON(w, http.StatusOK, map[string]any{"issues": issues, "count": len(issues)})
}

type searchContextRequest struct {
	Task             string   `json:"task"`
	Language         string   `json:"language,omitempty"`
	Technology       string   `json:"technology,omitempty"`
	ProjectRoot      string   `json:"projectRoot,omitempty"`
	PreferredRoots   []string `json:"preferredRoots,omitempty"`
	RootGroup        string   `json:"rootGroup,omitempty"`
	MaxContextTokens int      `json:"maxContextTokens,omitempty"`
	Semantic         bool     `json:"semantic,omitempty"`
}

func (h *handler) handleSearchContext(w http.ResponseWriter, r *http.Request) {
	var req searchContextRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	res, err := implctx.Get(r.Context(), h.opt.Store, implctx.Request{
		Task:             req.Task,
		Language:         req.Language,
		Technology:       req.Technology,
		ProjectRoot:      req.ProjectRoot,
		PreferredRoots:   req.PreferredRoots,
		RootGroup:        req.RootGroup,
		MaxContextTokens: req.MaxContextTokens,
		Semantic:         req.Semantic || h.opt.EnableSemantic,
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, res)
}

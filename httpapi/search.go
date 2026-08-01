// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"implcache-mcp/implctx"
	"implcache-mcp/librarian"
	"implcache-mcp/store"
	"implcache-mcp/usage"
)

type searchPlaygroundRequest struct {
	Query              string   `json:"query"`
	Roots              []string `json:"roots,omitempty"`
	RootName           string   `json:"rootName,omitempty"`
	Limit              int      `json:"limit,omitempty"`
	Semantic           bool     `json:"semantic,omitempty"`
	Explain            bool     `json:"explain,omitempty"`
	AllRoots           bool     `json:"allRoots,omitempty"`
	LibraryDocsOnly    bool     `json:"libraryDocsOnly,omitempty"`
	ExcludeLibraryDocs bool     `json:"excludeLibraryDocs,omitempty"`
	LibraryDocsLevel   string   `json:"libraryDocsLevel,omitempty"`
	LibraryDocsStatus  string   `json:"libraryDocsStatus,omitempty"`
}

func (h *handler) handleSearchPlayground(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req searchPlaygroundRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	res, err := librarian.SearchPlayground(r.Context(), h.opt.Store, librarian.SearchPlaygroundOptions{
		Query:              req.Query,
		Roots:              req.Roots,
		RootName:           req.RootName,
		Limit:              req.Limit,
		Semantic:           req.Semantic || h.opt.EnableSemantic,
		Explain:            req.Explain,
		AllRoots:           req.AllRoots,
		LibraryDocsOnly:    req.LibraryDocsOnly,
		ExcludeLibraryDocs: req.ExcludeLibraryDocs,
		LibraryDocsLevel:   req.LibraryDocsLevel,
		LibraryDocsStatus:  req.LibraryDocsStatus,
	})
	if err != nil {
		var need *store.ErrNeedsRoot
		if errors.As(err, &need) {
			h.recordUsage(r, usage.RootSelectionEvent("search", req.Query, need.Inference.AvailableRoots, time.Since(start)))
			WriteJSON(w, http.StatusConflict, need.Inference)
			return
		}
		h.recordUsage(r, usage.ErrorEvent("search", req.Query, "request_error", err.Error(), time.Since(start)))
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ev := usage.RequestEvent{
		RequestID:     usage.NewRequestID(),
		OccurredAt:    time.Now().UTC(),
		ToolName:      "search",
		TaskHash:      usage.HashTask(req.Query),
		LatencyMS:     int(time.Since(start).Milliseconds()),
		CitationCount: len(res.Hits),
		SourceCount:   len(res.Hits),
		ResultStatus:  usage.StatusGroundedLocal,
	}
	if len(res.Hits) == 0 {
		ev.ResultStatus = usage.StatusNoLocalMatch
	}
	for _, root := range res.Roots {
		ev.Roots = append(ev.Roots, usage.RootRef{RootKey: root, RootName: root, Selected: true})
	}
	ev.RootCount = len(ev.Roots)
	for i, hit := range res.Hits {
		if i >= 32 {
			break
		}
		ev.Evidence = append(ev.Evidence, usage.EvidenceEvent{
			EvidenceType:       usage.EvidenceCitation,
			EvidenceKey:        hit.URI,
			RootKey:            hit.RootName,
			SourceURI:          hit.URI,
			Authority:          hit.Authority,
			RankPosition:       i + 1,
			SelectedForPackage: true,
		})
	}
	h.recordUsage(r, ev)
	WriteJSON(w, http.StatusOK, res)
}

type findSymbolsRequest struct {
	Name  string   `json:"name"`
	Roots []string `json:"roots,omitempty"`
	Limit int      `json:"limit,omitempty"`
}

func (h *handler) handleSearchSymbols(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req findSymbolsRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	syms, err := h.opt.Store.FindSymbols(r.Context(), req.Name, req.Roots, req.Limit)
	if err != nil {
		h.recordUsage(r, usage.ErrorEvent("search_symbols", req.Name, "request_error", err.Error(), time.Since(start)))
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	ev := usage.RequestEvent{
		RequestID:    usage.NewRequestID(),
		OccurredAt:   time.Now().UTC(),
		ToolName:     "search_symbols",
		TaskHash:     usage.HashTask(req.Name),
		LatencyMS:    int(time.Since(start).Milliseconds()),
		SymbolCount:  len(syms),
		ResultStatus: usage.StatusGroundedLocal,
	}
	if len(syms) == 0 {
		ev.ResultStatus = usage.StatusNoLocalMatch
	}
	for i, sym := range syms {
		if i >= 32 {
			break
		}
		ev.Evidence = append(ev.Evidence, usage.EvidenceEvent{
			EvidenceType:       usage.EvidenceSymbol,
			EvidenceKey:        usage.SymbolKey(sym.NameNorm, sym.RootName),
			RootKey:            sym.RootName,
			RankPosition:       i + 1,
			SelectedForPackage: true,
		})
	}
	for _, root := range req.Roots {
		ev.Roots = append(ev.Roots, usage.RootRef{RootKey: root, RootName: root, Selected: true})
	}
	ev.RootCount = len(ev.Roots)
	h.recordUsage(r, ev)
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
	Version          string   `json:"version,omitempty"`
	ProjectRoot      string   `json:"projectRoot,omitempty"`
	PreferredRoots   []string `json:"preferredRoots,omitempty"`
	KnowledgeGroup   string   `json:"knowledgeGroup,omitempty"`
	RootGroup        string   `json:"rootGroup,omitempty"` // deprecated alias
	MaxContextTokens int      `json:"maxContextTokens,omitempty"`
	Semantic         bool     `json:"semantic,omitempty"`
	Debug            bool     `json:"debug,omitempty"`
}

func (h *handler) handleSearchContext(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req searchContextRequest
	if err := decodeJSON(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	kg := strings.TrimSpace(req.KnowledgeGroup)
	if kg == "" {
		kg = strings.TrimSpace(req.RootGroup)
	}
	res, err := implctx.Get(r.Context(), h.opt.Store, implctx.Request{
		Task:             req.Task,
		Language:         req.Language,
		Technology:       req.Technology,
		Version:          req.Version,
		ProjectRoot:      req.ProjectRoot,
		PreferredRoots:   req.PreferredRoots,
		KnowledgeGroup:   kg,
		RootGroup:        req.RootGroup,
		MaxContextTokens: req.MaxContextTokens,
		Semantic:         req.Semantic || h.opt.EnableSemantic,
		Debug:            req.Debug,
	})
	if err != nil {
		var need *store.ErrNeedsRoot
		if errors.As(err, &need) {
			h.recordUsage(r, usage.RootSelectionEvent("search_context", req.Task, need.Inference.AvailableRoots, time.Since(start)))
			WriteJSON(w, http.StatusConflict, need.Inference)
			return
		}
		h.recordUsage(r, usage.ErrorEvent("search_context", req.Task, "request_error", err.Error(), time.Since(start)))
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	h.recordUsage(r, usage.FromImplementationContext("search_context", req.Task, res, time.Since(start)))
	WriteJSON(w, http.StatusOK, res)
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"implcache-mcp/store"
)

const apiPrefix = "/api/v1/"

type ctxKey int

const roleCtxKey ctxKey = 1

// handler holds shared server state for the Librarian REST API v1 endpoints.
type handler struct {
	opt Options
}

// NewHandler builds the ImplCache Librarian REST API v1 handler. All API
// routes are served under /api/v1/*; when opt.StaticFS is set and
// opt.LibrarianEnabled is true, all other paths serve the bundled SPA
// (stripping opt.LibrarianBasePath, falling back to index.html). The MCP
// transport is not mounted here; the caller mounts /mcp separately.
func NewHandler(opt Options) http.Handler {
	opt = opt.normalize()
	h := &handler{opt: opt}

	mux := http.NewServeMux()
	h.registerRoutes(mux)
	api := h.withSecurityHeaders(h.withAuth(mux))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			api.ServeHTTP(w, r)
			return
		}
		if opt.StaticFS != nil && opt.LibrarianEnabled {
			h.withSecurityHeaders(http.HandlerFunc(h.serveStatic)).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "" {
			WriteError(w, http.StatusNotFound, "not_found",
				"Librarian UI is disabled; restart with -enable-librarian (and usually -enable-http-mutations -mode admin)")
			return
		}
		http.NotFound(w, r)
	})
}

func (h *handler) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com data:; img-src 'self' data:; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// withAuth enforces Bearer auth when APIToken and/or ViewerAPIToken are set.
// Admin token → administrator; viewer token → viewer (mutations denied).
// When neither token is configured, requests are open (loopback-oriented default).
func (h *handler) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin := strings.TrimSpace(h.opt.APIToken)
		viewer := strings.TrimSpace(h.opt.ViewerAPIToken)
		if admin == "" && viewer == "" {
			r = r.WithContext(context.WithValue(r.Context(), roleCtxKey, "administrator"))
			next.ServeHTTP(w, r)
			return
		}
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		// SSE cannot set Authorization; allow ?access_token= as a fallback.
		supplied := ""
		if strings.HasPrefix(auth, prefix) {
			supplied = strings.TrimPrefix(auth, prefix)
		} else if t := strings.TrimSpace(r.URL.Query().Get("access_token")); t != "" {
			supplied = t
		}
		if supplied == "" {
			WriteError(w, http.StatusUnauthorized, "authentication", "missing bearer token")
			return
		}
		role := ""
		if admin != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(admin)) == 1 {
			role = "administrator"
		} else if viewer != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(viewer)) == 1 {
			role = "viewer"
		}
		if role == "" {
			WriteError(w, http.StatusUnauthorized, "authentication", "invalid bearer token")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), roleCtxKey, role))
		next.ServeHTTP(w, r)
	})
}

func requestRole(r *http.Request) string {
	if v, ok := r.Context().Value(roleCtxKey).(string); ok && v != "" {
		return v
	}
	return "administrator"
}

// allowMutation enforces read-only, role, and ingest/delete permission flags for
// mutating endpoints. kind is "ingest" or "delete". On denial it writes a 403
// authorization error and returns false.
func (h *handler) allowMutation(w http.ResponseWriter, r *http.Request, kind string) bool {
	if requestRole(r) == "viewer" {
		WriteError(w, http.StatusForbidden, "authorization", "viewer role cannot mutate")
		return false
	}
	if h.opt.ReadOnly {
		WriteError(w, http.StatusForbidden, "authorization", "server is read-only")
		return false
	}
	switch kind {
	case "ingest":
		if !h.opt.AllowIngest {
			WriteError(w, http.StatusForbidden, "authorization", "ingest is disabled")
			return false
		}
	case "delete":
		if !h.opt.AllowDelete {
			WriteError(w, http.StatusForbidden, "authorization", "delete is disabled")
			return false
		}
	}
	return true
}

func (h *handler) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+apiPrefix+"server", h.handleServer)

	mux.HandleFunc("GET "+apiPrefix+"sources", h.handleListSources)
	mux.HandleFunc("GET "+apiPrefix+"sources/{kind}/{id}", h.handleGetSource)
	mux.HandleFunc("GET "+apiPrefix+"sources/{kind}/{id}/health", h.handleSourceHealth)
	mux.HandleFunc("GET "+apiPrefix+"sources/{kind}/{id}/errors", h.handleSourceErrors)

	mux.HandleFunc("GET "+apiPrefix+"jobs", h.handleListJobs)
	mux.HandleFunc("GET "+apiPrefix+"jobs/{id}", h.handleGetJob)
	mux.HandleFunc("GET "+apiPrefix+"jobs/{id}/events", h.handleJobEvents)
	mux.HandleFunc("POST "+apiPrefix+"jobs/{id}/cancel", h.handleCancelJob)

	mux.HandleFunc("GET "+apiPrefix+"library/stats", h.handleLibraryStats)
	mux.HandleFunc("GET "+apiPrefix+"library/documents", h.handleListDocuments)
	mux.HandleFunc("GET "+apiPrefix+"library/documents/{id}", h.handleGetDocument)
	mux.HandleFunc("GET "+apiPrefix+"library/documents/{id}/symbols", h.handleDocumentSymbols)
	mux.HandleFunc("POST "+apiPrefix+"library/purge-empty-docs", h.handlePurgeEmptyDocs)

	mux.HandleFunc("GET "+apiPrefix+"roots", h.handleListRoots)
	mux.HandleFunc("GET "+apiPrefix+"root-groups", h.handleListRootGroups)
	mux.HandleFunc("PUT "+apiPrefix+"root-groups/{name}", h.handleUpsertRootGroup)
	mux.HandleFunc("DELETE "+apiPrefix+"root-groups/{name}", h.handleDeleteRootGroup)
	// Knowledge Group aliases (same handlers; preferred API name).
	mux.HandleFunc("GET "+apiPrefix+"knowledge-groups", h.handleListRootGroups)
	mux.HandleFunc("PUT "+apiPrefix+"knowledge-groups/{name}", h.handleUpsertRootGroup)
	mux.HandleFunc("DELETE "+apiPrefix+"knowledge-groups/{name}", h.handleDeleteRootGroup)

	mux.HandleFunc("POST "+apiPrefix+"search", h.handleSearchPlayground)
	mux.HandleFunc("POST "+apiPrefix+"search/symbols", h.handleSearchSymbols)
	mux.HandleFunc("POST "+apiPrefix+"search/context", h.handleSearchContext)

	mux.HandleFunc("GET "+apiPrefix+"analytics/status", h.handleAnalyticsStatus)
	mux.HandleFunc("GET "+apiPrefix+"analytics/summary", h.handleAnalyticsSummary)
	mux.HandleFunc("GET "+apiPrefix+"analytics/timeseries", h.handleAnalyticsTimeseries)
	mux.HandleFunc("GET "+apiPrefix+"analytics/coverage", h.handleAnalyticsCoverage)
	mux.HandleFunc("GET "+apiPrefix+"analytics/grounding", h.handleAnalyticsGrounding)
	mux.HandleFunc("GET "+apiPrefix+"analytics/outcomes", h.handleAnalyticsOutcomes)
	mux.HandleFunc("GET "+apiPrefix+"analytics/evidence", h.handleAnalyticsEvidence)
	mux.HandleFunc("GET "+apiPrefix+"analytics/efficiency", h.handleAnalyticsEfficiency)
	mux.HandleFunc("GET "+apiPrefix+"analytics/knowledge", h.handleAnalyticsKnowledge)
	mux.HandleFunc("GET "+apiPrefix+"analytics/requests", h.handleAnalyticsRequests)
	mux.HandleFunc("GET "+apiPrefix+"analytics/requests/{id}", h.handleAnalyticsRequestDetail)
	mux.HandleFunc("POST "+apiPrefix+"analytics/export", h.handleAnalyticsExport)
	mux.HandleFunc("PUT "+apiPrefix+"settings/analytics", h.handleAnalyticsSettings)
	mux.HandleFunc("DELETE "+apiPrefix+"analytics/data", h.handleAnalyticsClear)

	mux.HandleFunc("GET "+apiPrefix+"health", h.handleLibraryHealth)
	mux.HandleFunc("GET "+apiPrefix+"logs", h.handleLogs)

	mux.HandleFunc("POST "+apiPrefix+"sources/web", h.handleUpsertWebSource)
	mux.HandleFunc("POST "+apiPrefix+"sources/web/preview", h.handleWebPreview)
	mux.HandleFunc("POST "+apiPrefix+"sources/web/{name}/ingest", h.handleWebIngest)
	mux.HandleFunc("POST "+apiPrefix+"sources/web/{name}/refresh", h.handleWebRefresh)
	mux.HandleFunc("DELETE "+apiPrefix+"sources/web/{name}", h.handleDeleteWebSource)

	mux.HandleFunc("POST "+apiPrefix+"sources/git", h.handleGitIngest)
	mux.HandleFunc("POST "+apiPrefix+"sources/git/inspect", h.handleGitInspect)
	mux.HandleFunc("POST "+apiPrefix+"sources/git/{name}/refresh", h.handleGitRefresh)
	mux.HandleFunc("DELETE "+apiPrefix+"sources/git/{name}", h.handleDeleteGitSource)

	mux.HandleFunc("POST "+apiPrefix+"sources/pdf/inspect", h.handlePDFInspect)
	mux.HandleFunc("POST "+apiPrefix+"sources/pdf/ingest", h.handlePDFIngest)
	mux.HandleFunc("DELETE "+apiPrefix+"sources/pdf", h.handleDeletePDFSource)

	mux.HandleFunc("POST "+apiPrefix+"sources/local/preview", h.handleLocalPreview)
	mux.HandleFunc("POST "+apiPrefix+"sources/local/ingest", h.handleLocalIngest)
	mux.HandleFunc("DELETE "+apiPrefix+"sources/local/{name}", h.handleDeleteLocalSource)

	mux.HandleFunc("POST "+apiPrefix+"uploads", h.handleUploads)
}

// serverCapabilities describes the running server for GET /api/v1/server.
type serverCapabilities struct {
	ServerVersion         string   `json:"serverVersion"`
	APIVersion            int      `json:"apiVersion"`
	SchemaVersion         int      `json:"schemaVersion"`
	ReadOnly              bool     `json:"readOnly"`
	SemanticEnabled       bool     `json:"semanticEnabled"`
	OCRSupported          bool     `json:"ocrSupported"`
	SupportedSourceTypes  []string `json:"supportedSourceTypes"`
	AuthMode              string   `json:"authMode"`
	LibrarianEnabled      bool     `json:"librarianEnabled"`
	AllowIngest           bool     `json:"allowIngest"`
	AllowDelete           bool     `json:"allowDelete"`
	MaxDocumentBytes      int64    `json:"maxDocumentBytes"`
	MaxIngestFiles        int      `json:"maxIngestFiles"`
	Role                  string   `json:"role"`
	AnalyticsEnabled   bool `json:"analyticsEnabled"`
	AnalyticsAvailable bool `json:"analyticsAvailable"`
}

func (h *handler) handleServer(w http.ResponseWriter, r *http.Request) {
	authMode := "none"
	if strings.TrimSpace(h.opt.APIToken) != "" || strings.TrimSpace(h.opt.ViewerAPIToken) != "" {
		authMode = "bearer"
	}
	role := requestRole(r)
	if authMode == "none" && (h.opt.ReadOnly || (!h.opt.AllowIngest && !h.opt.AllowDelete)) {
		role = "viewer"
	}
	schema := store.CurrentSchemaVersion()
	if h.opt.Store != nil {
		if v, err := h.opt.Store.SchemaVersion(r.Context()); err == nil {
			schema = v
		}
	}
	analyticsEnabled, analyticsAvailable := false, false
	if h.opt.Usage != nil {
		st := h.opt.Usage.Status(r.Context())
		analyticsEnabled = st.Enabled
		analyticsAvailable = st.Available
	}
	WriteJSON(w, http.StatusOK, serverCapabilities{
		ServerVersion:        firstNonEmpty(h.opt.ServerVersion, "dev"),
		APIVersion:           1,
		SchemaVersion:        schema,
		ReadOnly:             h.opt.ReadOnly,
		SemanticEnabled:      h.opt.EnableSemantic,
		OCRSupported:         false,
		SupportedSourceTypes: []string{"local", "web", "pdf", "repo"},
		AuthMode:             authMode,
		LibrarianEnabled:     h.opt.LibrarianEnabled,
		AllowIngest:          h.opt.AllowIngest && !h.opt.ReadOnly && role != "viewer",
		AllowDelete:          h.opt.AllowDelete && !h.opt.ReadOnly && role != "viewer",
		MaxDocumentBytes:     h.opt.MaxDocumentBytes,
		MaxIngestFiles:       h.opt.MaxIngestFiles,
		Role:                 role,
		AnalyticsEnabled:   analyticsEnabled,
		AnalyticsAvailable: analyticsAvailable,
	})
}

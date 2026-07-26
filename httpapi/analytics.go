// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"implcache-mcp/usage"
)

func (h *handler) requireAdministrator(w http.ResponseWriter, r *http.Request) bool {
	if requestRole(r) == "viewer" {
		WriteError(w, http.StatusForbidden, "authorization", "viewer role cannot mutate")
		return false
	}
	return true
}

func (h *handler) handleAnalyticsStatus(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteJSON(w, http.StatusOK, usage.Status{
			Available:    false,
			LocalOnly:    true,
			MetadataOnly: true,
			Message:      "Analytics unavailable",
		})
		return
	}
	WriteJSON(w, http.StatusOK, h.opt.Usage.Status(r.Context()))
}

type analyticsSettingsBody struct {
	Enabled           *bool `json:"enabled"`
	RetentionDays     *int  `json:"retentionDays"`
	StoreTaskText     *bool `json:"storeTaskText"`
	StoreEvidenceText *bool `json:"storeEvidenceText"`
}

func (h *handler) handleAnalyticsSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdministrator(w, r) {
		return
	}
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics database unavailable")
		return
	}
	var body analyticsSettingsBody
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	cur := h.opt.Usage.Config()
	enabled := cur.Enabled
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	days := cur.RetentionDays
	if body.RetentionDays != nil {
		days = *body.RetentionDays
	}
	storeTask := cur.StoreTaskText
	if body.StoreTaskText != nil {
		storeTask = *body.StoreTaskText
	}
	storeEv := cur.StoreEvidenceText
	if body.StoreEvidenceText != nil {
		storeEv = *body.StoreEvidenceText
	}
	if err := h.opt.Usage.UpdateSettings(r.Context(), enabled, days, storeTask, storeEv); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, h.opt.Usage.Status(r.Context()))
}

type clearAnalyticsBody struct {
	Confirm bool `json:"confirm"`
	Vacuum  bool `json:"vacuum"`
}

func (h *handler) handleAnalyticsClear(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdministrator(w, r) {
		return
	}
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics database unavailable")
		return
	}
	var body clearAnalyticsBody
	if err := decodeJSON(r, &body); err != nil {
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !body.Confirm {
		WriteError(w, http.StatusBadRequest, "bad_request", "confirm must be true")
		return
	}
	if err := h.opt.Usage.ClearAll(r.Context(), body.Vacuum); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, h.opt.Usage.Status(r.Context()))
}

func (h *handler) parseAnalyticsFilter(r *http.Request) usage.Filter {
	q := r.URL.Query()
	f := usage.Filter{
		Root:         strings.TrimSpace(q.Get("root")),
		Tool:         strings.TrimSpace(q.Get("tool")),
		Coverage:     strings.TrimSpace(q.Get("coverage")),
		Status:       strings.TrimSpace(q.Get("status")),
		RequestClass: strings.TrimSpace(q.Get("requestClass")),
		Bucket:       strings.TrimSpace(q.Get("bucket")),
		Sort:         strings.TrimSpace(q.Get("sort")),
		Order:        strings.TrimSpace(q.Get("order")),
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = t
		} else if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.From = t
		}
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = t
		} else if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.To = t
		}
	}
	if f.From.IsZero() {
		if days, err := strconv.Atoi(strings.TrimSpace(q.Get("days"))); err == nil && days > 0 {
			f.From = time.Now().UTC().AddDate(0, 0, -days)
		}
	}
	if lim, err := strconv.Atoi(strings.TrimSpace(q.Get("limit"))); err == nil {
		f.Limit = lim
	}
	if off, err := strconv.Atoi(strings.TrimSpace(q.Get("offset"))); err == nil {
		f.Offset = off
	}
	return f
}

func (h *handler) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	sum, err := h.opt.Usage.QuerySummary(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, sum)
}

func (h *handler) handleAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	pts, err := h.opt.Usage.QueryTimeseries(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if pts == nil {
		pts = []usage.TimePoint{}
	}
	WriteJSON(w, http.StatusOK, map[string]any{"points": pts, "count": len(pts)})
}

func (h *handler) handleAnalyticsCoverage(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	c, err := h.opt.Usage.QueryCoverageBreakdown(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, c)
}

func (h *handler) handleAnalyticsGrounding(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	g, err := h.opt.Usage.QueryGrounding(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, g)
}

func (h *handler) handleAnalyticsOutcomes(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	o, err := h.opt.Usage.QueryOutcomes(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, o)
}

func (h *handler) handleAnalyticsEvidence(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	e, err := h.opt.Usage.QueryEvidence(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, e)
}

func (h *handler) handleAnalyticsEfficiency(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	e, err := h.opt.Usage.QueryEfficiency(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, e)
}

func (h *handler) handleAnalyticsKnowledge(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	k, err := h.opt.Usage.QueryKnowledge(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, k)
}

func (h *handler) handleAnalyticsExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdministrator(w, r) {
		return
	}
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	f := h.parseAnalyticsFilter(r)
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	switch format {
	case "csv":
		b, err := h.opt.Usage.ExportCSV(r.Context(), f)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="implcache-analytics.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	case "json":
		b, err := h.opt.Usage.ExportJSON(r.Context(), f)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="implcache-analytics.json"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	default:
		WriteError(w, http.StatusBadRequest, "bad_request", "format must be json or csv")
	}
}

func (h *handler) handleAnalyticsRequests(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	list, err := h.opt.Usage.QueryRecentRequests(r.Context(), h.parseAnalyticsFilter(r))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if list.Requests == nil {
		list.Requests = []usage.RequestRow{}
	}
	WriteJSON(w, http.StatusOK, list)
}

func (h *handler) handleAnalyticsRequestDetail(w http.ResponseWriter, r *http.Request) {
	if h.opt.Usage == nil {
		WriteError(w, http.StatusServiceUnavailable, "unavailable", "analytics unavailable")
		return
	}
	id := r.PathValue("id")
	d, err := h.opt.Usage.QueryRequestDetail(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		WriteError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, d)
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"implcache-mcp/store"
	"implcache-mcp/usage"
)

func TestAnalyticsEndpointsAndNilUsage(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Nil usage: status still OK; search still works.
	hNil := NewHandler(Options{Store: st, ServerVersion: "test", LibrarianEnabled: false})
	rec := httptest.NewRecorder()
	hNil.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status code %d", rec.Code)
	}

	us, err := usage.Open(filepath.Join(dir, "u.db"), usage.Config{Enabled: true, RetentionDays: 90})
	if err != nil {
		t.Fatal(err)
	}
	defer us.Close()

	h := NewHandler(Options{Store: st, Usage: us, ServerVersion: "test", LibrarianEnabled: false})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}

	us.Record(usage.RequestEvent{
		RequestID: "r1", OccurredAt: time.Now().UTC(), ToolName: "get_implementation_context",
		ResultStatus: usage.StatusGroundedLocal, Coverage: "high",
	})
	deadline := time.Now().Add(3 * time.Second)
	for us.Status(httptest.NewRequest(http.MethodGet, "/", nil).Context()).RequestCount < 1 {
		if time.Now().After(deadline) {
			t.Fatal("flush timeout")
		}
		time.Sleep(40 * time.Millisecond)
	}

	for _, path := range []string{
		"/api/v1/analytics/summary",
		"/api/v1/analytics/timeseries?bucket=day",
		"/api/v1/analytics/coverage",
		"/api/v1/analytics/grounding",
		"/api/v1/analytics/outcomes",
		"/api/v1/analytics/evidence",
		"/api/v1/analytics/efficiency",
		"/api/v1/analytics/knowledge",
		"/api/v1/analytics/requests?limit=10",
		"/api/v1/analytics/requests/r1",
	} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s -> %d %s", path, rec.Code, rec.Body.String())
		}
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/analytics/export?format=json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("export json -> %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/analytics/export?format=csv", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("export csv -> %d %s", rec.Code, rec.Body.String())
	}

	body, _ := json.Marshal(map[string]any{"enabled": false, "retentionDays": 30})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/analytics", bytes.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings %d %s", rec.Code, rec.Body.String())
	}

	body, _ = json.Marshal(map[string]any{"confirm": true})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/analytics/data", bytes.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear %d %s", rec.Code, rec.Body.String())
	}
}

func TestSearchContextWithNilUsage(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewHandler(Options{Store: st, ServerVersion: "test"})
	body, _ := json.Marshal(map[string]any{"task": "does not matter without corpus"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search/context", bytes.NewReader(body))
	h.ServeHTTP(rec, req)
	// May be 200 or 400/409 depending on corpus; must not 500 from analytics.
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("unexpected 500: %s", rec.Body.String())
	}
}

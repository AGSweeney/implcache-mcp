// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return NewHandler(Options{
		Store:            st,
		DBPath:           dbPath,
		ServerVersion:    "test",
		AllowIngest:      true,
		AllowDelete:      true,
		LibrarianEnabled: true,
	})
}

func TestHandleServer(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got serverCapabilities
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ServerVersion != "test" {
		t.Fatalf("serverVersion=%q", got.ServerVersion)
	}
	if got.APIVersion != 1 || got.SchemaVersion != 11 {
		t.Fatalf("api/schema=%d/%d", got.APIVersion, got.SchemaVersion)
	}
	if !got.AllowIngest || !got.AllowDelete {
		t.Fatalf("expected ingest/delete allowed: %+v", got)
	}
	if got.AuthMode != "none" {
		t.Fatalf("expected authMode=none, got %q", got.AuthMode)
	}
}

func TestHandleListSources(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Sources []any `json:"sources"`
		Count   int   `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 0 || len(got.Sources) != 0 {
		t.Fatalf("expected empty inventory on a fresh store, got %+v", got)
	}
}

func TestHandleServerRequiresBearerToken(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := NewHandler(Options{Store: st, DBPath: dbPath, APIToken: "secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/server", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/server", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/server", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct token, got %d body=%s", rec.Code, rec.Body.String())
	}
	var got serverCapabilities
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Role != "administrator" {
		t.Fatalf("role=%q", got.Role)
	}
}

func TestViewerTokenCannotMutate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := NewHandler(Options{
		Store: st, DBPath: dbPath,
		APIToken: "admin", ViewerAPIToken: "viewer",
		AllowIngest: true, AllowDelete: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/server", nil)
	req.Header.Set("Authorization", "Bearer viewer")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var cap serverCapabilities
	_ = json.Unmarshal(rec.Body.Bytes(), &cap)
	if cap.Role != "viewer" || cap.AllowIngest {
		t.Fatalf("expected viewer without ingest: %+v", cap)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/sources/local/ingest", strings.NewReader(`{"path":"."}`))
	req.Header.Set("Authorization", "Bearer viewer")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer mutate, got %d body=%s", rec.Code, rec.Body.String())
	}
}

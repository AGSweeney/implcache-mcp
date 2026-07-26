// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestHandleDeleteLocalSource(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	_, err = st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://NetBurner Examples/EFFS/main.cpp", Title: "main.cpp",
		SourceType: store.SourceSource, Path: "EFFS/main.cpp", RootName: "NetBurner Examples",
		Hash: "h1", Chunks: []store.Chunk{{Body: "int main() {}", StartLine: 1, EndLine: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(Options{Store: st, DBPath: dbPath, AllowIngest: true, AllowDelete: true, LibrarianEnabled: true})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/local/"+url.PathEscape("NetBurner Examples"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		DeletedDocuments int64 `json:"deletedDocuments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.DeletedDocuments < 1 {
		t.Fatalf("expected deleted documents, got %+v", got)
	}
}

func TestHandlePurgeEmptyDocs(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if _, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://demo/empty.md", Title: "empty", SourceType: store.SourceMarkdown,
		RootName: "demo", Hash: "e1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI: "project://demo/ok.md", Title: "ok", SourceType: store.SourceMarkdown,
		RootName: "demo", Hash: "e2", Chunks: []store.Chunk{{Body: "hello"}},
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(Options{Store: st, DBPath: dbPath, AllowIngest: true, AllowDelete: true, LibrarianEnabled: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/purge-empty-docs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Deleted int64 `json:"deleted"`
		Before  int   `json:"before"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Deleted != 1 || got.Before != 1 {
		t.Fatalf("got %+v", got)
	}
	doc, _, err := st.GetDocumentByURI(ctx, "project://demo/ok.md")
	if err != nil || doc == nil {
		t.Fatalf("kept doc missing: %v", err)
	}
}

func TestHandleDeletePDFSourceKeepsFullURI(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := NewHandler(Options{Store: st, DBPath: dbPath, AllowIngest: true, AllowDelete: true, LibrarianEnabled: true})
	uri := "pdf://NetBurner/guide.pdf"
	// Missing source should still accept the full URI (not truncate at ':').
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/pdf?uri="+url.QueryEscape(uri), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), uri) {
		t.Fatalf("response lost URI: %s", rec.Body.String())
	}
}

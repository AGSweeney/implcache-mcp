// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package pdf

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("..", "testdata", "pdf", name)
	return p
}

func TestInspectTextManual(t *testing.T) {
	rep, err := InspectPDF(fixture(t, "text_manual.pdf"), InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.PageCount != 2 {
		t.Fatalf("pages=%d", rep.PageCount)
	}
	if rep.Classification != "text" {
		t.Fatalf("class=%s warnings=%v", rep.Classification, rep.Warnings)
	}
	if rep.TextPages < 1 {
		t.Fatalf("textPages=%d", rep.TextPages)
	}
}

func TestInspectImageOnly(t *testing.T) {
	rep, err := InspectPDF(fixture(t, "image_only.pdf"), InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Classification != "image-only" {
		t.Fatalf("class=%s", rep.Classification)
	}
	found := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "OCR required") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected OCR warning, got %v", rep.Warnings)
	}
}

func TestInspectBookmarks(t *testing.T) {
	rep, err := InspectPDF(fixture(t, "bookmarked.pdf"), InspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Bookmarks < 2 {
		t.Fatalf("bookmarks=%d titles=%v", rep.Bookmarks, rep.BookmarkTitles)
	}
}

func TestHeaderFooterSuppression(t *testing.T) {
	pages, err := ExtractPages(fixture(t, "text_manual.pdf"), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages=%d", len(pages))
	}
	for _, p := range pages {
		if strings.Contains(p.Text, "COMMON HEADER") {
			t.Fatalf("header not suppressed: %q", p.Text)
		}
		if !strings.Contains(p.Text, "RetryPolicy") && !strings.Contains(p.Text, "RegisterHandler") {
			t.Fatalf("body missing: %q", p.Text)
		}
	}
}

func TestIngestPDFPageCitations(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	res, err := IngestPDF(ctx, st, IngestOptions{
		Path:     fixture(t, "bookmarked.pdf"),
		RootName: "device-pdf",
		Product:  "Example Device SDK",
		Version:  "1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped || res.Chunks < 1 {
		t.Fatalf("result=%+v", res)
	}
	if !strings.HasPrefix(res.DocumentURI, "pdf://device-pdf/") {
		t.Fatalf("uri=%s", res.DocumentURI)
	}

	doc, chunks, err := st.GetDocumentByURI(ctx, res.DocumentURI)
	if err != nil {
		t.Fatal(err)
	}
	if doc.SourceType != store.SourcePDF {
		t.Fatalf("sourceType=%s", doc.SourceType)
	}
	cited := false
	for _, c := range chunks {
		if c.StartPage > 0 && c.EndPage >= c.StartPage {
			cited = true
			break
		}
	}
	if !cited {
		t.Fatalf("no page citations on chunks: %+v", chunks)
	}

	hits, err := st.Search(ctx, "RegisterHandler", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected search hit")
	}
	if hits[0].StartPage == 0 {
		t.Fatalf("search hit missing page citation: %+v", hits[0])
	}

	src, err := st.GetPDFSourceByURI(ctx, res.DocumentURI)
	if err != nil {
		t.Fatal(err)
	}
	if src.PageCount != 2 || src.FileHash == "" {
		t.Fatalf("pdf source=%+v", src)
	}

	// Reingest unchanged → skip
	res2, err := IngestPDF(ctx, st, IngestOptions{Path: fixture(t, "bookmarked.pdf"), RootName: "device-pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Skipped {
		t.Fatal("expected unchanged skip")
	}

	ok, err := RemovePDF(ctx, st, res.DocumentURI)
	if err != nil || !ok {
		t.Fatalf("remove ok=%v err=%v", ok, err)
	}
	if _, err := st.GetPDFSourceByURI(ctx, res.DocumentURI); err == nil {
		t.Fatal("expected pdf source removed")
	}
}

func TestIngestImageOnlyFails(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, err = IngestPDF(context.Background(), st, IngestOptions{
		Path:     fixture(t, "image_only.pdf"),
		RootName: "img",
	})
	if err == nil || !strings.Contains(err.Error(), "OCR") {
		t.Fatalf("want OCR error, got %v", err)
	}
}

func TestFailedIngestLeavesPrior(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	res, err := IngestPDF(ctx, st, IngestOptions{Path: fixture(t, "text_manual.pdf"), RootName: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = IngestPDF(ctx, st, IngestOptions{Path: fixture(t, "image_only.pdf"), RootName: "keep"})
	if err == nil {
		t.Fatal("expected failure")
	}
	doc, chunks, err := st.GetDocumentByURI(ctx, res.DocumentURI)
	if err != nil || doc == nil || len(chunks) == 0 {
		t.Fatalf("prior doc missing: doc=%v chunks=%d err=%v", doc, len(chunks), err)
	}
}

func TestDocumentURI(t *testing.T) {
	u := DocumentURI("Root", `..\evil\manual.pdf`)
	if u != "pdf://Root/manual.pdf" {
		t.Fatalf("uri=%s", u)
	}
}

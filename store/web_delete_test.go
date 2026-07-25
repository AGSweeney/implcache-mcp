// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDeleteWebSourceOnlyRemovesLinkedDocuments(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "webdel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()

	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://NetBurner/examples/main.cpp", Title: "main.cpp",
		SourceType: SourceSource, RootName: "NetBurner", Hash: "local1",
		Chunks: []Chunk{{Body: "int main() { return 0; }"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://NetBurner/NBQuickStartGuides/index.html", Title: "QuickStart",
		SourceType: SourceWeb, RootName: "NetBurner", Hash: "web1",
		Chunks: []Chunk{{Body: "NetBurner quick start guide"}},
	}); err != nil {
		t.Fatal(err)
	}
	webDoc, _, err := st.GetDocumentByURI(ctx, "project://NetBurner/NBQuickStartGuides/index.html")
	if err != nil {
		t.Fatal(err)
	}

	srcID, err := st.UpsertWebSource(ctx, WebSource{
		Name: "NetBurner QuickStart", RootName: "NetBurner",
		StartURL: "https://www.netburner.com/NBQuickStartGuides/",
		Profile:  "generic", AllowedPrefixes: []string{"https://www.netburner.com/NBQuickStartGuides/"},
		Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertWebPage(ctx, WebPage{
		WebSourceID: srcID, DocumentID: webDoc.ID,
		SourceURL:    "https://www.netburner.com/NBQuickStartGuides/",
		RelativePath: "NBQuickStartGuides/index.html", PageTitle: "QuickStart",
		ContentHash: "h", CrawlGeneration: 1, LastSeenGeneration: 1,
	}); err != nil {
		t.Fatal(err)
	}

	ok, err := st.DeleteWebSource(ctx, "NetBurner QuickStart")
	if err != nil || !ok {
		t.Fatalf("DeleteWebSource: ok=%v err=%v", ok, err)
	}

	if _, _, err := st.GetDocumentByURI(ctx, "project://NetBurner/examples/main.cpp"); err != nil {
		t.Fatalf("sibling local doc should remain: %v", err)
	}
	if _, _, err := st.GetDocumentByURI(ctx, "project://NetBurner/NBQuickStartGuides/index.html"); err == nil {
		t.Fatal("web doc should be deleted")
	}
}

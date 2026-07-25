// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"implcache-mcp/store"
)

func TestHTMLToMarkdownStripsChrome(t *testing.T) {
	html := `<!doctype html><html><head><title>Core of an example-device-sdk Application</title></head>
<body>
<header id="wwconnect_header"><div class="ww_skin_breadcrumbs"><a href="x.html">User's Guide</a> &gt; Core</div>
<div class="ww_skin_page_toolbar"><a href="#" title="Print"></a></div></header>
<div id="page_content">
  <div class="Heading_3">Core of an example-device-sdk Application</div>
  <div class="Body">An example-device-sdk application must always contain RegisterHandler() and a matching shutdown hook.</div>
  <div class="Body">RegisterHandler() must contain at least one example-device-sdk API call.</div>
</div>
</body></html>`

	md, err := HTMLToMarkdown(html)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "User's Guide") && strings.Contains(md, "](") && !strings.Contains(md, "RegisterHandler") {
		t.Fatalf("still chrome-dominated: %q", md)
	}
	if !strings.Contains(md, "RegisterHandler") {
		t.Fatalf("missing prose: %q", md)
	}
	if strings.Contains(md, "[](") {
		t.Fatalf("empty toolbar links leaked: %q", md)
	}

	doc, err := ContentForDocIngest("page.html", []byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Core of an example-device-sdk Application" {
		t.Fatalf("title=%q", doc.Title)
	}
}

func TestShouldSkipHelpPath(t *testing.T) {
	if !ShouldSkipHelpPath(`C:\help\connect\search.html`) {
		t.Fatal("expected skip connect")
	}
	if !ShouldSkipHelpPath(`/online_help/creo_toolkit.html`) {
		t.Fatal("expected skip TOC shell")
	}
	if ShouldSkipHelpPath(`/online_help/creo_toolkit/user_guide/Core.html`) {
		t.Fatal("should not skip real guide page")
	}
	if !ShouldSkipHelpPath(`C:\Help\Vendor\Common\1033\mft\jquery\jquery.js`) {
		t.Fatal("expected skip mft assets")
	}
	if !ShouldSkipHelpPath(`/Help/Common/1033/index.htm`) {
		t.Fatal("expected skip index shell")
	}
	if ShouldSkipHelpPath(`/Help/Common/1033/101790.htm`) {
		t.Fatal("should not skip topic page")
	}
}

func TestControlAppHTMLToMarkdown(t *testing.T) {
	html := `<!doctype html><html><head><title> Instruction blocks in FBD programs </title></head>
<body>
<div id="page_header"><p class="breadcrumbs"><a href="a.htm">DemoController</a> &gt; FBD</p>
<form id="search_form"><input id="keyword"></form></div>
<div id="content_section">
<table class="relatedtopics aboveheading"><tr><td><a href="toc1.htm"><img src="122.gif" alt="Book Contents"></a></td></tr></table>
<h1>Instruction blocks in FBD programs</h1>
<p>The example-control-app instruction set includes IEC 61131-3 compliant instruction blocks.</p>
</div>
<div id="related_and_nav_column"><div id="TOCBoxContents"></div></div>
<div id="page_footer">Copyright c 2025 Example Automation Demo</div>
</body></html>`

	doc, err := ContentForDocIngest("101790.htm", []byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Title, "Instruction blocks in FBD programs") {
		t.Fatalf("title=%q", doc.Title)
	}
	if !strings.Contains(doc.Markdown, "IEC 61131-3") {
		t.Fatalf("missing prose: %q", doc.Markdown)
	}
	if strings.Contains(doc.Markdown, "Copyright c 2025") {
		t.Fatalf("footer leaked: %q", doc.Markdown)
	}
	if strings.Contains(doc.Markdown, "Book Contents") {
		t.Fatalf("relatedtopics chrome leaked: %q", doc.Markdown)
	}
}

func TestIngestHTMLFile(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "page.html")
	html := `<!doctype html><html><head><title>API Guide</title></head><body>
<div id="page_content"><h1>API Guide</h1><p>Use the search endpoint with FTS5 queries.</p><h2>Auth</h2><p>Pass a token header.</p></div>
</body></html>`
	if err := os.WriteFile(htmlPath, []byte(html), 0o600); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(filepath.Join(dir, "kb.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	res, err := IngestMarkdown(ctx, st, htmlPath, false, "html-root")
	if err != nil {
		t.Fatal(err)
	}
	if res.Ingested != 1 {
		t.Fatalf("ingested=%d errors=%v", res.Ingested, res.Errors)
	}

	hits, err := st.Search(ctx, "FTS5", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected hits from converted HTML")
	}

	doc, chunks, err := st.GetDocumentByURI(ctx, "project://html-root/page.html")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "API Guide" {
		t.Fatalf("title=%q", doc.Title)
	}
	for _, c := range chunks {
		if strings.Contains(c.Body, "<p>") {
			t.Fatalf("raw HTML leaked: %q", c.Body)
		}
	}
}

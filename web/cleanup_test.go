// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"strings"
	"testing"
)

func TestCleanHTMLStripsSphinxNavAndNormalizesTitle(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>UART - ESP32 -  &mdash; ESP-IDF Programming Guide v5.1 documentation</title></head>
<body>
<nav class="wy-nav-side"><div class="wy-menu">Sidebar junk</div></nav>
<div role="main">
<div class="toctree-wrapper">hidden toc</div>
<h1>UART</h1>
<p>Call uart_driver_install before use.</p>
</div>
</body></html>`
	title, md, err := CleanHTML("text/html", []byte(html), ProfileSphinx)
	if err != nil {
		t.Fatal(err)
	}
	if title != "UART - ESP32" {
		t.Fatalf("title=%q", title)
	}
	if strings.Contains(strings.ToLower(md), "sidebar junk") {
		t.Fatalf("nav leaked into markdown: %q", md)
	}
	if !strings.Contains(md, "uart_driver_install") {
		t.Fatalf("missing body: %q", md)
	}
}

func TestSkipCrawlHrefHostname(t *testing.T) {
	if !skipCrawlHref("www.netburner.com") {
		t.Fatal("expected hostname-like href skipped")
	}
	if skipCrawlHref("namespace_d_h_c_p.html") {
		t.Fatal("doc page should not be skipped")
	}
	if !skipCrawlPath("/NBDocs/Developer/html/search_opensearch.php") {
		t.Fatal("php path should be skipped")
	}
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	reSphinxArticle = regexp.MustCompile(`(?is)<div[^>]+role=["']main["'][^>]*>(.*)</div>\s*(?:<footer|</body|$)`)
	reSphinxContent = regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*document[^"']*["'][^>]*>(.*)</div>\s*(?:<div[^>]+class=["'][^"']*sphinxsidebar|$)`)
)

func extractSphinxMain(body []byte) []byte {
	s := string(body)
	if m := reSphinxArticle.FindStringSubmatch(s); len(m) == 2 && len(m[1]) > 40 {
		return []byte(m[1])
	}
	if m := reSphinxContent.FindStringSubmatch(s); len(m) == 2 && len(m[1]) > 40 {
		return []byte(m[1])
	}
	// Prefer <article> or #content if present.
	for _, id := range []string{`id="content"`, `id="main-content"`, `class="body"`} {
		if i := strings.Index(s, id); i >= 0 {
			start := strings.LastIndex(s[:i], "<")
			if start >= 0 {
				return []byte(s[start:])
			}
		}
	}
	return body
}

// SkipSphinxPath reports paths that are low-value Sphinx chrome.
func SkipSphinxPath(rel string) bool {
	p := strings.ToLower(rel)
	for _, bad := range []string{
		"search.html", "genindex.html", "py-modindex.html",
		"_sources/", "searchindex.js", "_static/",
	} {
		if strings.Contains(p, bad) {
			return true
		}
	}
	return false
}

func trimBytes(b []byte) []byte {
	return bytes.TrimSpace(b)
}

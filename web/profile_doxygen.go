// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"regexp"
	"strings"
)

var reDoxygenContents = regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*contents[^"']*["'][^>]*>(.*)</div>\s*(?:<hr|</body|$)`)

func extractDoxygenMain(body []byte) []byte {
	s := string(body)
	if m := reDoxygenContents.FindStringSubmatch(s); len(m) == 2 && len(m[1]) > 40 {
		return []byte(m[1])
	}
	return body
}

// SkipDoxygenPath reports low-value generated Doxygen pages.
func SkipDoxygenPath(rel string) bool {
	p := strings.ToLower(rel)
	base := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i+1:]
	}
	switch {
	case strings.HasPrefix(base, "namespacemembers"),
		strings.HasPrefix(base, "functions_"),
		strings.HasPrefix(base, "globals_"),
		strings.HasPrefix(base, "annotated"),
		base == "classes.html",
		base == "files.html",
		base == "namespaces.html",
		base == "doxygen_crawl.html",
		base == "search.html",
		strings.HasPrefix(base, "search_"),
		strings.HasSuffix(base, ".php"),
		base == "dir_*.html",
		strings.Contains(p, "/search/"),
		base == "jquery.js":
		return true
	}
	return false
}

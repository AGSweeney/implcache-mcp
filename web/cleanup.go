// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"regexp"
	"strings"

	"implcache-mcp/ingest"
)

// Profile names for extraction.
const (
	ProfileGeneric = "generic"
	ProfileSphinx  = "sphinx"
	ProfileDoxygen = "doxygen"
)

var reTitle = regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)

// CleanHTML converts fetched HTML/text into markdown using the named profile.
func CleanHTML(contentType string, body []byte, profile string) (title, markdown string, err error) {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "text/markdown", "text/plain":
		text := string(body)
		return firstHeading(text), text, nil
	}

	htmlStr := string(body)
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case ProfileSphinx:
		htmlStr = string(extractSphinxMain(body))
	case ProfileDoxygen:
		htmlStr = string(extractDoxygenMain(body))
	}
	if m := reTitle.FindStringSubmatch(string(body)); len(m) == 2 {
		title = strings.TrimSpace(m[1])
	}
	md, err := ingest.HTMLToMarkdown(htmlStr)
	if err != nil {
		return "", "", err
	}
	if title == "" {
		title = firstHeading(md)
	}
	return title, md, nil
}

func firstHeading(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}

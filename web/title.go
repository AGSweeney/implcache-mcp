// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

var (
	// Only strip the generator suffix, not earlier "Topic - Chip" segments.
	reSphinxTitleSuffix = regexp.MustCompile(`(?i)\s*[-–—]\s*(?:ESP-IDF|Espressif)\b.*$`)
	reSphinxVersion     = regexp.MustCompile(`(?i)\bv?(\d+\.\d+(?:\.\d+)?)\b`)
	reDoxygenTitle      = regexp.MustCompile(`(?i)^(.+?)\s+(\d+\.\d+(?:\.\d+)?)\s*:\s*(.+)$`)
	reGenericVersion    = regexp.MustCompile(`(?i)\b(?:version|v|release)\s*[:.]?\s*(\d+\.\d+(?:\.\d+)?)\b`)
)

// NormalizeDocTitle unescapes HTML entities and strips generator chrome from titles.
func NormalizeDocTitle(title, profile string) string {
	title = strings.TrimSpace(html.UnescapeString(title))
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case ProfileSphinx:
		title = reSphinxTitleSuffix.ReplaceAllString(title, "")
		title = strings.TrimRight(title, " -–—")
	case ProfileDoxygen:
		if m := reDoxygenTitle.FindStringSubmatch(title); len(m) == 4 {
			title = strings.TrimSpace(m[3])
		}
	}
	return strings.TrimSpace(title)
}

// DetectDocVersion extracts a product/docs version from a page title when possible.
func DetectDocVersion(title, profile string) string {
	title = html.UnescapeString(title)
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case ProfileDoxygen:
		if m := reDoxygenTitle.FindStringSubmatch(title); len(m) == 4 {
			return m[2]
		}
	case ProfileSphinx:
		if m := reSphinxVersion.FindStringSubmatch(title); len(m) == 2 {
			return m[1]
		}
	}
	if m := reGenericVersion.FindStringSubmatch(title); len(m) == 2 {
		return m[1]
	}
	// Bare trailing "v1.2.3" / "3.5.8" near the end of short titles.
	fields := strings.Fields(title)
	for i := len(fields) - 1; i >= 0 && i >= len(fields)-3; i-- {
		f := strings.Trim(fields[i], "():,")
		f = strings.TrimPrefix(strings.ToLower(f), "v")
		if looksLikeSemver(f) {
			return f
		}
	}
	return ""
}

func looksLikeSemver(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

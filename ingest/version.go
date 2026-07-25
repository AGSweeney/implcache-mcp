// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"regexp"
	"strings"
)

var (
	rePathVersion = regexp.MustCompile(`(?i)(?:^|[/\\._-])v?(\d+\.\d+(?:\.\d+)?(?:-[A-Za-z0-9.]+)?)(?:[/\\._-]|$)`)
	rePathVerDir  = regexp.MustCompile(`(?i)[/\\]v?(\d+)[/\\]`)
	reBodyVersion = regexp.MustCompile(`(?im)^\s*(?:version|sdk version|api version|product version)\s*[:\-]\s*v?(\d+\.\d+(?:\.\d+)?|\d+\.x)\b`)
	reBodySDKVer  = regexp.MustCompile(`(?i)\b(?:sdk|api|library)\s+v?(\d+\.\d+(?:\.\d+)?|\d+\.x)\b`)
)

// InferProductVersion extracts a best-effort version from path and document text.
// Returns "" when unknown. Never invents a version without evidence.
func InferProductVersion(rootName, relPath, body string) string {
	path := rootName + "/" + strings.ReplaceAll(relPath, `\`, `/`)
	if v := firstMatch(rePathVersion, path); v != "" {
		return normalizeVer(v)
	}
	if v := firstMatch(rePathVerDir, path); v != "" {
		return normalizeVer(v + ".x")
	}
	head := body
	if len(head) > 4000 {
		head = head[:4000]
	}
	if v := firstMatch(reBodyVersion, head); v != "" {
		return normalizeVer(v)
	}
	if v := firstMatch(reBodySDKVer, head); v != "" {
		return normalizeVer(v)
	}
	return ""
}

func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func normalizeVer(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	return v
}

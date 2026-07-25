// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"path"
	"path/filepath"
	"strings"
)

// FileURI builds a file:/// URI from an absolute filesystem path.
func FileURI(absPath string) string {
	absPath = filepath.Clean(absPath)
	absPath = filepath.ToSlash(absPath)
	// Ensure leading slash for Windows drive paths: D:/x -> /D:/x
	if !strings.HasPrefix(absPath, "/") {
		absPath = "/" + absPath
	}
	return "file://" + absPath
}

// ProjectURI builds project://{rootName}/{rel} with slash-separated relative path.
// Root names are controlled ingest identifiers (not percent-encoded) for stable URIs.
// ".." path segments are dropped.
func ProjectURI(rootName, relPath string) string {
	return schemeURI("project", rootName, relPath)
}

// GitURI builds git://{rootName}/{rel} for repository-ingested files.
func GitURI(rootName, relPath string) string {
	return schemeURI("git", rootName, relPath)
}

func schemeURI(scheme, rootName, relPath string) string {
	rootName = strings.Trim(rootName, "/")
	rel := filepath.ToSlash(relPath)
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.TrimPrefix(rel, "/")
	parts := strings.Split(rel, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			continue
		}
		clean = append(clean, p)
	}
	return scheme + "://" + rootName + "/" + strings.Join(clean, "/")
}

// TitleFromPath returns a display title from a path.
func TitleFromPath(p string) string {
	base := path.Base(filepath.ToSlash(p))
	if base == "." || base == "/" || base == "" {
		return p
	}
	return base
}

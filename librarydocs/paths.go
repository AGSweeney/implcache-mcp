// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

const dirName = "LibraryDocs"

// IsLibraryDocsPath reports whether rel is under LibraryDocs/.
func IsLibraryDocsPath(rel string) bool {
	rel = filepathToSlash(strings.TrimSpace(rel))
	rel = strings.TrimPrefix(rel, "./")
	return rel == dirName || strings.HasPrefix(rel, dirName+"/")
}

// NormalizeRepoRelPath cleans a repo-relative path and rejects traversal.
// Returns the cleaned slash path or an error if the path escapes the repo.
func NormalizeRepoRelPath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	rel = filepathToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	if strings.HasPrefix(rel, "/") || strings.Contains(rel, ":") {
		return "", fmt.Errorf("absolute or URI path not allowed: %s", rel)
	}
	// Reject ".." segments entirely (including LibraryDocs/../outside.md).
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path traversal rejected: %s", rel)
		}
	}
	cleaned := path.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned == "." {
		return "", fmt.Errorf("path traversal rejected: %s", rel)
	}
	if strings.ContainsRune(cleaned, 0) {
		return "", fmt.Errorf("invalid path")
	}
	return cleaned, nil
}

// NormalizeSourcePaths validates and cleans a list of source_paths.
// Invalid entries are omitted; warnings are returned.
func NormalizeSourcePaths(paths []string) (clean []string, warnings []string) {
	seen := map[string]struct{}{}
	for _, p := range paths {
		n, err := NormalizeRepoRelPath(p)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("rejected source_path %q: %v", p, err))
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		clean = append(clean, n)
	}
	return clean, warnings
}

// LibraryDocsRoot returns the absolute LibraryDocs directory under checkout.
func LibraryDocsRoot(checkout string) string {
	return filepath.Join(checkout, dirName)
}

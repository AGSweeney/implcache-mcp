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
	return "project://" + rootName + "/" + strings.Join(clean, "/")
}

// TitleFromPath returns a display title from a path.
func TitleFromPath(p string) string {
	base := path.Base(filepath.ToSlash(p))
	if base == "." || base == "/" || base == "" {
		return p
	}
	return base
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"path"
	"strings"
)

// ClassifyPath returns a LibraryDocs content class for a repo-relative path.
// Non-LibraryDocs paths return empty string.
func ClassifyPath(rel string) string {
	rel = filepathToSlash(strings.TrimSpace(rel))
	rel = strings.TrimPrefix(rel, "./")
	if !IsLibraryDocsPath(rel) {
		return ""
	}
	lower := strings.ToLower(rel)
	base := path.Base(lower)
	switch {
	case lower == "librarydocs/index.md":
		return ClassIndex
	case lower == "librarydocs/project/component_inventory.md":
		return ClassInventory
	case lower == "librarydocs/validation.md":
		return ClassValidation
	case strings.HasPrefix(lower, "librarydocs/libraries/"):
		return ClassCuratedLibraryDoc
	case strings.HasPrefix(lower, "librarydocs/platform/"):
		return ClassCuratedPlatformDoc
	case strings.HasPrefix(lower, "librarydocs/artifacts/"):
		return ClassCuratedArtifact
	case strings.HasPrefix(lower, "librarydocs/project/"):
		return ClassCuratedProjectDoc
	case base == "readme.md" && lower == "librarydocs/readme.md":
		return ClassLibraryDocsOther
	default:
		return ClassLibraryDocsOther
	}
}

// ArtifactTypeFromPath infers artifact class from path under artifacts/.
func ArtifactTypeFromPath(rel string) string {
	rel = strings.ToLower(filepathToSlash(rel))
	switch {
	case strings.Contains(rel, "/interfaces/"):
		return "interface"
	case strings.Contains(rel, "/patterns/"):
		return "pattern"
	case strings.Contains(rel, "/data/"):
		return "data"
	case strings.Contains(rel, "/build/"):
		return "build"
	case strings.Contains(rel, "/bench/"):
		return "bench"
	case strings.Contains(rel, "/python/"):
		return "python"
	default:
		return "other"
	}
}

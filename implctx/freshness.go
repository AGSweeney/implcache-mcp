// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"strings"

	"implcache-mcp/ingest"
)

// freshnessFromSources derives freshness from selected citation authorities/versions.
// States: current | version-specific | mixed | stale | unknown
func freshnessFromSources(citations []Citation, versions []string, archivedHints int) string {
	if len(citations) == 0 && len(versions) == 0 {
		return "unknown"
	}
	if archivedHints > 0 {
		return "stale"
	}
	verSet := map[string]struct{}{}
	addVer := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || strings.EqualFold(v, "unknown") {
			return
		}
		verSet[strings.ToLower(v)] = struct{}{}
	}
	for _, v := range versions {
		addVer(v)
	}
	for _, c := range citations {
		addVer(c.Version)
		if c.Version == "" && strings.HasPrefix(c.URI, "project://") {
			rest := strings.TrimPrefix(c.URI, "project://")
			root, path, _ := strings.Cut(rest, "/")
			addVer(ingest.InferProductVersion(root, path, c.Title+" "+c.Section))
		}
	}
	if len(verSet) > 1 {
		return "mixed"
	}
	if len(verSet) == 1 {
		return "version-specific"
	}
	// Authority mix without versions.
	hasProject, hasDocs, hasGenerated := false, false, false
	for _, c := range citations {
		switch c.Authority {
		case "current_project", "related_internal_project", "curated_internal_recipe":
			hasProject = true
		case "official_documentation", "official_example":
			hasDocs = true
		case "generated_summary", "third_party_reference", "unknown":
			hasGenerated = true
		}
	}
	if hasGenerated && !hasProject && !hasDocs {
		return "unknown"
	}
	if hasProject && hasDocs {
		return "current"
	}
	if hasProject || hasDocs {
		return "current"
	}
	return "unknown"
}

func webSearchFrom(coverage, freshness string) bool {
	if coverage == "low" {
		return true
	}
	switch freshness {
	case "stale", "mixed", "unknown":
		return true
	default:
		return false
	}
}

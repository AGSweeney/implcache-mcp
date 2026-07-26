// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IndexRelativePath is the conventional INDEX location.
const IndexRelativePath = "LibraryDocs/INDEX.md"

// ParseIndexFile reads and parses INDEX.md.
func ParseIndexFile(checkout string) ([]IndexRecord, []string, error) {
	p := filepath.Join(checkout, filepath.FromSlash(IndexRelativePath))
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, nil, err
	}
	recs, warns := ParseIndexMarkdown(string(data))
	return recs, warns, nil
}

// ParseIndexMarkdown parses INDEX tables.
func ParseIndexMarkdown(md string) ([]IndexRecord, []string) {
	var warns []string
	tables := ParseMarkdownTables(md)
	var out []IndexRecord
	seenPath := map[string]struct{}{}
	seenID := map[string]struct{}{}
	for _, t := range tables {
		if !tableHasAny(t.Headers, "path", "id") {
			continue
		}
		for _, row := range t.Rows {
			m := rowMap(t.Headers, row)
			rawPath := m["path"]
			if rawPath == "" {
				continue
			}
			// INDEX paths are often relative to LibraryDocs/
			p := strings.Trim(rawPath, "`")
			p = filepathToSlash(p)
			if !strings.HasPrefix(strings.ToLower(p), "librarydocs/") {
				p = pathJoinLibraryDocs(p)
			}
			n, err := NormalizeRepoRelPath(p)
			if err != nil {
				warns = append(warns, fmt.Sprintf("%s: broken relative path %q: %v", IndexRelativePath, rawPath, err))
				continue
			}
			if _, ok := seenPath[n]; ok {
				warns = append(warns, fmt.Sprintf("%s: duplicate path %s", IndexRelativePath, n))
				continue
			}
			seenPath[n] = struct{}{}
			id := strings.TrimSpace(m["id"])
			if id != "" {
				if _, ok := seenID[id]; ok {
					warns = append(warns, fmt.Sprintf("%s: duplicate component ID %s", IndexRelativePath, id))
				} else {
					seenID[id] = struct{}{}
				}
			}
			out = append(out, IndexRecord{
				Path:      n,
				ID:        id,
				Level:     strings.ToLower(m["level"]),
				Component: m["component"],
				Purpose:   m["purpose"],
				Topics:    splitList(m["topics"]),
				Status:    strings.ToLower(m["status"]),
			})
		}
	}
	return out, warns
}

func pathJoinLibraryDocs(rel string) string {
	rel = strings.TrimPrefix(filepathToSlash(rel), "./")
	if rel == "" {
		return dirName
	}
	return dirName + "/" + rel
}

// CrossCheckIndexInventory records missing ID mappings.
func CrossCheckIndexInventory(index []IndexRecord, inv []ComponentRecord) []string {
	invIDs := map[string]struct{}{}
	for _, c := range inv {
		invIDs[c.ID] = struct{}{}
	}
	idxIDs := map[string]struct{}{}
	for _, r := range index {
		if r.ID != "" {
			idxIDs[r.ID] = struct{}{}
		}
	}
	var warns []string
	for id := range invIDs {
		if _, ok := idxIDs[id]; !ok {
			warns = append(warns, fmt.Sprintf("inventory ID missing from INDEX: %s", id))
		}
	}
	for id := range idxIDs {
		if _, ok := invIDs[id]; !ok {
			warns = append(warns, fmt.Sprintf("INDEX ID missing from inventory: %s", id))
		}
	}
	return warns
}

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

// InventoryRelativePath is the conventional inventory location.
const InventoryRelativePath = "LibraryDocs/project/COMPONENT_INVENTORY.md"

// ParseInventoryFile reads and parses COMPONENT_INVENTORY.md.
func ParseInventoryFile(checkout string) ([]ComponentRecord, []string, error) {
	p := filepath.Join(checkout, filepath.FromSlash(InventoryRelativePath))
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, nil, err
	}
	recs, warns := ParseInventoryMarkdown(string(data))
	return recs, warns, nil
}

// ParseInventoryMarkdown parses inventory tables from markdown.
func ParseInventoryMarkdown(md string) ([]ComponentRecord, []string) {
	var warns []string
	tables := ParseMarkdownTables(md)
	var out []ComponentRecord
	seen := map[string]struct{}{}
	for _, t := range tables {
		if !tableHasAny(t.Headers, "id", "name") {
			continue
		}
		for _, row := range t.Rows {
			m := rowMap(t.Headers, row)
			id := strings.TrimSpace(m["id"])
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				warns = append(warns, fmt.Sprintf("%s: duplicate component ID %s", InventoryRelativePath, id))
				continue
			}
			seen[id] = struct{}{}
			src, srcWarns := NormalizeSourcePaths(splitList(m["source_paths"]))
			warns = append(warns, prefixPathWarns(InventoryRelativePath, srcWarns)...)
			rec := ComponentRecord{
				ID:          id,
				Name:        m["name"],
				Level:       strings.ToLower(m["level"]),
				Folder:      m["folder"],
				SourcePaths: src,
				Reuse:       strings.ToLower(m["reuse"]),
				Owner:       m["owner"],
				IOOwnership: m["io_ownership"],
				ArtifactIDs: splitList(m["artifact_ids"]),
				DocStatus:   strings.ToLower(firstNonEmpty(m["doc_status"], m["status"])),
				Evidence:    normalizeEvidence(m["evidence"]),
			}
			extra := map[string]string{}
			for k, v := range m {
				switch k {
				case "id", "name", "level", "folder", "source_paths", "reuse",
					"owner", "io_ownership", "artifact_ids", "doc_status", "status", "evidence":
				default:
					if v != "" {
						extra[k] = v
					}
				}
			}
			if len(extra) > 0 {
				rec.Extra = extra
			}
			out = append(out, rec)
		}
	}
	return out, warns
}

func tableHasAny(headers []string, keys ...string) bool {
	set := map[string]bool{}
	for _, h := range headers {
		set[h] = true
	}
	for _, k := range keys {
		if set[k] {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func prefixPathWarns(file string, warns []string) []string {
	if len(warns) == 0 {
		return nil
	}
	out := make([]string, len(warns))
	for i, w := range warns {
		out[i] = file + ": " + w
	}
	return out
}

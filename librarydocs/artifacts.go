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

// ArtifactsReadmePath is the conventional artifact registry.
const ArtifactsReadmePath = "LibraryDocs/artifacts/README.md"

// ParseArtifactsFile reads artifacts/README.md.
func ParseArtifactsFile(checkout string) ([]ArtifactRecord, []string, error) {
	p := filepath.Join(checkout, filepath.FromSlash(ArtifactsReadmePath))
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return ParseArtifactsMarkdown(string(data)), nil, nil
}

// ParseArtifactsMarkdown parses the artifact registry table.
func ParseArtifactsMarkdown(md string) []ArtifactRecord {
	tables := ParseMarkdownTables(md)
	var out []ArtifactRecord
	seen := map[string]struct{}{}
	for _, t := range tables {
		if !tableHasAny(t.Headers, "id", "artifact_id", "file") {
			// also accept "artifact id" normalized
			ok := false
			for _, h := range t.Headers {
				if h == "id" || h == "artifact_ids" || h == "file" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		for _, row := range t.Rows {
			m := rowMap(t.Headers, row)
			id := firstNonEmpty(m["id"], firstNonEmpty(m["artifact_ids"], m["artifact_id"]))
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			file := strings.Trim(m["file"], "`")
			file = filepathToSlash(file)
			if file != "" && !strings.HasPrefix(strings.ToLower(file), "librarydocs/") {
				file = pathJoinLibraryDocs(strings.TrimPrefix(file, "artifacts/"))
				if !strings.Contains(strings.ToLower(file), "/artifacts/") {
					file = dirName + "/artifacts/" + strings.TrimPrefix(filepathToSlash(m["file"]), "artifacts/")
				}
			}
			if n, err := NormalizeRepoRelPath(file); err == nil {
				file = n
			}
			out = append(out, ArtifactRecord{
				ID:           id,
				File:         file,
				Component:    m["component"],
				Usefulness:   m["usefulness"],
				Description:  m["description"],
				ArtifactType: ArtifactTypeFromPath(file),
			})
		}
	}
	return out
}

// CheckArtifactRefs warns when artifact IDs on components are unregistered.
func CheckArtifactRefs(components []ComponentRecord, artifacts []ArtifactRecord) []string {
	reg := map[string]struct{}{}
	for _, a := range artifacts {
		reg[a.ID] = struct{}{}
	}
	if len(reg) == 0 {
		return nil
	}
	var warns []string
	for _, c := range components {
		for _, id := range c.ArtifactIDs {
			if _, ok := reg[id]; !ok {
				warns = append(warns, fmt.Sprintf("artifact ID not registered: %s (component %s)", id, c.ID))
			}
		}
	}
	return warns
}

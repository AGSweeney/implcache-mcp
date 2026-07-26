// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"strings"
)

// MarkdownTable is a parsed GitHub-flavored markdown table.
type MarkdownTable struct {
	Headers []string
	Rows    [][]string
}

// ParseMarkdownTables extracts pipe tables from markdown.
func ParseMarkdownTables(md string) []MarkdownTable {
	lines := strings.Split(md, "\n")
	var tables []MarkdownTable
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") {
			i++
			continue
		}
		header := splitPipeRow(line)
		if len(header) == 0 {
			i++
			continue
		}
		if i+1 >= len(lines) || !isSeparatorRow(strings.TrimSpace(lines[i+1])) {
			i++
			continue
		}
		t := MarkdownTable{Headers: normalizeHeaders(header)}
		i += 2
		for i < len(lines) {
			rowLine := strings.TrimSpace(lines[i])
			if !strings.HasPrefix(rowLine, "|") {
				break
			}
			if isSeparatorRow(rowLine) {
				i++
				continue
			}
			cells := splitPipeRow(rowLine)
			// pad/truncate to header width
			row := make([]string, len(t.Headers))
			for j := range t.Headers {
				if j < len(cells) {
					row[j] = cells[j]
				}
			}
			t.Rows = append(t.Rows, row)
			i++
		}
		if len(t.Rows) > 0 {
			tables = append(tables, t)
		}
	}
	return tables
}

func splitPipeRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func isSeparatorRow(line string) bool {
	if !strings.Contains(line, "-") {
		return false
	}
	for _, r := range line {
		if r != '|' && r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return strings.Contains(line, "|")
}

func normalizeHeaders(h []string) []string {
	out := make([]string, len(h))
	for i, s := range h {
		out[i] = normalizeHeaderKey(s)
	}
	return out
}

func normalizeHeaderKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "/", " ")
	s = strings.Join(strings.Fields(s), " ")
	repl := map[string]string{
		"id":                         "id",
		"name":                       "name",
		"level":                      "level",
		"folder":                     "folder",
		"source paths":               "source_paths",
		"source path":                "source_paths",
		"reuse":                      "reuse",
		"owner task thread process":  "owner",
		"owner":                      "owner",
		"socket storage ownership":   "io_ownership",
		"i o ownership":              "io_ownership",
		"artifact ids":               "artifact_ids",
		"artifact id":                "artifact_ids",
		"doc status":                 "doc_status",
		"status":                     "status",
		"evidence":                   "evidence",
		"path":                       "path",
		"component":                  "component",
		"purpose":                    "purpose",
		"topics":                     "topics",
		"file":                       "file",
		"usefulness":                 "usefulness",
		"description":                "description",
	}
	if v, ok := repl[s]; ok {
		return v
	}
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func rowMap(headers []string, row []string) map[string]string {
	m := make(map[string]string, len(headers))
	for i, h := range headers {
		if i < len(row) {
			m[h] = strings.TrimSpace(row[i])
		}
	}
	return m
}

func splitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "—" {
		return nil
	}
	// support comma, semicolon, or backtick-ish lists
	for _, sep := range []string{";", ",", "|"} {
		if strings.Contains(s, sep) {
			var out []string
			for _, p := range strings.Split(s, sep) {
				p = strings.TrimSpace(p)
				p = strings.Trim(p, "`")
				if p != "" && p != "-" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	return []string{strings.Trim(s, "`")}
}

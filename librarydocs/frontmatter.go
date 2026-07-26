// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxFrontmatterBytes = 64 << 10 // 64 KiB

// Frontmatter holds known LibraryDocs YAML fields plus unknowns.
type Frontmatter struct {
	Title      string
	Component  string
	Level      string
	Reuse      string
	Platforms  []string
	Topics     []string
	SourcePaths []string
	Status     string
	Evidence   string
	Questions  []string
	Related    []string
	Unknown    map[string]any
	RawPresent bool
}

// SplitFrontmatter extracts leading YAML frontmatter from markdown.
// On malformed YAML, returns a warning and empty Frontmatter with RawPresent false.
func SplitFrontmatter(markdown string) (fm Frontmatter, body string, warning string) {
	s := strings.TrimLeft(markdown, "\ufeff")
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return Frontmatter{}, markdown, ""
	}
	rest := s[4:]
	if strings.HasPrefix(s, "---\r\n") {
		rest = s[5:]
	}
	end := strings.Index(rest, "\n---\n")
	endCR := strings.Index(rest, "\r\n---\r\n")
	sepLen := 5
	if end < 0 || (endCR >= 0 && endCR < end) {
		if endCR < 0 {
			// also allow closing ---\n at EOF-ish
			end = strings.Index(rest, "\n---")
			if end >= 0 && end+4 == len(rest) {
				sepLen = 4
			} else if end >= 0 {
				// check \n---\r\n
				if end+5 <= len(rest) && rest[end:end+5] == "\n---\r" {
					sepLen = 6
				} else {
					return Frontmatter{}, markdown, "malformed YAML frontmatter: missing closing ---"
				}
			} else {
				return Frontmatter{}, markdown, "malformed YAML frontmatter: missing closing ---"
			}
		} else {
			end = endCR
			sepLen = 8
		}
	} else {
		sepLen = 5
	}
	raw := rest[:end]
	body = rest[end+sepLen:]
	if len(raw) > maxFrontmatterBytes {
		return Frontmatter{}, markdown, fmt.Sprintf("frontmatter exceeds %d bytes", maxFrontmatterBytes)
	}
	fm, err := parseFrontmatterYAML(raw)
	if err != nil {
		return Frontmatter{}, markdown, "malformed YAML frontmatter: " + err.Error()
	}
	fm.RawPresent = true
	return fm, body, ""
}

func parseFrontmatterYAML(raw string) (Frontmatter, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &root); err != nil {
		return Frontmatter{}, err
	}
	var fm Frontmatter
	fm.Unknown = map[string]any{}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fm, nil
	}
	m := root.Content[0]
	if m.Kind != yaml.MappingNode {
		return fm, fmt.Errorf("frontmatter root must be a mapping")
	}
	known := map[string]bool{
		"title": true, "component": true, "level": true, "reuse": true,
		"platforms": true, "topics": true, "source_paths": true, "status": true,
		"evidence": true, "retrieval": true,
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		key := strings.TrimSpace(m.Content[i].Value)
		val := m.Content[i+1]
		switch key {
		case "title":
			fm.Title = scalarString(val)
		case "component":
			fm.Component = scalarString(val)
		case "level":
			fm.Level = strings.ToLower(scalarString(val))
		case "reuse":
			fm.Reuse = strings.ToLower(scalarString(val))
		case "platforms":
			fm.Platforms = sequenceStrings(val)
		case "topics":
			fm.Topics = sequenceStrings(val)
		case "source_paths":
			fm.SourcePaths = sequenceStrings(val)
		case "status":
			fm.Status = strings.ToLower(scalarString(val))
		case "evidence":
			fm.Evidence = normalizeEvidence(scalarString(val))
		case "retrieval":
			if val.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(val.Content); j += 2 {
					rk := strings.TrimSpace(val.Content[j].Value)
					rv := val.Content[j+1]
					switch rk {
					case "questions":
						fm.Questions = sequenceStrings(rv)
					case "related":
						fm.Related = sequenceStrings(rv)
					default:
						var anyVal any
						_ = rv.Decode(&anyVal)
						fm.Unknown["retrieval."+rk] = anyVal
					}
				}
			}
		default:
			if !known[key] {
				var anyVal any
				_ = val.Decode(&anyVal)
				fm.Unknown[key] = anyVal
			}
		}
	}
	return fm, nil
}

func scalarString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Value)
}

func sequenceStrings(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		s := strings.TrimSpace(n.Value)
		if s == "" {
			return nil
		}
		return []string{s}
	}
	if n.Kind != yaml.SequenceNode {
		return nil
	}
	var out []string
	for _, c := range n.Content {
		s := strings.TrimSpace(c.Value)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func normalizeEvidence(e string) string {
	e = strings.TrimSpace(strings.ToUpper(e))
	switch e {
	case "E1", "E2", "E3", "E4":
		return e
	case "1":
		return "E1"
	case "2":
		return "E2"
	case "3":
		return "E3"
	case "4":
		return "E4"
	default:
		return e
	}
}

// ApplyFrontmatter merges frontmatter into DocMeta (path class already set).
func ApplyFrontmatter(dm *DocMeta, fm Frontmatter, pathWarnings *[]string) {
	if dm == nil || !fm.RawPresent {
		return
	}
	if fm.Title != "" {
		dm.Title = fm.Title
	}
	if fm.Component != "" {
		dm.Component = fm.Component
	}
	if fm.Level != "" {
		dm.Level = fm.Level
	}
	if fm.Reuse != "" {
		dm.Reuse = fm.Reuse
	}
	if fm.Status != "" {
		dm.Status = fm.Status
	}
	if fm.Evidence != "" {
		dm.EvidenceLevel = fm.Evidence
	}
	if len(fm.Platforms) > 0 {
		dm.Platforms = fm.Platforms
	}
	if len(fm.Topics) > 0 {
		dm.Topics = fm.Topics
	}
	if len(fm.Questions) > 0 {
		dm.RetrievalQuestions = fm.Questions
	}
	if len(fm.Related) > 0 {
		dm.RelatedDocs = fm.Related
	}
	if len(fm.SourcePaths) > 0 {
		clean, warns := NormalizeSourcePaths(fm.SourcePaths)
		dm.SourcePaths = clean
		if pathWarnings != nil {
			*pathWarnings = append(*pathWarnings, warns...)
		}
	}
	if len(fm.Unknown) > 0 {
		dm.UnknownFrontmatter = fm.Unknown
	}
}

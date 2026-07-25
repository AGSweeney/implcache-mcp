// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"regexp"
	"strings"

	"implcache-mcp/store"
)

var (
	reHeading = regexp.MustCompile(`(?m)^#{1,3}\s+(.+)$`)
	reOrdered = regexp.MustCompile(`(?m)^\s*\d+[\.\)]\s+(.+)$`)
	reBullet  = regexp.MustCompile(`(?m)^\s*[-*+]\s+(.+)$`)
	reAPIBack = regexp.MustCompile("`([A-Za-z_][\\w:.#]*)`")
)

type recipeFields struct {
	Summary      string
	APIs         []string
	Includes     []string
	Prereqs      []string
	Sequence     []string
	Cleanup      []string
	Constraints  []string
	Pitfalls     []string
	Examples     []string
	Version      string
	HasSequence  bool
	ReviewStatus string
	Authority    string
	URI          string
	Subject      string
	RootName     string
}

type mdSection struct {
	Title   string
	Content string
}

func parseRecipe(e store.KnowledgeEntry) recipeFields {
	rf := recipeFields{
		ReviewStatus: e.ReviewStatus,
		Authority:    e.Authority,
		URI:          e.URI,
		Subject:      e.Subject,
		RootName:     e.RootName,
		Version:      e.Version,
		Summary:      firstSentence(e.Subject + ". " + stripMD(e.BodyMarkdown)),
	}
	body := e.BodyMarkdown
	for _, sec := range splitMarkdownSections(body) {
		items := listItems(sec.Content)
		switch classifyRecipeSection(sec.Title) {
		case "sequence":
			rf.Sequence = append(rf.Sequence, items...)
			rf.HasSequence = len(items) > 0 || rf.HasSequence
		case "cleanup":
			rf.Cleanup = append(rf.Cleanup, items...)
			if len(items) > 0 {
				rf.HasSequence = true
				for _, it := range items {
					rf.Sequence = appendUnique(rf.Sequence, "Cleanup: "+it)
				}
			}
		case "includes":
			rf.Includes = append(rf.Includes, items...)
			for _, line := range strings.Split(sec.Content, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "#include") || strings.HasPrefix(line, "import ") {
					rf.Includes = appendUnique(rf.Includes, line)
				}
			}
		case "apis":
			rf.APIs = append(rf.APIs, extractAPITokens(sec.Content)...)
			rf.APIs = append(rf.APIs, items...)
		case "prereqs":
			rf.Prereqs = append(rf.Prereqs, items...)
		case "constraints":
			rf.Constraints = append(rf.Constraints, items...)
		case "pitfalls":
			rf.Pitfalls = append(rf.Pitfalls, items...)
		case "examples":
			ex := strings.TrimSpace(sec.Content)
			if ex != "" {
				rf.Examples = append(rf.Examples, store.ClipExcerpt(ex, 600))
			}
		case "version":
			if v := strings.TrimSpace(firstLine(sec.Content)); v != "" {
				rf.Version = v
			}
		}
	}
	// Ordered lists anywhere in body count as grounded sequence if under init-like prose.
	if !rf.HasSequence {
		if items := reOrdered.FindAllStringSubmatch(body, -1); len(items) >= 2 {
			lower := strings.ToLower(body)
			if containsAny(lower, "initialize", "init", "sequence", "steps", "then call", "call order") {
				for _, m := range items {
					rf.Sequence = append(rf.Sequence, strings.TrimSpace(m[1]))
				}
				rf.HasSequence = len(rf.Sequence) > 0
			}
		}
	}
	rf.APIs = uniqueStrings(rf.APIs)
	rf.Includes = uniqueStrings(rf.Includes)
	return rf
}

// classifyRecipeSection maps a markdown heading to a recipe field.
// Order follows specific phrases first. "required" alone is never treated as APIs.
// Cleanup is checked before bare "requirements" so "Cleanup requirements" stays cleanup.
func classifyRecipeSection(title string) string {
	key := strings.ToLower(strings.TrimSpace(title))
	if key == "" {
		return "body"
	}
	switch {
	case containsAny(key, "cleanup", "teardown", "shutdown", "terminate"):
		return "cleanup"
	case containsAny(key, "include", "import") || strings.Contains(key, "header"):
		return "includes"
	case containsAny(key, "prereq", "prerequisite") ||
		(containsAny(key, "requirement", "requirements") && !containsAny(key, "api", "cleanup")):
		return "prereqs"
	case containsAny(key, "required api", "required apis", "api reference") ||
		(strings.Contains(key, "api") && !strings.Contains(key, "header")) ||
		(containsAny(key, "functions", "symbols") && !containsAny(key, "sequence", "init")):
		return "apis"
	case containsAny(key, "initialization", "init order"):
		return "sequence"
	case containsAny(key, "sequence", "steps", "procedure", "workflow", "call order"):
		return "sequence"
	case containsAny(key, "constraint") || strings.Contains(key, "must not") ||
		(strings.Contains(key, "rule") && !strings.Contains(key, "header")):
		return "constraints"
	case containsAny(key, "pitfall", "gotcha", "warning", "common error", "avoid"):
		return "pitfalls"
	case containsAny(key, "example", "sample"):
		return "examples"
	case containsAny(key, "version"):
		return "version"
	default:
		return "body"
	}
}

// splitMarkdownSections returns heading sections in source order (deterministic).
func splitMarkdownSections(body string) []mdSection {
	idxs := reHeading.FindAllStringSubmatchIndex(body, -1)
	if len(idxs) == 0 {
		return []mdSection{{Title: "", Content: body}}
	}
	out := make([]mdSection, 0, len(idxs))
	for i, m := range idxs {
		title := strings.TrimSpace(body[m[2]:m[3]])
		start := m[1]
		end := len(body)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		out = append(out, mdSection{Title: title, Content: strings.TrimSpace(body[start:end])})
	}
	return out
}

func listItems(content string) []string {
	var out []string
	for _, m := range reOrdered.FindAllStringSubmatch(content, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	for _, m := range reBullet.FindAllStringSubmatch(content, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

func extractAPITokens(s string) []string {
	var out []string
	for _, m := range reAPIBack.FindAllStringSubmatch(s, -1) {
		tok := m[1]
		if looksIdent(tok) {
			out = append(out, tok)
		}
	}
	return out
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

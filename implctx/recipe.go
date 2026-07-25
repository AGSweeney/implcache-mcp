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
	sections := splitMarkdownSections(body)
	for title, content := range sections {
		key := strings.ToLower(title)
		items := listItems(content)
		switch {
		case containsAny(key, "sequence", "steps", "procedure", "workflow", "initialization", "init order", "call order"):
			rf.Sequence = append(rf.Sequence, items...)
			rf.HasSequence = len(items) > 0
		case containsAny(key, "cleanup", "teardown", "shutdown", "terminate", "lifecycle"):
			rf.Cleanup = append(rf.Cleanup, items...)
			if len(items) > 0 {
				rf.HasSequence = true
				for _, it := range items {
					rf.Sequence = appendUnique(rf.Sequence, "Cleanup: "+it)
				}
			}
		case containsAny(key, "include", "import", "header"):
			rf.Includes = append(rf.Includes, items...)
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "#include") || strings.HasPrefix(line, "import ") {
					rf.Includes = appendUnique(rf.Includes, line)
				}
			}
		case containsAny(key, "api", "symbol", "function", "required"):
			rf.APIs = append(rf.APIs, extractAPITokens(content)...)
			rf.APIs = append(rf.APIs, items...)
		case containsAny(key, "prereq", "requirement", "before you"):
			rf.Prereqs = append(rf.Prereqs, items...)
		case containsAny(key, "constraint", "must", "rule"):
			rf.Constraints = append(rf.Constraints, items...)
		case containsAny(key, "pitfall", "gotcha", "warning", "common error", "avoid"):
			rf.Pitfalls = append(rf.Pitfalls, items...)
		case containsAny(key, "example", "sample"):
			ex := strings.TrimSpace(content)
			if ex != "" {
				rf.Examples = append(rf.Examples, store.ClipExcerpt(ex, 600))
			}
		case containsAny(key, "version"):
			if v := strings.TrimSpace(firstLine(content)); v != "" {
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

func splitMarkdownSections(body string) map[string]string {
	idxs := reHeading.FindAllStringSubmatchIndex(body, -1)
	out := map[string]string{}
	if len(idxs) == 0 {
		out[""] = body
		return out
	}
	for i, m := range idxs {
		title := strings.TrimSpace(body[m[2]:m[3]])
		start := m[1]
		end := len(body)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		out[title] = strings.TrimSpace(body[start:end])
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

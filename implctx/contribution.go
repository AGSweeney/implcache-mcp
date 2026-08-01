// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"path"
	"sort"
	"strings"

	"implcache-mcp/store"
)

// RootCiteCount is citation volume for one contributing root.
type RootCiteCount struct {
	RootName string `json:"rootName"`
	Count    int    `json:"count"`
}

// RootContribution records searched vs package-contributing roots for the final package.
// Diagnostic only — excluded from context fingerprint and token estimates.
type RootContribution struct {
	RootsSearched         int             `json:"rootsSearched"`
	RootsContributing     int             `json:"rootsContributing"`
	SearchedRoots         []string        `json:"searchedRoots,omitempty"`
	ContributingRoots     []string        `json:"contributingRoots,omitempty"`
	UnusedSearchedRoots   []string        `json:"unusedSearchedRoots,omitempty"`
	CitationsByRoot       []RootCiteCount `json:"citationsByRoot,omitempty"`
	RelatedContributing   int             `json:"relatedContributing,omitempty"`
	MaxRelatedProjects    int             `json:"maxRelatedProjects,omitempty"`
	RelatedOverLimit      int             `json:"relatedOverLimit,omitempty"`
	NearDuplicatePairs    int             `json:"nearDuplicatePairs,omitempty"`
}

// attachRootContribution fills resp.RootContribution from the final trimmed package.
func attachRootContribution(resp *Response, searched []string, memberRoles map[string]string, policies store.KnowledgeGroupPolicies) {
	if resp == nil {
		return
	}
	searched = uniqueNonEmpty(searched)
	citeCounts := map[string]int{}
	contributing := map[string]struct{}{}

	addRoot := func(root string, asCitation bool) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		contributing[root] = struct{}{}
		if asCitation {
			citeCounts[root]++
		}
	}

	for _, c := range resp.Citations {
		root := c.RootName
		if root == "" {
			root = rootFromURI(c.URI)
		}
		addRoot(root, true)
	}
	for _, e := range resp.Examples {
		root := e.RootName
		if root == "" {
			root = rootFromURI(e.URI)
		}
		// Examples count as contribution even when not also cited.
		addRoot(root, false)
		if _, ok := citeCounts[root]; !ok {
			// Ensure example-only roots appear in citations-by-root as 0? User asked
			// citations by root — keep citation counts separate; example-only still contributes.
		}
	}
	for _, s := range resp.RelevantSymbols {
		addRoot(s.RootName, false)
	}

	var contribList []string
	for r := range contributing {
		contribList = append(contribList, r)
	}
	sort.Strings(contribList)

	var unused []string
	for _, r := range searched {
		if _, ok := contributing[r]; !ok {
			unused = append(unused, r)
		}
	}
	sort.Strings(unused)

	byRoot := make([]RootCiteCount, 0, len(citeCounts))
	for r, n := range citeCounts {
		byRoot = append(byRoot, RootCiteCount{RootName: r, Count: n})
	}
	sort.Slice(byRoot, func(i, j int) bool {
		if byRoot[i].Count != byRoot[j].Count {
			return byRoot[i].Count > byRoot[j].Count
		}
		return byRoot[i].RootName < byRoot[j].RootName
	})

	relatedN := 0
	for _, r := range contribList {
		role := ""
		if memberRoles != nil {
			role = memberRoles[r]
		}
		if role == store.MemberRoleRelatedProject {
			relatedN++
		}
	}
	maxRelated := policies.MaxRelatedProjects
	if !policies.IncludeRelatedProjectsForPatterns {
		maxRelated = 0
	}
	over := relatedN - maxRelated
	if over < 0 {
		over = 0
	}

	resp.RootContribution = &RootContribution{
		RootsSearched:       len(searched),
		RootsContributing:   len(contribList),
		SearchedRoots:       searched,
		ContributingRoots:   contribList,
		UnusedSearchedRoots: unused,
		CitationsByRoot:     byRoot,
		RelatedContributing: relatedN,
		MaxRelatedProjects:  maxRelated,
		RelatedOverLimit:    over,
		NearDuplicatePairs:  countNearDuplicateCrossRoot(resp),
	}
}

func rootFromURI(uri string) string {
	uri = strings.TrimSpace(uri)
	for _, prefix := range []string{"project://", "pdf://", "git://"} {
		if strings.HasPrefix(uri, prefix) {
			rest := strings.TrimPrefix(uri, prefix)
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				return rest[:i]
			}
			return rest
		}
	}
	return ""
}

type evidenceItem struct {
	root     string
	uri      string
	basename string
	titleKey string
}

func countNearDuplicateCrossRoot(resp *Response) int {
	if resp == nil {
		return 0
	}
	var items []evidenceItem
	seenURI := map[string]struct{}{}
	add := func(uri, root, title string) {
		uri = strings.TrimSpace(uri)
		if uri == "" {
			return
		}
		key := strings.ToLower(uri)
		if _, ok := seenURI[key]; ok {
			return
		}
		seenURI[key] = struct{}{}
		if strings.TrimSpace(root) == "" {
			root = rootFromURI(uri)
		}
		base := path.Base(uriPath(uri))
		items = append(items, evidenceItem{
			root:     root,
			uri:      uri,
			basename: strings.ToLower(base),
			titleKey: normalizeTitleKey(title),
		})
	}
	for _, c := range resp.Citations {
		add(c.URI, c.RootName, c.Title)
	}
	for _, e := range resp.Examples {
		add(e.URI, e.RootName, e.Title)
	}

	pairs := 0
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			a, b := items[i], items[j]
			if a.root == "" || b.root == "" || strings.EqualFold(a.root, b.root) {
				continue
			}
			if nearDuplicateEvidence(a, b) {
				pairs++
			}
		}
	}
	return pairs
}

func uriPath(uri string) string {
	for _, prefix := range []string{"project://", "pdf://", "git://"} {
		if strings.HasPrefix(uri, prefix) {
			rest := strings.TrimPrefix(uri, prefix)
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				return rest[i+1:]
			}
			return ""
		}
	}
	return uri
}

func normalizeTitleKey(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	if title == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func nearDuplicateEvidence(a, b evidenceItem) bool {
	if a.basename != "" && a.basename == b.basename && a.basename != "." && a.basename != "/" {
		return true
	}
	if a.titleKey != "" && a.titleKey == b.titleKey && len(a.titleKey) >= 8 {
		return true
	}
	// Shared distinctive path suffix (e.g. .../tcp/listen.md vs .../docs/tcp/listen.md).
	pa, pb := uriPath(a.uri), uriPath(b.uri)
	if pa != "" && pb != "" {
		sa, sb := strings.ToLower(pa), strings.ToLower(pb)
		if strings.HasSuffix(sa, sb) || strings.HasSuffix(sb, sa) {
			shorter := sa
			if len(sb) < len(sa) {
				shorter = sb
			}
			if strings.Count(shorter, "/") >= 1 && len(shorter) >= 12 {
				return true
			}
		}
	}
	return false
}

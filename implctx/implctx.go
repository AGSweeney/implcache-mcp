// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"fmt"
	"strings"

	"implcache-mcp/store"
)

// Request asks for a compact implementation package for a coding task.
type Request struct {
	Task             string   `json:"task"`
	Language         string   `json:"language,omitempty"`
	Technology       string   `json:"technology,omitempty"`
	ProjectRoot      string   `json:"projectRoot,omitempty"`
	PreferredRoots   []string `json:"preferredRoots,omitempty"`
	RootGroup        string   `json:"rootGroup,omitempty"`
	MaxContextTokens int      `json:"maxContextTokens,omitempty"`
}

// Citation points at a grounded source.
type Citation struct {
	URI       string `json:"uri"`
	Title     string `json:"title,omitempty"`
	Section   string `json:"section,omitempty"`
	Lines     string `json:"lines,omitempty"`
	Authority string `json:"authority,omitempty"`
	RootName  string `json:"rootName,omitempty"`
}

// ExampleRef is a short cited example.
type ExampleRef struct {
	URI       string `json:"uri"`
	Title     string `json:"title,omitempty"`
	Excerpt   string `json:"excerpt"`
	Authority string `json:"authority,omitempty"`
	Lines     string `json:"lines,omitempty"`
}

// Response is a budgeted implementation-context package.
type Response struct {
	Task                 string         `json:"task"`
	Technology           string         `json:"technology,omitempty"`
	Language             string         `json:"language,omitempty"`
	Summary              string         `json:"summary"`
	RequiredAPIs         []string       `json:"requiredApis,omitempty"`
	RelevantSymbols      []store.Symbol `json:"relevantSymbols,omitempty"`
	Includes             []string       `json:"includes,omitempty"`
	Sequence             []string       `json:"sequence,omitempty"`
	Examples             []ExampleRef   `json:"examples,omitempty"`
	Constraints          []string       `json:"constraints,omitempty"`
	Pitfalls             []string       `json:"pitfalls,omitempty"`
	ProjectConventions   []string       `json:"projectConventions,omitempty"`
	Citations            []Citation     `json:"citations"`
	Coverage             string         `json:"coverage"` // high|medium|low
	Freshness            string         `json:"freshness"`
	WebSearchRecommended bool           `json:"webSearchRecommended"`
	MissingInformation   []string       `json:"missingInformation,omitempty"`
	RecommendedFollowUp  []string       `json:"recommendedFollowUp,omitempty"`
	RootsUsed            []string       `json:"rootsUsed"`
	EstimatedTokens      int            `json:"estimatedTokens"`
	Chars                int            `json:"chars"`
	TokenEstimateNote    string         `json:"tokenEstimateNote"`
}

// Get assembles a compact implementation package from local knowledge.
func Get(ctx context.Context, st *store.Store, req Request) (*Response, error) {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return nil, fmt.Errorf("task is required")
	}
	budget := store.DefaultContextBudget()
	if req.MaxContextTokens > 0 {
		budget.MaxTokensEstimate = req.MaxContextTokens
		budget.MaxTotalChars = req.MaxContextTokens * 4
	}

	roots, err := resolveRoots(ctx, st, req)
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		inf, err := st.ResolveRoots(ctx, task+" "+req.Technology+" "+req.Language, nil)
		if err != nil {
			return nil, err
		}
		if inf.NeedsChoice {
			return nil, &store.ErrNeedsRoot{Inference: inf}
		}
		roots = inf.Roots
	}

	resp := &Response{
		Task:              task,
		Technology:        req.Technology,
		Language:          req.Language,
		RootsUsed:         roots,
		Freshness:         "unknown",
		TokenEstimateNote: "estimated as utf8_runes/4",
	}

	// 1) Recipes first (curated > generated).
	recipes, _ := st.SearchKnowledgeEntries(ctx, task, req.Technology, req.Language, roots, 3)
	for _, r := range recipes {
		if r.ReviewStatus == store.ReviewHumanReviewed || len(resp.Sequence) == 0 {
			resp.Summary = firstSentence(r.Subject + ". " + stripMD(r.BodyMarkdown))
			resp.Citations = append(resp.Citations, Citation{
				URI: r.URI, Title: r.Subject, Authority: r.Authority, RootName: r.RootName,
			})
		}
	}

	// 2) Symbol hits from task tokens that look like identifiers.
	for _, tok := range symbolTokens(task) {
		syms, err := st.FindSymbols(ctx, tok, roots, 5)
		if err != nil {
			continue
		}
		for _, sym := range syms {
			resp.RelevantSymbols = append(resp.RelevantSymbols, sym)
			resp.RequiredAPIs = appendUnique(resp.RequiredAPIs, sym.Name)
			resp.Citations = append(resp.Citations, Citation{
				URI: sym.URI, Title: sym.Title, Section: sym.Kind,
				Lines:     fmt.Sprintf("%d-%d", sym.StartLine, sym.EndLine),
				Authority: sym.Authority, RootName: sym.RootName,
			})
			if len(resp.RequiredAPIs) >= 8 {
				break
			}
		}
	}

	// 3) Budgeted FTS for examples / constraints / pitfalls.
	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query:     task,
		Limit:     budget.MaxResults * 3,
		Roots:     roots,
		MaxPerDoc: budget.MaxPerDocument,
	})
	if err != nil {
		return nil, err
	}

	totalChars := 0
	addChars := func(s string) bool {
		totalChars += len([]rune(s))
		return store.EstimateTokens(strings.Repeat("x", totalChars)) <= budget.MaxTokensEstimate &&
			totalChars <= budget.MaxTotalChars
	}

	for _, h := range hits {
		ex := store.ClipExcerpt(cleanupSnippet(h.Snippet), budget.MaxExcerptChars)
		if !addChars(ex) {
			break
		}
		cit := Citation{
			URI: h.URI, Title: h.Title, Section: h.Heading,
			Lines:     lineRange(h.StartLine, h.EndLine),
			Authority: h.Authority, RootName: h.RootName,
		}
		resp.Citations = append(resp.Citations, cit)

		lower := strings.ToLower(h.Heading + " " + h.Snippet)
		switch {
		case len(resp.Examples) < budget.MaxExamples &&
			(h.Authority == store.AuthorityCurrentProject || h.Authority == store.AuthorityOfficialExample ||
				h.Authority == store.AuthorityRelatedProject || strings.Contains(lower, "example")):
			resp.Examples = append(resp.Examples, ExampleRef{
				URI: h.URI, Title: h.Title, Excerpt: ex, Authority: h.Authority, Lines: cit.Lines,
			})
		case strings.Contains(lower, "pitfall") || strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
			resp.Pitfalls = appendUnique(resp.Pitfalls, store.ClipExcerpt(cleanupSnippet(h.Snippet), 180))
		case strings.Contains(lower, "must") || strings.Contains(lower, "require") || strings.Contains(lower, "constraint"):
			resp.Constraints = appendUnique(resp.Constraints, store.ClipExcerpt(cleanupSnippet(h.Snippet), 180))
		case h.Authority == store.AuthorityCurrentProject:
			resp.ProjectConventions = appendUnique(resp.ProjectConventions, store.ClipExcerpt(cleanupSnippet(h.Snippet), 160))
		}

		for _, inc := range extractIncludes(h.Snippet) {
			resp.Includes = appendUnique(resp.Includes, inc)
		}
		for _, api := range extractProAPIs(h.Snippet) {
			resp.RequiredAPIs = appendUnique(resp.RequiredAPIs, api)
		}
	}

	if resp.Summary == "" {
		resp.Summary = fmt.Sprintf("Implementation context for %q from roots %s.", task, strings.Join(roots, ", "))
	}
	if len(resp.Sequence) == 0 && len(resp.RequiredAPIs) > 0 {
		for _, api := range resp.RequiredAPIs {
			if len(resp.Sequence) >= 6 {
				break
			}
			resp.Sequence = append(resp.Sequence, "Use `"+api+"`")
		}
	}

	resp.Coverage = coverageOf(resp)
	resp.WebSearchRecommended = resp.Coverage == "low"
	if resp.Coverage == "low" {
		resp.MissingInformation = append(resp.MissingInformation, "Few grounded local hits; verify against current vendor docs if versions matter.")
	}
	if len(resp.Examples) == 0 {
		resp.RecommendedFollowUp = append(resp.RecommendedFollowUp, "find_examples or search_knowledge for a worked sample")
	}
	if len(resp.RelevantSymbols) == 0 && len(resp.RequiredAPIs) > 0 {
		resp.RecommendedFollowUp = append(resp.RecommendedFollowUp, "find_symbol on a required API for signature/lineage")
	}
	resp.RecommendedFollowUp = append(resp.RecommendedFollowUp, "get_document on a citation URI only if deeper context is required")

	// Deduplicate citations
	resp.Citations = dedupeCitations(resp.Citations)
	body := resp.Summary + strings.Join(resp.RequiredAPIs, " ") + strings.Join(resp.Sequence, " ")
	for _, e := range resp.Examples {
		body += e.Excerpt
	}
	resp.Chars = len([]rune(body))
	resp.EstimatedTokens = store.EstimateTokens(body)
	return resp, nil
}

func resolveRoots(ctx context.Context, st *store.Store, req Request) ([]string, error) {
	seen := map[string]struct{}{}
	var roots []string
	add := func(r string) {
		r = strings.TrimSpace(r)
		if r == "" {
			return
		}
		if _, ok := seen[r]; ok {
			return
		}
		seen[r] = struct{}{}
		roots = append(roots, r)
	}
	if req.ProjectRoot != "" {
		add(req.ProjectRoot)
	}
	for _, r := range req.PreferredRoots {
		add(r)
	}
	if g := strings.TrimSpace(req.RootGroup); g != "" {
		members, err := st.ListRootGroupMembers(ctx, g)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			add(m.RootName)
		}
	}
	return roots, nil
}

func symbolTokens(task string) []string {
	var out []string
	for _, f := range strings.Fields(task) {
		f = strings.Trim(f, ".,;:()[]{}\"'")
		if store.NormalizeSymbol(f) == "" {
			continue
		}
		if looksIdent(f) {
			out = append(out, f)
		}
	}
	return out
}

func looksIdent(s string) bool {
	if len(s) < 3 {
		return false
	}
	return strings.ContainsAny(s, "_:.#") || (hasUpper(s) && hasLower(s)) || strings.HasPrefix(s, "Pro") || strings.HasPrefix(s, "pfc")
}

func hasUpper(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}
func hasLower(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

func extractIncludes(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#include") {
			out = append(out, line)
		}
	}
	return out
}

func extractProAPIs(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		f = strings.Trim(f, "`,.;()<>\"")
		if strings.HasPrefix(f, "Pro") && len(f) > 5 {
			out = append(out, f)
		}
		if strings.HasPrefix(f, "pfc") || strings.HasPrefix(f, "wfc") {
			out = append(out, f)
		}
	}
	return out
}

func cleanupSnippet(s string) string {
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	return strings.TrimSpace(s)
}

func lineRange(a, b int) string {
	if a <= 0 && b <= 0 {
		return ""
	}
	if b <= 0 || b == a {
		return fmt.Sprintf("%d", a)
	}
	return fmt.Sprintf("%d-%d", a, b)
}

func appendUnique(xs []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return xs
	}
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}

func dedupeCitations(in []Citation) []Citation {
	seen := map[string]struct{}{}
	var out []Citation
	for _, c := range in {
		key := c.URI + "|" + c.Lines
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func coverageOf(r *Response) string {
	score := 0
	if len(r.RequiredAPIs) > 0 || len(r.RelevantSymbols) > 0 {
		score += 2
	}
	if len(r.Examples) > 0 {
		score += 2
	}
	if len(r.Citations) >= 3 {
		score++
	}
	if len(r.Constraints)+len(r.Pitfalls) > 0 {
		score++
	}
	switch {
	case score >= 5:
		return "high"
	case score >= 2:
		return "medium"
	default:
		return "low"
	}
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 280 {
		s = string([]rune(s)[:280]) + "…"
	}
	return s
}

func stripMD(s string) string {
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "*", "")
	return strings.TrimSpace(s)
}

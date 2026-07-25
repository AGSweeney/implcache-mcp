// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"implcache-mcp/ingest"
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
	Version   string `json:"version,omitempty"`
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
// Empty slices/strings with omitempty are omitted from JSON.
type Response struct {
	Task                 string         `json:"task"`
	Technology           string         `json:"technology,omitempty"`
	Language             string         `json:"language,omitempty"`
	Summary              string         `json:"summary,omitempty"`
	RequiredAPIs         []string       `json:"requiredApis,omitempty"`
	RelevantSymbols      []store.Symbol `json:"relevantSymbols,omitempty"`
	Includes             []string       `json:"includes,omitempty"`
	Prerequisites        []string       `json:"prerequisites,omitempty"`
	Sequence             []string       `json:"sequence,omitempty"`
	Examples             []ExampleRef   `json:"examples,omitempty"`
	Constraints          []string       `json:"constraints,omitempty"`
	Pitfalls             []string       `json:"pitfalls,omitempty"`
	ProjectConventions   []string       `json:"projectConventions,omitempty"`
	Citations            []Citation     `json:"citations,omitempty"`
	Coverage             string         `json:"coverage,omitempty"`
	Freshness            string         `json:"freshness,omitempty"`
	WebSearchRecommended bool           `json:"webSearchRecommended,omitempty"`
	MissingInformation   []string       `json:"missingInformation,omitempty"`
	RecommendedFollowUp  []string       `json:"recommendedFollowUp,omitempty"`
	RootsUsed            []string       `json:"rootsUsed,omitempty"`
	RecipeReviewStatus   string         `json:"recipeReviewStatus,omitempty"`
	Version              string         `json:"version,omitempty"`
	ContextFingerprint   string         `json:"contextFingerprint,omitempty"`
	EstimatedTokens      int            `json:"estimatedTokens,omitempty"`
	Chars                int            `json:"chars,omitempty"`
	TokenEstimateNote    string         `json:"tokenEstimateNote,omitempty"`
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
		TokenEstimateNote: "estimated from serialized JSON payload (utf8_runes/4)",
	}

	sequenceGrounded := false
	var versions []string
	archivedHints := 0

	// 1) Recipes — human-reviewed first; populate structured fields.
	recipes, _ := st.SearchKnowledgeEntries(ctx, task, req.Technology, req.Language, roots, 5)
	for _, r := range recipes {
		rf := parseRecipe(r)
		prefer := r.ReviewStatus == store.ReviewHumanReviewed || resp.RecipeReviewStatus == ""
		if !prefer {
			continue
		}
		if resp.RecipeReviewStatus == "" || r.ReviewStatus == store.ReviewHumanReviewed {
			resp.RecipeReviewStatus = r.ReviewStatus
			if rf.Summary != "" {
				resp.Summary = rf.Summary
			}
			if rf.Version != "" {
				resp.Version = rf.Version
				versions = append(versions, rf.Version)
			}
			for _, api := range rf.APIs {
				resp.RequiredAPIs = appendUnique(resp.RequiredAPIs, api)
			}
			for _, inc := range rf.Includes {
				resp.Includes = appendUnique(resp.Includes, inc)
			}
			for _, p := range rf.Prereqs {
				resp.Prerequisites = appendUnique(resp.Prerequisites, p)
			}
			for _, c := range rf.Constraints {
				resp.Constraints = appendUnique(resp.Constraints, c)
			}
			for _, p := range rf.Pitfalls {
				resp.Pitfalls = appendUnique(resp.Pitfalls, p)
			}
			if rf.HasSequence && len(rf.Sequence) > 0 {
				resp.Sequence = append([]string{}, rf.Sequence...)
				sequenceGrounded = true
			}
			for _, ex := range rf.Examples {
				if len(resp.Examples) >= budget.MaxExamples {
					break
				}
				resp.Examples = append(resp.Examples, ExampleRef{
					URI: r.URI, Title: r.Subject, Excerpt: ex, Authority: r.Authority,
				})
			}
			resp.Citations = append(resp.Citations, Citation{
				URI: r.URI, Title: r.Subject, Authority: r.Authority, RootName: r.RootName, Version: r.Version,
			})
			if r.ReviewStatus == store.ReviewHumanReviewed {
				break
			}
		}
	}

	// 2) Symbol hits from explicit identifier-like task tokens.
	taskToks := symbolTokens(task)
	for _, tok := range taskToks {
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

	// 3) Budgeted FTS for examples / constraints / pitfalls / grounded workflow text.
	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query:     task,
		Limit:     budget.MaxResults * 3,
		Roots:     roots,
		MaxPerDoc: budget.MaxPerDocument,
	})
	if err != nil {
		return nil, err
	}

	// 3b) Natural-language tasks: harvest symbols from retrieved docs when few ID cues.
	if len(taskToks) < 2 || len(resp.RelevantSymbols) == 0 {
		for _, sym := range harvestSymbolsFromHits(ctx, st, roots, hits, task, 8) {
			already := false
			for _, existing := range resp.RelevantSymbols {
				if existing.ID == sym.ID {
					already = true
					break
				}
			}
			if already {
				continue
			}
			resp.RelevantSymbols = append(resp.RelevantSymbols, sym)
			resp.RequiredAPIs = appendUnique(resp.RequiredAPIs, sym.Name)
			resp.Citations = append(resp.Citations, Citation{
				URI: sym.URI, Title: sym.Title, Section: sym.Kind,
				Lines:     fmt.Sprintf("%d-%d", sym.StartLine, sym.EndLine),
				Authority: sym.Authority, RootName: sym.RootName,
			})
		}
	}

	for _, h := range hits {
		ex := store.ClipExcerpt(cleanupSnippet(h.Snippet), budget.MaxExcerptChars)
		ver := h.ProductVersion
		if ver == "" {
			ver = ingest.InferProductVersion(h.RootName, h.Path, h.Heading+"\n"+h.Snippet)
		}
		cit := Citation{
			URI: h.URI, Title: h.Title, Section: h.Heading,
			Lines:     lineRange(h.StartLine, h.EndLine),
			Authority: h.Authority, RootName: h.RootName, Version: ver,
		}
		resp.Citations = append(resp.Citations, cit)
		if ver != "" {
			versions = append(versions, ver)
		}
		if h.ArchivedHint() {
			archivedHints++
		}

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

		// Grounded sequence from ordered workflow sections in retrieved docs.
		if !sequenceGrounded && containsAny(lower, "sequence", "steps", "initialization", "init order", "call order") {
			if items := listItems(cleanupSnippet(h.Snippet)); len(items) >= 2 {
				resp.Sequence = items
				sequenceGrounded = true
			}
		}

		for _, inc := range extractIncludes(h.Snippet) {
			resp.Includes = appendUnique(resp.Includes, inc)
		}
		for _, api := range extractAPILike(h.Snippet) {
			resp.RequiredAPIs = appendUnique(resp.RequiredAPIs, api)
		}
	}

	if resp.Summary == "" {
		resp.Summary = fmt.Sprintf("Implementation context for %q from roots %s.", task, strings.Join(roots, ", "))
	}
	if !sequenceGrounded {
		resp.Sequence = nil
		if len(resp.RequiredAPIs) > 0 {
			resp.MissingInformation = append(resp.MissingInformation,
				"Required APIs found, but no source-grounded call sequence was located.")
		}
	}

	resp.Citations = dedupeCitations(resp.Citations)
	if resp.Summary != "" && (len(resp.RequiredAPIs) > 0 || len(resp.Examples) > 0) {
		resp.Summary = firstSentence(resp.Summary)
	}

	resp.Coverage = coverageOf(resp)
	resp.Freshness = freshnessFromSources(resp.Citations, versions, archivedHints)
	resp.WebSearchRecommended = webSearchFrom(resp.Coverage, resp.Freshness)
	if resp.Coverage == "low" {
		resp.MissingInformation = appendUnique(resp.MissingInformation, "Few grounded local hits; verify against current vendor docs if versions matter.")
	}
	if len(resp.Examples) == 0 {
		resp.RecommendedFollowUp = append(resp.RecommendedFollowUp, "search_knowledge for a worked sample")
	}
	if len(resp.RelevantSymbols) == 0 && len(resp.RequiredAPIs) > 0 {
		resp.RecommendedFollowUp = append(resp.RecommendedFollowUp, "find_symbol on a required API for signature/lineage")
	}
	resp.RecommendedFollowUp = append(resp.RecommendedFollowUp, "get_document on a citation URI only if deeper context is required")

	resp.ContextFingerprint = fingerprintResponse(ctx, st, req, resp)
	trimToBudget(resp, budget.MaxTokensEstimate)
	chars, tokens, _ := serializeTokens(resp)
	resp.Chars = chars
	resp.EstimatedTokens = tokens
	return resp, nil
}

func fingerprintResponse(ctx context.Context, st *store.Store, req Request, resp *Response) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(strings.ToLower(req.Task)))
	b.WriteByte('|')
	b.WriteString(strings.ToLower(req.Language))
	b.WriteByte('|')
	b.WriteString(strings.ToLower(req.Technology))
	b.WriteByte('|')
	roots := append([]string{}, resp.RootsUsed...)
	sort.Strings(roots)
	b.WriteString(strings.Join(roots, ","))
	b.WriteByte('|')
	apis := append([]string{}, resp.RequiredAPIs...)
	sort.Strings(apis)
	b.WriteString(strings.Join(apis, ","))
	b.WriteByte('|')
	for _, c := range resp.Citations {
		b.WriteString(c.URI)
		b.WriteByte('#')
		b.WriteString(c.Lines)
		if h, err := st.GetHashByURI(ctx, c.URI); err == nil && h != "" {
			b.WriteByte('@')
			b.WriteString(h)
		}
		b.WriteByte(';')
	}
	for _, s := range resp.RelevantSymbols {
		b.WriteString(s.NameNorm)
		b.WriteByte('@')
		b.WriteString(s.URI)
		b.WriteByte(';')
	}
	for _, e := range resp.Examples {
		b.WriteString(e.URI)
		b.WriteByte('|')
		b.WriteString(e.Excerpt)
		b.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:16])
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
	return ingest.LooksLikeAPIToken(s)
}

func extractIncludes(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#include") || strings.HasPrefix(line, "import ") {
			out = append(out, line)
		}
	}
	return out
}

func extractAPILike(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		f = strings.Trim(f, "`,.;()<>\"'")
		if looksIdent(f) && len(f) > 4 {
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
	if len(r.Sequence) > 0 {
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

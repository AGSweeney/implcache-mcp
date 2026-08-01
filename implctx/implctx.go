// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"implcache-mcp/ingest"
	"implcache-mcp/librarydocs"
	"implcache-mcp/store"
)

// Request asks for a compact implementation package for a coding task.
type Request struct {
	Task             string   `json:"task"`
	Language         string   `json:"language,omitempty"`
	Technology       string   `json:"technology,omitempty"`
	Version          string   `json:"version,omitempty"` // optional requested product/API version
	ProjectRoot      string   `json:"projectRoot,omitempty"`
	PreferredRoots   []string `json:"preferredRoots,omitempty"`
	// KnowledgeGroup is the preferred API for trusted cross-root retrieval.
	KnowledgeGroup string `json:"knowledgeGroup,omitempty"`
	// RootGroup is a deprecated alias for KnowledgeGroup (same semantics).
	RootGroup        string `json:"rootGroup,omitempty"`
	MaxContextTokens int    `json:"maxContextTokens,omitempty"`
	// MaxResults is a server-side retrieval ceiling supplied by the tool layer.
	MaxResults int `json:"-"`
	// Semantic supplements FTS with sparse term-vector similarity (server -enable-semantic).
	Semantic bool `json:"semantic,omitempty"`
	// Debug includes task-token extraction in the response.
	Debug bool `json:"debug,omitempty"`
}

// Citation points at a grounded source.
type Citation struct {
	URI        string   `json:"uri"`
	Title      string   `json:"title,omitempty"`
	Section    string   `json:"section,omitempty"`
	Lines      string   `json:"lines,omitempty"`
	Authority  string   `json:"authority,omitempty"`
	RootName   string   `json:"rootName,omitempty"`
	Version    string   `json:"version,omitempty"`
	SourceURIs []string `json:"sourceUris,omitempty"` // recipe lineage
	// LibraryDocs enrichment (optional; omitted for non-LibraryDocs citations).
	ComponentID    string   `json:"componentId,omitempty"`
	Component      string   `json:"component,omitempty"`
	ContentClass   string   `json:"contentClass,omitempty"`
	DocStatus      string   `json:"docStatus,omitempty"`
	EvidenceLevel  string   `json:"evidenceLevel,omitempty"`
	RelatedSources []string `json:"relatedSources,omitempty"`
	ArtifactIDs    []string `json:"artifactIds,omitempty"`
}

// ExampleRef is a short cited example.
type ExampleRef struct {
	URI       string `json:"uri"`
	Title     string `json:"title,omitempty"`
	Excerpt   string `json:"excerpt"`
	Authority string `json:"authority,omitempty"`
	RootName  string `json:"rootName,omitempty"`
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
	RootsUsed            []string       `json:"rootsUsed,omitempty"` // roots searched
	KnowledgeGroup       string         `json:"knowledgeGroup,omitempty"`
	RecipeReviewStatus   string         `json:"recipeReviewStatus,omitempty"`
	Version              string         `json:"version,omitempty"`
	DebugTaskTokens      []string       `json:"debugTaskTokens,omitempty"`
	ContextFingerprint   string            `json:"contextFingerprint,omitempty"`
	EstimatedTokens      int               `json:"estimatedTokens,omitempty"`
	Chars                int               `json:"chars,omitempty"`
	TokenEstimateNote    string            `json:"tokenEstimateNote,omitempty"`
	// SelectionTrace explains package assembly choices (hydration, pins, diversity).
	SelectionTrace []SelectionReason `json:"selectionTrace,omitempty"`
	// RootContribution compares searched vs package-contributing roots (diagnostic).
	RootContribution *RootContribution `json:"rootContribution,omitempty"`
}

// SelectionReason is one package-assembly decision for debugging/regression.
type SelectionReason struct {
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
	URI    string `json:"uri,omitempty"`
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

	roots, kgID, memberRoles, err := resolveRoots(ctx, st, req)
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
		kgID = inf.KnowledgeGroup
		if kgID != "" {
			if g, err := st.LookupKnowledgeGroup(ctx, kgID); err == nil && g != nil {
				memberRoles = store.MemberRoleByRoot(g)
			}
		}
	}

	resp := &Response{
		Task:              task,
		Technology:        req.Technology,
		Language:          req.Language,
		RootsUsed:         roots,
		KnowledgeGroup:    kgID,
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
					URI: r.URI, Title: r.Subject, Excerpt: ex, Authority: r.Authority, RootName: r.RootName,
				})
			}
			resp.Citations = append(resp.Citations, Citation{
				URI: r.URI, Title: r.Subject, Authority: r.Authority, RootName: r.RootName, Version: r.Version,
				SourceURIs: append([]string{}, r.SourceURIs...),
			})
			if r.ReviewStatus == store.ReviewHumanReviewed {
				break
			}
		}
	}

	// 2) Symbol hits from explicit identifier-like task tokens.
	taskToks := symbolTokens(task)
	if req.Debug {
		resp.DebugTaskTokens = append([]string{}, taskToks...)
	}
	apiCap := 8
	symPerTok := 5
	if budget.MaxResults > 0 && budget.MaxResults < apiCap {
		apiCap = budget.MaxResults
	}
	if budget.MaxResults > 0 && budget.MaxResults < symPerTok {
		symPerTok = budget.MaxResults
	}
	for _, tok := range taskToks {
		syms, err := st.FindSymbols(ctx, tok, roots, symPerTok)
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
			if len(resp.RequiredAPIs) >= apiCap {
				break
			}
		}
		if len(resp.RequiredAPIs) >= apiCap {
			break
		}
	}

	// 3) Budgeted FTS for examples / constraints / pitfalls / grounded workflow text.
	hits, err := searchPackageHits(ctx, st, task, roots, budget, req, resp, kgID, memberRoles)
	if err != nil {
		return nil, err
	}
	hits = librarydocs.EnrichHits(ctx, st, hits, librarydocs.DefaultRankingConfig())
	// Pull in project/official docs known via symbols when FTS preferred a weak decoy.
	hits = mergeCitedDocumentHits(ctx, st, hits, resp)
	// Prefer example/constraint/sequence sections within already-selected docs.
	hits = mergeSignalChunksFromHits(ctx, st, hits, task, resp)
	// FTS snippets are display windows — materialize bodies before extraction.
	hits = materializeHitBodies(ctx, st, hits, resp)
	pinURIs := map[string]string{}

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
		attachLibraryDocsCitation(&cit, h)
		resp.Citations = append(resp.Citations, cit)
		if ver != "" {
			versions = append(versions, ver)
		}
		if h.ArchivedHint() {
			archivedHints++
		}

		lower := strings.ToLower(h.Heading + " " + h.Snippet)
		// Independent signals — a project/official_example hit may also carry constraints.
		if len(resp.Examples) < budget.MaxExamples &&
			(exampleEligibleAuthority(h.Authority) || strings.Contains(lower, "example")) {
			resp.Examples = append(resp.Examples, ExampleRef{
				URI: h.URI, Title: h.Title, Excerpt: ex, Authority: h.Authority, RootName: h.RootName, Lines: cit.Lines,
			})
			pinURI(pinURIs, h.URI, "pinned_example_source")
			trace(resp, "extract", "example_from_hit", h.URI)
		}
		if strings.Contains(lower, "pitfall") || strings.Contains(lower, "error") || strings.Contains(lower, "fail") {
			resp.Pitfalls = appendUnique(resp.Pitfalls, store.ClipExcerpt(cleanupSnippet(h.Snippet), 180))
			pinURI(pinURIs, h.URI, "pinned_pitfall_source")
		}
		if strings.Contains(lower, "must") || strings.Contains(lower, "require") || strings.Contains(lower, "constraint") {
			resp.Constraints = appendUnique(resp.Constraints, store.ClipExcerpt(cleanupSnippet(h.Snippet), 180))
			pinURI(pinURIs, h.URI, "pinned_constraint_source")
			trace(resp, "extract", "constraint_from_hit", h.URI)
		}
		if h.Authority == store.AuthorityCurrentProject {
			resp.ProjectConventions = appendUnique(resp.ProjectConventions, store.ClipExcerpt(cleanupSnippet(h.Snippet), 160))
		}

		// Grounded sequence from ordered workflow sections in retrieved docs.
		if !sequenceGrounded && containsAny(lower, "sequence", "steps", "initialization", "init order", "call order") {
			if items := listItems(cleanupSnippet(h.Snippet)); len(items) >= 2 {
				resp.Sequence = items
				sequenceGrounded = true
				pinURI(pinURIs, h.URI, "pinned_sequence_source")
				trace(resp, "extract", "sequence_from_hit", h.URI)
			}
		}

		for _, inc := range extractIncludes(h.Snippet) {
			resp.Includes = appendUnique(resp.Includes, inc)
		}
		for _, api := range extractAPILike(h.Snippet) {
			if len(resp.RequiredAPIs) >= apiCap {
				break
			}
			resp.RequiredAPIs = appendUnique(resp.RequiredAPIs, api)
		}
	}
	if len(resp.RequiredAPIs) > apiCap {
		resp.RequiredAPIs = resp.RequiredAPIs[:apiCap]
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

	resp.Citations = selectDiverseCitations(dedupeCitations(resp.Citations), budget.MaxResults, pinURIs, resp)
	resp.RelevantSymbols = selectPackageSymbols(resp.RelevantSymbols, budget.MaxResults, resp)
	if resp.Summary != "" && (len(resp.RequiredAPIs) > 0 || len(resp.Examples) > 0) {
		resp.Summary = firstSentence(resp.Summary)
	}

	resp.Coverage = coverageOf(resp)
	resp.Freshness = freshnessFromSources(resp.Citations, versions, archivedHints, req.Version)
	resp.WebSearchRecommended = webSearchFrom(resp.Coverage, resp.Freshness)
	enrichPackageSignals(resp, req.Version, versions)

	trimToBudget(resp, budget.MaxTokensEstimate)
	// Fingerprint the final trimmed payload the client receives.
	resp.ContextFingerprint = fingerprintResponse(ctx, st, req, resp)
	chars, tokens, _ := serializeTokens(resp)
	resp.Chars = chars
	resp.EstimatedTokens = tokens

	policies := store.KnowledgeGroupPolicies{}
	if kgID != "" {
		if g, err := st.LookupKnowledgeGroup(ctx, kgID); err == nil && g != nil {
			policies = g.Policies.Normalize()
			if memberRoles == nil {
				memberRoles = store.MemberRoleByRoot(g)
			}
		}
	}
	// Attach after token estimate so diagnostics do not inflate the budget figure.
	attachRootContribution(resp, roots, memberRoles, policies)
	return resp, nil
}

// fingerprintResponse hashes the final client-visible payload (post-trim).
// Meta fields that are derived from the fingerprint/estimate loop are excluded;
// citation content hashes are included so source edits change the fingerprint.
func fingerprintResponse(ctx context.Context, st *store.Store, req Request, resp *Response) string {
	cp := *resp
	cp.ContextFingerprint = ""
	cp.EstimatedTokens = 0
	cp.Chars = 0
	cp.TokenEstimateNote = ""
	cp.SelectionTrace = nil    // assembly diagnostics must not affect fingerprint
	cp.RootContribution = nil // searched-vs-contributing metrics must not affect fingerprint
	// Stabilize ordering for fingerprint inputs.
	roots := append([]string{}, cp.RootsUsed...)
	sort.Strings(roots)
	cp.RootsUsed = roots
	apis := append([]string{}, cp.RequiredAPIs...)
	sort.Strings(apis)
	cp.RequiredAPIs = apis

	payload, err := json.Marshal(&cp)
	if err != nil {
		payload = []byte(cp.Summary)
	}
	var b strings.Builder
	b.Write(payload)
	b.WriteByte('|')
	b.WriteString(strings.TrimSpace(strings.ToLower(req.Task)))
	b.WriteByte('|')
	type citeKey struct{ uri, lines, hash string }
	var cites []citeKey
	for _, c := range resp.Citations {
		h := ""
		if st != nil {
			if got, err := st.GetHashByURI(ctx, c.URI); err == nil {
				h = got
			}
		}
		cites = append(cites, citeKey{c.URI, c.Lines, h})
	}
	sort.Slice(cites, func(i, j int) bool {
		if cites[i].uri == cites[j].uri {
			return cites[i].lines < cites[j].lines
		}
		return cites[i].uri < cites[j].uri
	})
	for _, c := range cites {
		b.WriteString(c.uri)
		b.WriteByte('#')
		b.WriteString(c.lines)
		b.WriteByte('@')
		b.WriteString(c.hash)
		b.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:16])
}

func resolveRoots(ctx context.Context, st *store.Store, req Request) (roots []string, knowledgeGroup string, roles map[string]string, err error) {
	available, err := st.ListRootNames(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	kg := strings.TrimSpace(req.KnowledgeGroup)
	rg := strings.TrimSpace(req.RootGroup)
	if kg != "" && rg != "" && !strings.EqualFold(kg, rg) {
		return nil, "", nil, fmt.Errorf("knowledgeGroup %q and rootGroup %q disagree (use one field)", kg, rg)
	}
	if kg == "" {
		kg = rg
	}

	if kg != "" {
		g, err := st.LookupKnowledgeGroup(ctx, kg)
		if err != nil {
			return nil, "", nil, err
		}
		if g == nil {
			return nil, "", nil, fmt.Errorf("knowledgeGroup %q not found (configure the group first)", kg)
		}
		filterPreferred := len(req.PreferredRoots) > 0
		expanded, err := store.ExpandKnowledgeGroup(g, available, store.ExpandOpts{
			PreferredRoots:    req.PreferredRoots,
			ProjectRoot:       req.ProjectRoot,
			FilterToPreferred: filterPreferred,
		})
		if err != nil {
			return nil, "", nil, err
		}
		return expanded, storeGroupKey(g), store.MemberRoleByRoot(g), nil
	}

	seen := map[string]struct{}{}
	var selected []string
	add := func(r string) {
		r = strings.TrimSpace(r)
		if r == "" {
			return
		}
		if _, ok := seen[r]; ok {
			return
		}
		seen[r] = struct{}{}
		selected = append(selected, r)
	}
	if req.ProjectRoot != "" {
		add(req.ProjectRoot)
	}
	for _, r := range req.PreferredRoots {
		add(r)
	}
	if len(selected) == 0 {
		return nil, "", nil, nil
	}

	inf, err := st.ValidateRootScope(ctx, selected, available)
	if err != nil {
		return nil, "", nil, err
	}
	if inf.NeedsChoice {
		return nil, "", nil, &store.ErrNeedsRoot{Inference: inf}
	}

	// Multiple roots in one knowledge group → auto-expand (single root stays narrow).
	if len(inf.Roots) >= 2 && inf.KnowledgeGroup != "" {
		g, err := st.LookupKnowledgeGroup(ctx, inf.KnowledgeGroup)
		if err != nil {
			return nil, "", nil, err
		}
		if g != nil && g.Policies.Normalize().AllowCrossRootRetrieval {
			expanded, err := store.ExpandKnowledgeGroup(g, available, store.ExpandOpts{
				PreferredRoots: req.PreferredRoots,
				ProjectRoot:    req.ProjectRoot,
			})
			if err != nil {
				return nil, "", nil, err
			}
			return expanded, storeGroupKey(g), store.MemberRoleByRoot(g), nil
		}
	}
	return inf.Roots, inf.KnowledgeGroup, nil, nil
}

func storeGroupKey(g *store.RootGroup) string {
	if g == nil {
		return ""
	}
	if strings.TrimSpace(g.ID) != "" {
		return g.ID
	}
	return g.Name
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

func attachLibraryDocsCitation(cit *Citation, h store.SearchHit) {
	if cit == nil || h.LibraryDocs == nil {
		return
	}
	ld, ok := h.LibraryDocs.(*librarydocs.HitMeta)
	if !ok || ld == nil {
		return
	}
	cit.ComponentID = ld.ComponentID
	cit.Component = ld.Component
	cit.ContentClass = ld.ContentClass
	cit.DocStatus = ld.Status
	cit.EvidenceLevel = ld.Evidence
	cit.ArtifactIDs = append([]string{}, ld.ArtifactIDs...)
	cit.RelatedSources = append([]string{}, ld.SourcePaths...)
	if cit.DocStatus == "inferred" || cit.EvidenceLevel == "E3" || cit.EvidenceLevel == "E4" {
		// Do not present E3/E4 or inferred as verified in MCP output.
		if cit.DocStatus == "verified" {
			cit.DocStatus = "inferred"
		}
	}
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

// enrichPackageSignals fills actionable missingInformation and recommendedFollowUp.
func enrichPackageSignals(resp *Response, requestedVersion string, detectedVersions []string) {
	if resp.Coverage == "low" {
		resp.MissingInformation = appendUnique(resp.MissingInformation,
			"Few grounded local hits; verify against current vendor docs if versions matter.")
		if len(resp.RelevantSymbols) == 0 && len(resp.RequiredAPIs) == 0 {
			resp.RecommendedFollowUp = appendUnique(resp.RecommendedFollowUp,
				"search_knowledge with a narrower API or error-string query")
		}
	}
	if len(resp.Examples) == 0 {
		resp.RecommendedFollowUp = appendUnique(resp.RecommendedFollowUp,
			"search_knowledge for a worked sample")
	}
	if len(resp.RelevantSymbols) == 0 && len(resp.RequiredAPIs) > 0 {
		api := resp.RequiredAPIs[0]
		resp.RecommendedFollowUp = appendUnique(resp.RecommendedFollowUp,
			"find_symbol name=\""+api+"\" for signature/lineage")
	}
	if len(resp.Sequence) == 0 && len(resp.RequiredAPIs) > 0 {
		resp.RecommendedFollowUp = appendUnique(resp.RecommendedFollowUp,
			"get_document on a citation URI that mentions initialization or call order")
	}
	req := strings.TrimSpace(requestedVersion)
	if req != "" && (resp.Freshness == "mixed" || resp.Freshness == "unknown") {
		uniq := uniqueNonEmpty(detectedVersions)
		msg := "Requested version " + req + " is not clearly satisfied by local sources"
		if len(uniq) > 0 {
			msg += " (seen: " + strings.Join(uniq, ", ") + ")"
		} else {
			msg += " (no product_version on citations)"
		}
		resp.MissingInformation = appendUnique(resp.MissingInformation, msg)
		resp.RecommendedFollowUp = appendUnique(resp.RecommendedFollowUp,
			"re-run with preferredRoots pinned to the matching versioned corpus, or refresh official docs")
	}
	if resp.Freshness == "stale" {
		resp.MissingInformation = appendUnique(resp.MissingInformation,
			"Citations include archived/obsolete material; prefer non-archived sources before coding.")
	}
	resp.RecommendedFollowUp = appendUnique(resp.RecommendedFollowUp,
		"get_document on a citation URI only if deeper context is required")
}

func uniqueNonEmpty(in []string) []string {
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

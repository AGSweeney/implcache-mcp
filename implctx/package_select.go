// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"implcache-mcp/store"
)

const (
	maxAugmentDocs       = 4
	maxAugmentChunksEach = 2
	maxHydrateSnippet    = 2400 // keep signal text; ClipExcerpt applied at extract time
	maxMaterializeHits   = 12   // avoid materializing every related-project hit in a group
	maxSelectionTrace    = 48
)

// mergeCitedDocumentHits pulls project/official docs referenced by symbols (or
// early citations) into the FTS hit list when lexical search missed them.
func mergeCitedDocumentHits(ctx context.Context, st *store.Store, hits []store.SearchHit, resp *Response) []store.SearchHit {
	if st == nil || resp == nil {
		return hits
	}
	present := map[string]struct{}{}
	for _, h := range hits {
		present[strings.ToLower(strings.TrimSpace(h.URI))] = struct{}{}
	}

	type cand struct {
		uri  string
		auth string
		rank int
	}
	var cands []cand
	seenCand := map[string]struct{}{}
	add := func(uri, auth string) {
		uri = strings.TrimSpace(uri)
		if uri == "" {
			return
		}
		key := strings.ToLower(uri)
		if _, ok := present[key]; ok {
			return
		}
		if _, ok := seenCand[key]; ok {
			return
		}
		if !authorityWorthHydrating(auth) {
			return
		}
		seenCand[key] = struct{}{}
		cands = append(cands, cand{uri: uri, auth: auth, rank: store.AuthorityRank(auth)})
	}
	for _, sym := range resp.RelevantSymbols {
		add(sym.URI, sym.Authority)
	}
	for _, c := range resp.Citations {
		add(c.URI, c.Authority)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		return cands[i].uri < cands[j].uri
	})
	if len(cands) > maxAugmentDocs {
		cands = cands[:maxAugmentDocs]
	}

	out := append([]store.SearchHit{}, hits...)
	for _, c := range cands {
		doc, chunks, err := st.GetDocumentByURI(ctx, c.uri)
		if err != nil || doc == nil || len(chunks) == 0 {
			trace(resp, "hydrate", "skip_missing_or_empty_doc", c.uri)
			continue
		}
		auth := doc.Authority
		if auth == "" {
			auth = c.auth
		}
		n := len(chunks)
		if n > maxAugmentChunksEach {
			n = maxAugmentChunksEach
		}
		for i := 0; i < n; i++ {
			ch := chunks[i]
			out = append(out, store.SearchHit{
				ChunkID: ch.ID, DocumentID: doc.ID, URI: doc.URI, Title: doc.Title,
				RootName: doc.RootName, Path: doc.Path, Authority: auth,
				Language: doc.Language, Technology: doc.Technology,
				ProductVersion: doc.ProductVersion, Deprecated: doc.Deprecated, Archived: doc.Archived,
				Ordinal: ch.Ordinal, Heading: ch.Heading, Snippet: store.ClipExcerpt(ch.Body, maxHydrateSnippet),
				StartLine: ch.StartLine, EndLine: ch.EndLine,
				MatchKind: "symbol-hydrate",
			})
		}
		present[strings.ToLower(c.uri)] = struct{}{}
		trace(resp, "hydrate", "merged_symbol_cited_doc", c.uri)
	}
	return out
}

// materializeHitBodies replaces short FTS snippets with clipped full chunk bodies
// so package extraction can see "must"/sequence/example text that BM25 snippets cut.
func materializeHitBodies(ctx context.Context, st *store.Store, hits []store.SearchHit, resp *Response) []store.SearchHit {
	if st == nil || len(hits) == 0 {
		return hits
	}
	// Prefer materializing official docs/examples before related-project noise.
	idxs := make([]int, len(hits))
	for i := range hits {
		idxs[i] = i
	}
	sort.SliceStable(idxs, func(a, b int) bool {
		ra := store.AuthorityRank(hits[idxs[a]].Authority)
		rb := store.AuthorityRank(hits[idxs[b]].Authority)
		// Invert usual rank for materialize: prefer official_example/docs (rank 3–4)
		// slightly ahead of flooding with every current_project related hit.
		score := func(auth string, rank int) int {
			switch auth {
			case store.AuthorityOfficialExample:
				return 0
			case store.AuthorityOfficialDocs:
				return 1
			case store.AuthorityCuratedRecipe:
				return 2
			case store.AuthorityCurrentProject, store.AuthorityRelatedProject:
				return 3
			default:
				return 4 + rank
			}
		}
		sa, sb := score(hits[idxs[a]].Authority, ra), score(hits[idxs[b]].Authority, rb)
		if sa != sb {
			return sa < sb
		}
		return idxs[a] < idxs[b]
	})
	if len(idxs) > maxMaterializeHits {
		idxs = idxs[:maxMaterializeHits]
	}
	for _, i := range idxs {
		if hits[i].ChunkID <= 0 {
			continue
		}
		ch, err := st.GetChunk(ctx, hits[i].ChunkID)
		if err != nil || ch == nil || strings.TrimSpace(ch.Body) == "" {
			continue
		}
		body := store.ClipExcerpt(ch.Body, maxHydrateSnippet)
		prev := hits[i].Snippet
		if body == prev {
			continue
		}
		// Prefer full body over FTS snippet windows (often missing constraint/sequence lines).
		if len(body) >= len(cleanupSnippet(prev)) || strings.Contains(prev, "<b>") || scoreSignalChunk(body, nil) > scoreSignalChunk(prev, nil) {
			hits[i].Snippet = body
			if strings.TrimSpace(hits[i].Heading) == "" {
				hits[i].Heading = ch.Heading
			}
			trace(resp, "hydrate", "materialized_chunk_body", hits[i].URI)
		}
	}
	return hits
}

// mergeSignalChunksFromHits pulls example/constraint/sequence-bearing chunks from
// documents already in the hit list. FTS MaxPerDocument often keeps thin overview
// chunks while richer API/example sections sit later in the same URI.
func mergeSignalChunksFromHits(ctx context.Context, st *store.Store, hits []store.SearchHit, task string, resp *Response) []store.SearchHit {
	if st == nil || len(hits) == 0 {
		return hits
	}
	seenChunk := map[int64]struct{}{}
	type docRef struct {
		uri  string
		auth string
	}
	var docs []docRef
	seenDoc := map[string]struct{}{}
	for _, h := range hits {
		seenChunk[h.ChunkID] = struct{}{}
		key := strings.ToLower(strings.TrimSpace(h.URI))
		if key == "" {
			continue
		}
		if _, ok := seenDoc[key]; ok {
			continue
		}
		if !authorityWorthHydrating(h.Authority) {
			continue
		}
		seenDoc[key] = struct{}{}
		docs = append(docs, docRef{uri: h.URI, auth: h.Authority})
	}
	// Prefer official corpora when hydrating inside an explicit multi-root group
	// so project-heavy hit order cannot exclude docs/examples from the hydrate set.
	sort.SliceStable(docs, func(i, j int) bool {
		ibi, ibj := isOfficialAuthority(docs[i].auth), isOfficialAuthority(docs[j].auth)
		if ibi != ibj {
			return ibi
		}
		ri, rj := store.AuthorityRank(docs[i].auth), store.AuthorityRank(docs[j].auth)
		if ri != rj {
			return ri < rj
		}
		return docs[i].uri < docs[j].uri
	})
	if len(docs) > maxAugmentDocs {
		docs = docs[:maxAugmentDocs]
	}

	taskToks := signalTaskTokens(task)
	out := append([]store.SearchHit{}, hits...)
	for _, d := range docs {
		doc, chunks, err := st.GetDocumentByURI(ctx, d.uri)
		if err != nil || doc == nil {
			continue
		}
		type scored struct {
			ch    store.Chunk
			score int
		}
		var ranked []scored
		for _, ch := range chunks {
			if _, ok := seenChunk[ch.ID]; ok {
				continue
			}
			sc := scoreSignalChunk(ch.Heading+"\n"+ch.Body, taskToks)
			if sc <= 0 {
				continue
			}
			ranked = append(ranked, scored{ch: ch, score: sc})
		}
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].score != ranked[j].score {
				return ranked[i].score > ranked[j].score
			}
			return ranked[i].ch.Ordinal < ranked[j].ch.Ordinal
		})
		n := 0
		for _, r := range ranked {
			if n >= maxAugmentChunksEach {
				break
			}
			ch := r.ch
			auth := doc.Authority
			if auth == "" {
				auth = d.auth
			}
			out = append(out, store.SearchHit{
				ChunkID: ch.ID, DocumentID: doc.ID, URI: doc.URI, Title: doc.Title,
				RootName: doc.RootName, Path: doc.Path, Authority: auth,
				Language: doc.Language, Technology: doc.Technology,
				ProductVersion: doc.ProductVersion, Deprecated: doc.Deprecated, Archived: doc.Archived,
				Ordinal: ch.Ordinal, Heading: ch.Heading, Snippet: store.ClipExcerpt(ch.Body, maxHydrateSnippet),
				StartLine: ch.StartLine, EndLine: ch.EndLine,
				MatchKind: "signal-hydrate",
			})
			seenChunk[ch.ID] = struct{}{}
			n++
		}
		if n > 0 {
			trace(resp, "hydrate", "merged_signal_chunks", d.uri)
		}
	}
	return out
}

func signalTaskTokens(task string) []string {
	var out []string
	for _, f := range strings.Fields(strings.ToLower(task)) {
		f = strings.Trim(f, ".,;:()[]{}\"'")
		if len(f) < 4 {
			continue
		}
		switch f {
		case "that", "with", "from", "this", "have", "into", "using", "create", "server":
			continue
		}
		out = append(out, f)
	}
	return out
}

func scoreSignalChunk(text string, taskToks []string) int {
	lower := strings.ToLower(text)
	score := 0
	for _, kw := range []string{"example", "must ", "must\n", "require", "constraint", "pitfall", "sequence", "steps", "initialization", "call order"} {
		if strings.Contains(lower, kw) {
			score += 3
		}
	}
	for _, tok := range taskToks {
		if strings.Contains(lower, tok) {
			score += 1
		}
	}
	return score
}

func authorityWorthHydrating(auth string) bool {
	switch auth {
	case store.AuthorityCurrentProject, store.AuthorityRelatedProject,
		store.AuthorityCuratedRecipe, store.AuthorityOfficialExample,
		store.AuthorityOfficialDocs:
		return true
	default:
		return false
	}
}

func isOfficialAuthority(auth string) bool {
	switch auth {
	case store.AuthorityOfficialExample, store.AuthorityOfficialDocs:
		return true
	default:
		return false
	}
}

// exampleEligibleAuthority reports whether a hit may fill an examples[] slot.
func exampleEligibleAuthority(auth string) bool {
	switch auth {
	case store.AuthorityCurrentProject, store.AuthorityRelatedProject,
		store.AuthorityOfficialExample, store.AuthorityCuratedRecipe:
		return true
	default:
		return false
	}
}

// searchPackageHits runs FTS for package assembly. When a knowledge group scopes
// multiple roots, each member is queried separately so a strong AND match on one
// root cannot starve sibling docs/examples roots.
func searchPackageHits(ctx context.Context, st *store.Store, task string, roots []string, budget store.ContextBudget, req Request, resp *Response, knowledgeGroup string, memberRoles map[string]string) ([]store.SearchHit, error) {
	base := store.SearchOptions{
		Query:            task,
		Limit:            budget.MaxResults * 3,
		MaxResults:       req.MaxResults,
		Roots:            roots,
		MaxPerDoc:        budget.MaxPerDocument,
		Semantic:         req.Semantic,
		PreferredVersion: req.Version,
	}
	groupActive := strings.TrimSpace(knowledgeGroup) != "" || strings.TrimSpace(req.KnowledgeGroup) != "" || strings.TrimSpace(req.RootGroup) != ""
	if !groupActive || len(roots) <= 1 {
		hits, err := st.SearchOpts(ctx, base)
		if err != nil {
			return nil, err
		}
		return applyGroupRoleScoreBias(hits, memberRoles, req.ProjectRoot), nil
	}

	per := budget.MaxResults
	if per < 3 {
		per = 3
	}
	seen := map[int64]struct{}{}
	var merged []store.SearchHit
	add := func(hits []store.SearchHit, reason, root string) {
		n := 0
		for _, h := range hits {
			if _, ok := seen[h.ChunkID]; ok {
				continue
			}
			seen[h.ChunkID] = struct{}{}
			merged = append(merged, h)
			n++
		}
		if n > 0 {
			trace(resp, "search", reason, root)
		}
	}
	// Prefer recalling official docs/examples roots first for stable assembly.
	ordered := orderRootsByMemberRole(roots, memberRoles)
	for _, root := range ordered {
		opt := base
		opt.Roots = []string{root}
		opt.Limit = per
		hits, err := st.SearchOpts(ctx, opt)
		if err != nil {
			return nil, err
		}
		add(hits, "group_root_recall", root)
	}
	combined, err := st.SearchOpts(ctx, base)
	if err != nil {
		return nil, err
	}
	kg := knowledgeGroup
	if kg == "" {
		kg = req.KnowledgeGroup
	}
	if kg == "" {
		kg = req.RootGroup
	}
	add(combined, "group_combined_recall", kg)
	merged = applyGroupRoleScoreBias(merged, memberRoles, req.ProjectRoot)
	// Rank for processing order, but do not truncate by authority — that would
	// drop official_example/docs after current_project fills the limit.
	store.RankSearchHits(merged)
	return merged, nil
}

func orderRootsByMemberRole(roots []string, roles map[string]string) []string {
	if len(roles) == 0 {
		return roots
	}
	rank := func(role string) int {
		switch role {
		case store.MemberRoleOfficialExample:
			return 0
		case store.MemberRoleOfficialDocs:
			return 1
		case store.MemberRoleCuratedKnowledge:
			return 2
		case store.MemberRoleCurrentProject:
			return 3
		case store.MemberRoleRelatedProject:
			return 4
		default:
			return 5
		}
	}
	out := append([]string{}, roots...)
	sort.SliceStable(out, func(i, j int) bool {
		return rank(roles[out[i]]) < rank(roles[out[j]])
	})
	return out
}

// applyGroupRoleScoreBias applies soft within-tier score nudges from member roles.
// Document AuthorityRank remains the primary sort key (preserveAuthorityRoles).
func applyGroupRoleScoreBias(hits []store.SearchHit, roles map[string]string, projectRoot string) []store.SearchHit {
	if len(hits) == 0 || (len(roles) == 0 && strings.TrimSpace(projectRoot) == "") {
		return hits
	}
	projectRoot = strings.TrimSpace(projectRoot)
	for i := range hits {
		role := roles[hits[i].RootName]
		switch role {
		case store.MemberRoleOfficialDocs:
			hits[i].Score += 2 // prefer for API/constraint bearing hits within tier
		case store.MemberRoleOfficialExample:
			hits[i].Score += 2
		case store.MemberRoleCuratedKnowledge:
			hits[i].Score += 1
		case store.MemberRoleCurrentProject:
			hits[i].Score += 1
		}
		if projectRoot != "" && hits[i].RootName == projectRoot {
			hits[i].Score += 4
		}
	}
	return hits
}

func authorityBucket(auth string) string {
	switch auth {
	case store.AuthorityCurrentProject, store.AuthorityRelatedProject:
		return "project"
	case store.AuthorityOfficialExample:
		return "official_example"
	case store.AuthorityOfficialDocs:
		return "official_docs"
	case store.AuthorityCuratedRecipe:
		return "recipe"
	case store.AuthorityGeneratedSummary, store.AuthorityThirdParty, "":
		return "weak"
	default:
		if store.AuthorityRank(auth) <= store.AuthorityRank(store.AuthorityOfficialDocs) {
			return "other"
		}
		return "weak"
	}
}

// selectDiverseCitations keeps a MaxResults-sized set that prefers project +
// official_example + official docs, pins signal-bearing URIs, and demotes fillers.
func selectDiverseCitations(in []Citation, maxResults int, pinURIs map[string]string, resp *Response) []Citation {
	if len(in) == 0 {
		return nil
	}
	if maxResults <= 0 {
		maxResults = store.DefaultContextBudget().MaxResults
	}

	best := map[string]Citation{}
	order := make([]string, 0, len(in))
	for _, c := range in {
		key := strings.ToLower(strings.TrimSpace(c.URI))
		if key == "" {
			continue
		}
		prev, ok := best[key]
		if !ok {
			best[key] = c
			order = append(order, key)
			continue
		}
		if citationBetter(c, prev) {
			best[key] = c
		}
	}

	type item struct {
		c   Citation
		key string
	}
	buckets := map[string][]item{}
	for _, key := range order {
		c := best[key]
		b := authorityBucket(c.Authority)
		buckets[b] = append(buckets[b], item{c: c, key: key})
	}
	// Prefer official_example ahead of official_docs within fill order.
	for _, b := range []string{"official_example", "official_docs", "project"} {
		sort.SliceStable(buckets[b], func(i, j int) bool {
			return store.AuthorityRank(buckets[b][i].c.Authority) < store.AuthorityRank(buckets[b][j].c.Authority)
		})
	}

	var out []Citation
	seen := map[string]struct{}{}
	takeKey := func(key, reason string) bool {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" || len(out) >= maxResults {
			return false
		}
		if _, ok := seen[key]; ok {
			return false
		}
		c, ok := best[key]
		if !ok {
			return false
		}
		seen[key] = struct{}{}
		out = append(out, c)
		trace(resp, "select", reason, c.URI)
		return true
	}
	take := func(b string, n int, reason string) {
		for _, it := range buckets[b] {
			if len(out) >= maxResults || n <= 0 {
				return
			}
			if takeKey(it.key, reason) {
				n--
			}
		}
	}

	// Authority diversity reserves first so signal pins from a strong project
	// root cannot starve official_example / official_docs in an explicit group.
	switch {
	case maxResults >= 3:
		take("project", 1, "reserve_project")
		take("official_example", 1, "reserve_official_example")
		take("official_docs", 1, "reserve_official_docs")
		take("recipe", 1, "reserve_recipe")
	case maxResults == 2:
		take("project", 1, "reserve_project")
		take("official_example", 1, "reserve_official_example")
		if len(out) < 2 {
			take("official_docs", 1, "reserve_official_docs")
		}
		if len(out) < 2 {
			take("recipe", 1, "reserve_recipe")
		}
	default:
		take("project", 1, "reserve_project")
		if len(out) == 0 {
			take("official_example", 1, "reserve_official_example")
		}
		if len(out) == 0 {
			take("official_docs", 1, "reserve_official_docs")
		}
		if len(out) == 0 {
			take("recipe", 1, "reserve_recipe")
		}
	}

	// Preserve sequence / constraint / example source URIs in remaining slots.
	if len(pinURIs) > 0 {
		pins := make([]string, 0, len(pinURIs))
		for u := range pinURIs {
			pins = append(pins, u)
		}
		sort.Strings(pins)
		for _, u := range pins {
			reason := pinURIs[u]
			if reason == "" {
				reason = "pinned_signal_uri"
			}
			takeKey(u, reason)
		}
	}

	for _, b := range []string{"project", "official_example", "official_docs", "recipe", "other"} {
		take(b, maxResults, "fill_"+b)
	}
	if len(out) == 0 {
		take("weak", maxResults, "fill_weak_fallback")
	}
	return out
}

func citationBetter(a, b Citation) bool {
	ra, rb := store.AuthorityRank(a.Authority), store.AuthorityRank(b.Authority)
	if ra != rb {
		return ra < rb
	}
	as, bs := strings.TrimSpace(a.Section), strings.TrimSpace(b.Section)
	if (as != "" && as != "function" && as != "api" && as != "mention") &&
		(bs == "" || bs == "function" || bs == "api" || bs == "mention") {
		return true
	}
	return false
}

// selectPackageSymbols keeps the strongest authority per symbol name and demotes
// generated/third-party fillers when a project/official definition already exists.
func selectPackageSymbols(in []store.Symbol, maxResults int, resp *Response) []store.Symbol {
	if len(in) == 0 {
		return nil
	}
	if maxResults <= 0 {
		maxResults = store.DefaultContextBudget().MaxResults
	}
	bestByName := map[string]store.Symbol{}
	order := make([]string, 0, len(in))
	for _, sym := range in {
		key := strings.ToLower(strings.TrimSpace(sym.Name))
		if key == "" {
			continue
		}
		prev, ok := bestByName[key]
		if !ok {
			bestByName[key] = sym
			order = append(order, key)
			continue
		}
		if symbolBetter(sym, prev) {
			bestByName[key] = sym
		}
	}
	var strong, weak []store.Symbol
	for _, key := range order {
		sym := bestByName[key]
		switch sym.Authority {
		case store.AuthorityGeneratedSummary, store.AuthorityThirdParty, "":
			weak = append(weak, sym)
		default:
			strong = append(strong, sym)
		}
	}
	droppedWeak := 0
	for _, sym := range in {
		switch sym.Authority {
		case store.AuthorityGeneratedSummary, store.AuthorityThirdParty, "":
			key := strings.ToLower(strings.TrimSpace(sym.Name))
			if keep, ok := bestByName[key]; ok && keep.URI != sym.URI {
				droppedWeak++
			}
		}
	}
	if droppedWeak > 0 {
		trace(resp, "select", "symbol_drop_weak_duplicates", fmt.Sprintf("count=%d", droppedWeak))
	}
	out := append([]store.Symbol{}, strong...)
	if len(out) < maxResults {
		need := maxResults - len(out)
		if need > len(weak) {
			need = len(weak)
		}
		out = append(out, weak[:need]...)
	}
	if len(out) > maxResults {
		out = out[:maxResults]
	}
	return out
}

func symbolBetter(a, b store.Symbol) bool {
	ra, rb := store.AuthorityRank(a.Authority), store.AuthorityRank(b.Authority)
	if ra != rb {
		return ra < rb
	}
	// Prefer definitions over mentions when authority ties.
	if a.Kind != "mention" && b.Kind == "mention" {
		return true
	}
	return false
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
	}
	return out
}

func trace(resp *Response, stage, reason, uri string) {
	if resp == nil {
		return
	}
	if len(resp.SelectionTrace) >= maxSelectionTrace {
		return
	}
	resp.SelectionTrace = append(resp.SelectionTrace, SelectionReason{
		Stage: stage, Reason: reason, URI: uri,
	})
}

func pinURI(pins map[string]string, uri, reason string) {
	if pins == nil {
		return
	}
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return
	}
	key := strings.ToLower(uri)
	if _, ok := pins[key]; ok {
		return
	}
	pins[key] = reason
}

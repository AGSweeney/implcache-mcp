// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package vomit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"implcache-mcp/internal/safePath"
	"implcache-mcp/store"
)

const (
	defaultDocLimit   = 8
	defaultSnippetMax = 28 // lines in a focused excerpt
	defaultSearchHits = 40
	maxCitations      = 12
	maxAPIs           = 10
	maxIncludes       = 12
	maxExcerpts       = 5
)

// Request configures a vomit run.
type Request struct {
	Subject        string
	OutPath        string
	Limit          int // max unique documents to cite (default 8)
	MaxCharsPerDoc int // kept for API compat; used as soft body scan budget
	// RootNames scopes the search. Empty = infer from Subject; if still
	// ambiguous, Generate returns *store.ErrNeedsRoot.
	RootNames []string
	// OutputRoot confines filesystem writes. Empty defaults to ./vomit-output (cwd).
	OutputRoot string
	// AllowWrite controls whether a file is written. When false, ReturnBody is forced.
	AllowWrite bool
	// ReturnBody includes Markdown in Result.Body.
	ReturnBody bool
	// MaxPlaybookBytes caps generated Markdown size (default 2 MiB).
	MaxPlaybookBytes int
	// SaveRecipe persists a generated knowledge_entry with lineage.
	SaveRecipe bool
	Technology string
	Language   string
}

// Result summarizes a vomit run.
type Result struct {
	Subject     string   `json:"subject"`
	OutPath     string   `json:"outPath,omitempty"`
	Bytes       int      `json:"bytes"`
	SourceCount int      `json:"sourceCount"`
	Sources     []string `json:"sources"`
	Roots       []string `json:"roots,omitempty"`
	Body        string   `json:"body,omitempty"`
	RecipeURI   string   `json:"recipeUri,omitempty"`
	ReviewNote  string   `json:"reviewNote,omitempty"`
}

type expandedDoc struct {
	URI      string
	Title    string
	RootName string
	Path     string
	Body     string
	Score    float64
	Kind     string // sample | guide | api | other
}

type scoredHit struct {
	hit   store.SearchHit
	score float64
}

// Generate searches the knowledge base for subject and writes a Markdown
// implementation playbook (not a raw source dump).
func Generate(ctx context.Context, st *store.Store, req Request) (*Result, error) {
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultDocLimit
	}
	if limit > 20 {
		limit = 20
	}
	scanBudget := req.MaxCharsPerDoc
	if scanBudget <= 0 {
		scanBudget = 12000
	}

	inf, err := st.ResolveRoots(ctx, subject, req.RootNames)
	if err != nil {
		return nil, err
	}
	if inf.NeedsChoice {
		return nil, &store.ErrNeedsRoot{Inference: inf}
	}

	hits, err := gatherHits(ctx, st, subject, inf.Roots)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("no knowledge hits for subject %q in roots %v", subject, inf.Roots)
	}

	docs, err := expandDocs(ctx, st, hits, subject, limit, scanBudget)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents could be loaded for subject %q", subject)
	}

	md := renderPlaybook(subject, docs, hits)
	// Annotate which roots were used.
	if len(inf.Roots) > 0 {
		md = strings.Replace(md, "(playbook mode).\n",
			fmt.Sprintf("(playbook mode; roots: %s).\n", strings.Join(inf.Roots, ", ")), 1)
	}
	maxBytes := req.MaxPlaybookBytes
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}
	if len(md) > maxBytes {
		return nil, fmt.Errorf("playbook exceeds max size (%d bytes)", maxBytes)
	}

	sources := make([]string, 0, len(docs))
	for _, d := range docs {
		sources = append(sources, d.URI)
	}
	res := &Result{
		Subject:     subject,
		Bytes:       len(md),
		SourceCount: len(docs),
		Sources:     sources,
		Roots:       inf.Roots,
	}
	if req.ReturnBody || !req.AllowWrite {
		res.Body = md
	}
	if req.AllowWrite {
		outAbs, err := resolveVomitOutPath(req.OutputRoot, req.OutPath, subject)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(outAbs), 0o755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
		if err := os.WriteFile(outAbs, []byte(md), 0o644); err != nil {
			return nil, fmt.Errorf("write markdown: %w", err)
		}
		res.OutPath = outAbs
	}
	if req.SaveRecipe {
		root := ""
		if len(inf.Roots) > 0 {
			root = inf.Roots[0]
		}
		recipeURI := "project://recipes/" + slugify(subject)
		if root != "" {
			recipeURI = "project://" + root + "/_recipes/" + slugify(subject) + ".md"
		}
		id, err := st.UpsertKnowledgeEntry(ctx, store.KnowledgeEntry{
			URI:          recipeURI,
			Subject:      subject,
			Technology:   req.Technology,
			Language:     req.Language,
			BodyMarkdown: md,
			ReviewStatus: store.ReviewGenerated,
			Authority:    store.AuthorityGeneratedSummary,
			Confidence:   "medium",
			RootName:     root,
			Hash:         fmt.Sprintf("%x", len(md)),
			SourceURIs:   sources,
		})
		if err != nil {
			return nil, fmt.Errorf("save recipe: %w", err)
		}
		res.RecipeURI = recipeURI
		res.ReviewNote = fmt.Sprintf("saved generated recipe id=%d (not human-reviewed; will not outrank curated recipes)", id)
	}
	return res, nil
}

func gatherHits(ctx context.Context, st *store.Store, subject string, roots []string) ([]store.SearchHit, error) {
	tokens := significantTokens(subject)
	type querySpec struct {
		q      string
		weight float64
	}
	queries := []querySpec{
		{subject, 8},
		{subject + " example", 5},
		{subject + " sample", 5},
	}
	for _, tok := range tokens {
		queries = append(queries, querySpec{tok, 1})
	}

	best := map[int64]scoredHit{}
	for _, qs := range queries {
		hits, err := st.SearchOpts(ctx, store.SearchOptions{
			Query: qs.q,
			Limit: defaultSearchHits,
			Roots: roots,
		})
		if err != nil {
			continue
		}
		for i, h := range hits {
			s := qs.weight*1000 - float64(i) + tokenOverlapScore(h, tokens) + uriQualityBoost(h.URI, h.Title, subject)
			if prev, ok := best[h.ChunkID]; ok && prev.score >= s {
				continue
			}
			best[h.ChunkID] = scoredHit{hit: h, score: s}
		}
	}

	all := make([]scoredHit, 0, len(best))
	for _, sh := range best {
		all = append(all, sh)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	out := make([]store.SearchHit, len(all))
	for i, sh := range all {
		out[i] = sh.hit
	}
	return out, nil
}

func tokenOverlapScore(h store.SearchHit, tokens []string) float64 {
	hay := strings.ToLower(h.Title + " " + h.URI + " " + h.Snippet + " " + h.Heading)
	var score float64
	for _, tok := range tokens {
		if strings.Contains(hay, strings.ToLower(tok)) {
			score += 25
		}
	}
	return score
}

func uriQualityBoost(uri, title, subject string) float64 {
	u := strings.ToLower(uri + " " + title)
	sub := strings.ToLower(subject)
	var score float64

	// Domain pinning: keep control-app subjects in example-control-app
	// (and away from example-device-sdk / example-plugin-sdk).
	if isControlAppSubject(sub, nil) {
		if strings.Contains(u, "example-control-app") || strings.Contains(u, "control-app") {
			score += 200
		}
		if strings.Contains(u, "example-device-sdk") || strings.Contains(u, "example-plugin-sdk") ||
			strings.Contains(u, "project://example-plugin") {
			score -= 250
		}
	}

	// Prefer samples and user-guide pages over giant API header dumps.
	if strings.Contains(u, "/samples/") || strings.Contains(u, "_c.html") || strings.Contains(u, "_cxx.html") {
		score += 80
	}
	if strings.Contains(u, "/user_guide/") {
		score += 40
	}
	if strings.Contains(u, "example_") && strings.Contains(u, "resource_file") {
		score -= 160
	}
	if strings.Contains(u, "dialog") && strings.Contains(u, "menubar") {
		score -= 100 // dialog menubar ≠ app ribbon menubar
	}
	if strings.Contains(u, "/api/") && strings.HasSuffix(u, "_h.html") {
		score -= 80
	}

	// Demote common false friends for menu-init style subjects.
	if strings.Contains(sub, "registerhandler") || strings.Contains(sub, "menubar") ||
		strings.Contains(sub, "pushbutton") || strings.Contains(sub, "addmenuitem") {
		if strings.Contains(u, "async") {
			score -= 200
		}
		if strings.Contains(u, "dialogmenubar") || strings.Contains(u, "dialogpushbutton") ||
			strings.Contains(u, "/uifc") || strings.Contains(u, "t-uifc") {
			score -= 180
		}
		if strings.Contains(u, "testinstall") {
			score -= 160
		}
		if strings.Contains(u, "mfg") || strings.Contains(u, "mold") || strings.Contains(u, "gear") || strings.Contains(u, "autoaxis") {
			score -= 140
		}
		if strings.Contains(u, "ugmain") || strings.Contains(u, "testmain") || strings.Contains(u, "samplemain") {
			score += 60
		}
		if strings.Contains(u, "core_of_a_device") || strings.Contains(u, "registerhandler_arguments") {
			score += 30
		}
		// Prefer C device-sdk samples over plugin-sdk / UI docs when subject is classic SDK.
		if (strings.Contains(sub, "device-sdk") || strings.Contains(sub, "addmenuitem") || strings.Contains(sub, "registercommand")) &&
			!strings.Contains(sub, "plugin-sdk") && !strings.Contains(sub, "plugin") {
			if strings.Contains(u, "example-plugin-sdk") || strings.Contains(u, "/plugin/") {
				score -= 80
			}
		}
	}
	return score
}

var stopTokens = map[string]struct{}{
	"a": {}, "an": {}, "the": {}, "and": {}, "or": {}, "of": {}, "to": {},
	"for": {}, "in": {}, "on": {}, "with": {}, "how": {}, "from": {},
	"example": {}, "sdk": {},
}

func significantTokens(subject string) []string {
	fields := strings.Fields(subject)
	out := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, f := range fields {
		f = strings.Trim(f, ".,;:()[]{}\"'")
		if f == "" {
			continue
		}
		lower := strings.ToLower(f)
		if _, stop := stopTokens[lower]; stop {
			continue
		}
		if len(f) < 3 {
			continue
		}
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, f)
	}
	return out
}

func expandDocs(ctx context.Context, st *store.Store, hits []store.SearchHit, subject string, limit, scanBudget int) ([]expandedDoc, error) {
	type acc struct {
		score float64
		title string
	}
	order := make([]string, 0, limit)
	best := map[string]acc{}
	tokens := significantTokens(subject)

	for i, h := range hits {
		s := float64(len(hits)-i) + tokenOverlapScore(h, tokens) + uriQualityBoost(h.URI, h.Title, subject)
		a, ok := best[h.URI]
		if !ok {
			if len(order) >= limit {
				continue
			}
			order = append(order, h.URI)
			best[h.URI] = acc{score: s, title: h.Title}
			continue
		}
		if s > a.score {
			a.score = s
			if h.Title != "" {
				a.title = h.Title
			}
			best[h.URI] = a
		}
	}

	// Re-order by score.
	sort.SliceStable(order, func(i, j int) bool {
		return best[order[i]].score > best[order[j]].score
	})

	out := make([]expandedDoc, 0, len(order))
	for _, uri := range order {
		doc, chunks, err := st.GetDocumentByURI(ctx, uri)
		if err != nil {
			continue
		}
		var b strings.Builder
		for i, c := range chunks {
			if i > 0 {
				b.WriteString("\n\n")
			}
			if strings.TrimSpace(c.Heading) != "" {
				b.WriteString(strings.TrimSpace(c.Heading))
				b.WriteString("\n\n")
			}
			b.WriteString(strings.TrimSpace(c.Body))
		}
		body := cleanupBody(b.String())
		if scanBudget > 0 && len(body) > scanBudget {
			// Prefer a subject/API-dense window over the document prefix
			// (OCR PDFs often bury the callable APIs after a long TOC/intro).
			body = truncateAroundSubject(body, tokens, scanBudget)
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		title := doc.Title
		if title == "" {
			title = best[uri].title
		}
		if title == "" {
			title = filepath.Base(doc.Path)
		}
		out = append(out, expandedDoc{
			URI:      doc.URI,
			Title:    title,
			RootName: doc.RootName,
			Path:     doc.Path,
			Body:     body,
			Score:    best[uri].score,
			Kind:     classifyDoc(doc.URI, title),
		})
	}
	return out, nil
}

func classifyDoc(uri, title string) string {
	u := strings.ToLower(uri + " " + title)
	switch {
	case strings.Contains(u, "example-control-app") || strings.Contains(u, "control-app"):
		return "guide"
	case strings.Contains(u, "/samples/") || strings.HasSuffix(u, ".c") || strings.Contains(u, "_c.html"):
		return "sample"
	case strings.Contains(u, "effs") || strings.Contains(u, "programmersguide") || strings.Contains(u, "programmer"):
		return "guide"
	case strings.Contains(u, "/user_guide/"):
		return "guide"
	case strings.Contains(u, "/api/") || strings.HasPrefix(strings.ToLower(title), "function ") || strings.HasPrefix(strings.ToLower(title), "class "):
		return "api"
	default:
		return "other"
	}
}

// truncateAroundSubject keeps ~budget bytes centered on the best subject/API line.
// It anchors on the line index (not the whole window string) so a giant OCR intro
// line cannot force the window back to byte 0 via strings.Index.
func truncateAroundSubject(body string, tokens []string, budget int) string {
	if budget <= 0 || len(body) <= budget {
		return body
	}
	lines := strings.Split(body, "\n")
	best := bestLineIndex(lines, tokens)
	offset := 0
	if best > 0 {
		for i := 0; i < best; i++ {
			offset += len(lines[i]) + 1
		}
	}
	start := offset - budget/4
	if start < 0 {
		start = 0
	}
	end := start + budget
	if end > len(body) {
		end = len(body)
		start = end - budget
		if start < 0 {
			start = 0
		}
	}
	return body[start:end]
}

func isControlAppSubject(subject string, docs []expandedDoc) bool {
	sub := strings.ToLower(subject)
	if strings.Contains(sub, "example-control-app") ||
		strings.Contains(sub, "control-app") ||
		strings.Contains(sub, "controller download") ||
		strings.Contains(sub, "download program") {
		return true
	}
	if len(docs) == 0 {
		return false
	}
	n := 0
	for _, d := range docs {
		u := strings.ToLower(d.URI + " " + d.RootName)
		if strings.Contains(u, "example-control-app") || strings.Contains(u, "control-app") {
			n++
		}
	}
	return n*2 >= len(docs)
}

func renderPlaybook(subject string, docs []expandedDoc, hits []store.SearchHit) string {
	tokens := significantTokens(subject)
	controlApp := isControlAppSubject(subject, docs)
	var apis []string
	if !controlApp {
		apis = extractAPIs(docs, hits, subject, maxAPIs)
	}
	includes := extractIncludes(docs, maxIncludes)
	excerpts := extractFocusedExcerpts(docs, tokens, hits, maxExcerpts, defaultSnippetMax)
	steps := extractWorkflowSteps(docs, tokens, 10)

	var b strings.Builder
	now := time.Now().Format(time.RFC3339)

	b.WriteString("# Implementation Playbook: ")
	b.WriteString(subject)
	b.WriteString("\n\n")
	b.WriteString("> Generated by implcache-mcp **vomit** (playbook mode).\n")
	b.WriteString("> ")
	b.WriteString(now)
	b.WriteString("\n\n")

	// 1. Goal
	b.WriteString("## 1. Goal\n\n")
	if controlApp {
		b.WriteString("Accomplish **")
		b.WriteString(subject)
		b.WriteString("** using example-control-app help topics from the local knowledge base. ")
		b.WriteString("This playbook is a workflow + citations, not a dump of full help pages.\n\n")
	} else {
		b.WriteString("Implement **")
		b.WriteString(subject)
		b.WriteString("** using patterns from the local knowledge base. ")
		b.WriteString("This playbook is a call sequence + citations, not a dump of full source files.\n\n")
	}

	// 2. Prerequisites
	b.WriteString("## 2. Prerequisites\n\n")
	if controlApp {
		b.WriteString("### Tools / context\n\n")
		b.WriteString("- example-control-app installed\n")
		b.WriteString("- Target device catalog available (e.g. demo controller)\n")
		b.WriteString("- Communication path to the controller (USB / Ethernet / serial as applicable)\n")
		b.WriteString("- Compatible controller firmware for your example-control-app release (see compatibility topics)\n\n")
		b.WriteString("### Topics pulled from the knowledge base\n\n")
		for i, d := range docs {
			if i >= 6 {
				break
			}
			b.WriteString(fmt.Sprintf("- %s — `%s`\n", d.Title, d.URI))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("### Headers / includes seen in relevant sources\n\n")
		if len(includes) == 0 {
			b.WriteString("_No `#include` lines extracted; check cited samples._\n\n")
		} else {
			b.WriteString("```c\n")
			for _, inc := range includes {
				b.WriteString(inc)
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
		b.WriteString("### Typical project pieces\n\n")
		b.WriteString("- Application or plugin entrypoint from the cited samples\n")
		b.WriteString("- Headers / imports listed above\n")
		b.WriteString("- Host registration / startup config required by the SDK\n")
		b.WriteString("- Link against the matching SDK libraries for your host\n\n")
	}

	// 3. Call sequence / workflow
	if controlApp {
		b.WriteString("## 3. Minimal workflow\n\n")
		b.WriteString("Work in this order (derived from example-control-app help procedures):\n\n")
		if len(steps) == 0 {
			b.WriteString("1. Create or open an example-control-app project and add the target controller.\n")
			b.WriteString("2. Configure the device / programs needed for the task.\n")
			b.WriteString("3. Build, connect, then download to the controller.\n\n")
		} else {
			for i, step := range steps {
				b.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("## 3. Minimal call sequence\n\n")
		b.WriteString("Work in this order (derived from high-signal APIs in the knowledge hits):\n\n")
		if len(apis) == 0 {
			b.WriteString("1. Open the top cited source and locate the setup / init path for this subject.\n")
			b.WriteString("2. Keep the documented call order (init → use → cleanup); drop unrelated demo paths.\n")
			b.WriteString("3. Match headers, link libs, and host/device config shown in the citations.\n\n")
		} else {
			for i, api := range apis {
				b.WriteString(fmt.Sprintf("%d. Call / use `%s`\n", i+1, api))
			}
			b.WriteString("\n")
		}
	}

	// 4. Focused excerpts
	b.WriteString("## 4. Focused pattern excerpts\n\n")
	b.WriteString("Short windows around the subject — not full files.\n\n")
	if len(excerpts) == 0 {
		b.WriteString("_No focused excerpts could be carved; see citations._\n\n")
	}
	for i, ex := range excerpts {
		b.WriteString(fmt.Sprintf("### Pattern %d — %s\n\n", i+1, ex.Title))
		b.WriteString(ex.Why)
		b.WriteString("\n\n")
		b.WriteString("- Source: `")
		b.WriteString(ex.URI)
		b.WriteString("`\n\n")
		lang := "c"
		if strings.Contains(strings.ToLower(ex.URI), "example-control-app") || controlApp {
			lang = ""
		} else if strings.Contains(strings.ToLower(ex.URI), "cxx") || strings.Contains(strings.ToLower(ex.Title), "c++") {
			lang = "cpp"
		}
		if ex.Kind == "guide" && !strings.Contains(ex.Body, "{") {
			lang = ""
		}
		if lang == "" {
			b.WriteString("```\n")
		} else {
			b.WriteString("```")
			b.WriteString(lang)
			b.WriteString("\n")
		}
		b.WriteString(ex.Body)
		if !strings.HasSuffix(ex.Body, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}

	// 5. Pitfalls
	b.WriteString("## 5. Common pitfalls\n\n")
	b.WriteString(buildPitfalls(subject, docs, controlApp))
	b.WriteString("\n")

	// 6. Checklist
	b.WriteString("## 6. Checklist\n\n")
	b.WriteString(buildChecklist(subject, apis, steps, controlApp))
	b.WriteString("\n")

	// 7. Citations
	b.WriteString("## 7. Citations\n\n")
	b.WriteString("| Priority | Kind | Title | Why | URI |\n")
	b.WriteString("|----------|------|-------|-----|-----|\n")
	for i, d := range docs {
		if i >= maxCitations {
			break
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | `%s` |\n",
			i+1, d.Kind, escapeTable(d.Title), escapeTable(citeWhy(d, tokens)), escapeTable(d.URI)))
	}
	b.WriteString("\n")
	b.WriteString("Pull full text with `get_document` on a URI only when you need more than the excerpt.\n")
	return b.String()
}

type excerpt struct {
	Title string
	URI   string
	Kind  string
	Why   string
	Body  string
}

func extractWorkflowSteps(docs []expandedDoc, tokens []string, limit int) []string {
	linkRe := regexp.MustCompile(`(?m)^[ \t]*[-*][ \t]+\[([^\]]+)\]\([^)]+\)`)
	numRe := regexp.MustCompile(`(?m)^[ \t]*\d+\.\s+(.+)$`)
	seen := map[string]struct{}{}
	out := make([]string, 0, limit)
	add := func(s string) {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, ".*")
		if s == "" || len(s) < 8 || len(s) > 120 {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}

	// Prefer procedure titles from cited docs that match the subject.
	for _, d := range docs {
		title := strings.TrimSpace(d.Title)
		lower := strings.ToLower(title + " " + d.Body[:min(len(d.Body), 800)])
		score := 0
		for _, tok := range tokens {
			if strings.Contains(lower, strings.ToLower(tok)) {
				score++
			}
		}
		if score > 0 || looksLikeProcedureTitle(title) {
			add(title)
		}
		if len(out) >= limit {
			return out
		}
	}

	// Pull linked procedure names / numbered steps from hub pages.
	for _, d := range docs {
		for _, m := range linkRe.FindAllStringSubmatch(d.Body, -1) {
			if looksLikeProcedureTitle(m[1]) || tokenHit(m[1], tokens) {
				add(m[1])
			}
			if len(out) >= limit {
				return out
			}
		}
		for _, m := range numRe.FindAllStringSubmatch(d.Body, -1) {
			step := strings.TrimSpace(m[1])
			// Drop markdown-link wrappers.
			if i := strings.Index(step, "]("); i > 0 && strings.HasPrefix(step, "[") {
				step = step[1:i]
			}
			if looksLikeProcedureTitle(step) || tokenHit(step, tokens) {
				add(step)
			}
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func looksLikeProcedureTitle(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	for _, p := range []string{
		"create ", "add ", "build ", "download ", "upload ", "connect ",
		"configure ", "change ", "open ", "save ", "import ", "export ",
		"monitor ", "update ", "secure ", "disconnect ", "view ", "manage ",
		"setup ", "set up ", "program ", "compile ",
	} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func tokenHit(s string, tokens []string) bool {
	lower := strings.ToLower(s)
	hits := 0
	for _, tok := range tokens {
		if strings.Contains(lower, strings.ToLower(tok)) {
			hits++
		}
	}
	return hits >= 1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	// PascalCase / qualified / member calls (DeviceSdk, OSChangePrio, Client.Connect).
	apiPascalRe = regexp.MustCompile(`\b((?:[A-Za-z_][\w]*::)?[A-Z][A-Za-z0-9]+(?:\.[A-Z][A-Za-z0-9]+)?)\s*\(`)
	// C / snake_case calls (f_enterFS, f_mountfat, f_open, mmc_initfunc as callee).
	apiSnakeRe = regexp.MustCompile(`\b([a-z][A-Za-z0-9]*(?:_[A-Za-z0-9]+)+)\s*\(`)
)

func extractAPIs(docs []expandedDoc, hits []store.SearchHit, subject string, limit int) []string {
	counts := map[string]int{}
	add := func(s string, weight int) {
		for _, re := range []*regexp.Regexp{apiPascalRe, apiSnakeRe} {
			for _, m := range re.FindAllStringSubmatch(s, -1) {
				name := m[1]
				lower := strings.ToLower(name)
				if apiNoise(lower, false) {
					continue
				}
				counts[name] += weight
			}
		}
	}
	for _, h := range hits {
		add(cleanupBody(h.Snippet), 4)
	}
	tokens := significantTokens(subject)
	subjLower := strings.ToLower(subject)
	for _, d := range docs {
		weight := 1
		if d.Kind == "sample" {
			weight = 3
		}
		window := windowAround(d.Body, tokens, 60)
		if window == "" {
			window = d.Body
			if len(window) > 4000 {
				window = window[:4000]
			}
		}
		add(window, weight)
	}
	// Boost names that appear in the subject itself (e.g. f_enterFS f_mountfat …).
	for name := range counts {
		if strings.Contains(subjLower, strings.ToLower(name)) {
			counts[name] += 6
		}
	}

	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(counts))
	for k, v := range counts {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v == list[j].v {
			return list[i].k < list[j].k
		}
		return list[i].v > list[j].v
	})

	out := make([]string, 0, limit)
	for _, item := range list {
		out = append(out, item.k)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func menuRelevantAPI(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case lower == "addmenuitem" || lower == "registercommand" || lower == "registerhandler":
		return true
	case lower == "client.connect" || lower == "spitransfer" || lower == "retrypolicy":
		return true
	default:
		return false
	}
}

func apiNoise(lower string, menuSubject bool) bool {
	switch lower {
	case "if", "for", "while", "switch", "return", "sizeof", "typeof",
		"printf", "scanf", "gets", "puts", "main", "usermain",
		"sprintf", "snprintf", "fprintf", "malloc", "free", "memcpy", "memset":
		return true
	}
	if strings.HasPrefix(lower, "dialog") || strings.HasPrefix(lower, "test") {
		return true
	}
	if menuSubject {
		if strings.Contains(lower, "async") || strings.Contains(lower, "spawnstart") {
			return true
		}
	}
	return false
}

func extractIncludes(docs []expandedDoc, limit int) []string {
	re := regexp.MustCompile(`(?m)^[ \t]*#[ \t]*include[ \t]*[<"][^>"]+[>"]`)
	skipSubstr := []string{
		"testerror", "testselect", "testparams", "testfeat", "testmenu",
		"testmisc", "testdbms", "testnotify", "testsetup", "testsect",
		"testanalysis", "testgenedata", "testextobj", "testsimprep",
		"utilmessage", "utilstring", "utilfiles", "utilmenu", "utilcollect",
		"legacynotify.h", "legacymenu.h", // legacy file-menu API; app menus use MenuBar.h
		"demoonly", "main.h",
	}
	seenBase := map[string]struct{}{}
	out := make([]string, 0, limit)
	for _, d := range docs {
		if d.Kind != "sample" && d.Kind != "guide" {
			continue
		}
		for _, m := range re.FindAllString(d.Body, -1) {
			line := strings.TrimSpace(m)
			lower := strings.ToLower(line)
			skip := false
			for _, s := range skipSubstr {
				if strings.Contains(lower, s) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			base := includeBasename(line)
			if base == "" {
				continue
			}
			key := strings.ToLower(base)
			if _, ok := seenBase[key]; ok {
				continue
			}
			seenBase[key] = struct{}{}
			// Prefer angle-bracket form for SDK headers.
			if strings.HasPrefix(key, "device") || strings.HasPrefix(key, "menu") || strings.HasPrefix(key, "plugin") {
				line = "#include <" + base + ">"
			}
			out = append(out, line)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func includeBasename(line string) string {
	i := strings.IndexAny(line, `<"`)
	if i < 0 || i+1 >= len(line) {
		return ""
	}
	j := strings.IndexAny(line[i+1:], `>"`)
	if j < 0 {
		return ""
	}
	return line[i+1 : i+1+j]
}

func extractFocusedExcerpts(docs []expandedDoc, tokens []string, hits []store.SearchHit, limit, maxLines int) []excerpt {
	out := make([]excerpt, 0, limit)
	usedURI := map[string]struct{}{}
	byURI := make(map[string]expandedDoc, len(docs))
	for _, d := range docs {
		byURI[d.URI] = d
	}

	// Prefer hit snippets, but only for documents that made the citation set.
	for _, h := range hits {
		if len(out) >= limit {
			break
		}
		d, ok := byURI[h.URI]
		if !ok {
			continue
		}
		if _, ok := usedURI[h.URI]; ok {
			continue
		}
		if d.Kind == "api" {
			continue
		}
		snip := cleanupBody(h.Snippet)
		if !relevantSnippet(snip, tokens) && !relevantSnippet(d.Body, tokens) {
			continue
		}
		window := snip
		if w := windowAround(d.Body, tokens, maxLines); w != "" {
			window = w
		}
		window = trimExcerpt(window, maxLines)
		if window == "" {
			continue
		}
		usedURI[h.URI] = struct{}{}
		out = append(out, excerpt{
			Title: d.Title,
			URI:   d.URI,
			Kind:  d.Kind,
			Why:   "**Why:** High-signal hit for the subject APIs / init pattern.",
			Body:  window,
		})
	}

	// Fill from cited docs if needed.
	for _, d := range docs {
		if len(out) >= limit {
			break
		}
		if _, ok := usedURI[d.URI]; ok {
			continue
		}
		if d.Kind == "api" && len(out) >= 2 {
			continue // don't fill playbook with header dumps
		}
		w := windowAround(d.Body, tokens, maxLines)
		if w == "" {
			continue
		}
		usedURI[d.URI] = struct{}{}
		out = append(out, excerpt{
			Title: d.Title,
			URI:   d.URI,
			Kind:  d.Kind,
			Why:   "**Why:** " + citeWhy(d, tokens),
			Body:  trimExcerpt(w, maxLines),
		})
	}
	return out
}

func relevantSnippet(s string, tokens []string) bool {
	lower := strings.ToLower(s)
	hits := 0
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(tok)) {
			hits++
		}
	}
	return hits >= 1
}

func scoreLine(line string, tokens []string) int {
	lower := strings.ToLower(line)
	score := 0
	for _, tok := range tokens {
		if tok != "" && strings.Contains(lower, strings.ToLower(tok)) {
			score += 2
		}
	}
	// Prefer API-dense lines over OCR intro prose that only repeats the subject token.
	if apiPascalRe.MatchString(line) || apiSnakeRe.MatchString(line) {
		score += 6
	}
	if strings.Contains(lower, "f_") && strings.Contains(line, "(") {
		score += 4
	}
	if strings.Contains(lower, "int f_") || strings.Contains(lower, "void f_") ||
		strings.Contains(lower, "long f_") || strings.Contains(lower, "f_file *") {
		score += 5
	}
	// Giant OCR blobs that only echo the subject should lose to short API lines.
	if len(line) > 400 && score > 0 && !(apiPascalRe.MatchString(line) || apiSnakeRe.MatchString(line)) {
		score = 1
	}
	return score
}

func bestLineIndex(lines []string, tokens []string) int {
	best := -1
	bestScore := 0
	for i, line := range lines {
		score := scoreLine(line, tokens)
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	if bestScore == 0 {
		return -1
	}
	return best
}

func windowAround(body string, tokens []string, maxLines int) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		return ""
	}
	best := bestLineIndex(lines, tokens)
	if best < 0 {
		return ""
	}
	start := best - maxLines/3
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(lines) {
		end = len(lines)
		start = end - maxLines
		if start < 0 {
			start = 0
		}
	}
	// Drop a leading giant OCR line when the focus is later in the window.
	if start < best && len(lines[start]) > 400 && !(apiPascalRe.MatchString(lines[start]) || apiSnakeRe.MatchString(lines[start])) {
		start = best
		if end < start+1 {
			end = min(len(lines), start+maxLines)
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func trimExcerpt(s string, maxLines int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Drop leading copyright-only noise.
	lines := strings.Split(s, "\n")
	for len(lines) > 0 {
		t := strings.TrimSpace(lines[0])
		if t == "" || strings.Contains(t, "Copyright") || t == "*/" || t == "/*" || strings.HasPrefix(t, "```") {
			lines = lines[1:]
			continue
		}
		break
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func citeWhy(d expandedDoc, tokens []string) string {
	switch d.Kind {
	case "sample":
		return "working sample pattern"
	case "guide":
		return "user-guide explanation"
	case "api":
		return "API reference"
	default:
		if len(tokens) > 0 {
			return "mentions " + tokens[0]
		}
		return "related hit"
	}
}

func buildPitfalls(subject string, docs []expandedDoc, controlApp bool) string {
	var b strings.Builder
	sub := strings.ToLower(subject)
	if controlApp {
		b.WriteString("- Confirm example-control-app release ↔ controller firmware compatibility before download.\n")
		b.WriteString("- Build the project before download; unresolved faults block a clean transfer.\n")
		b.WriteString("- Connection path / driver (USB vs Ethernet) must match the physical link.\n")
		b.WriteString("- Password-protected controllers need credentials before upload/download.\n")
		b.WriteString("- Don’t confuse **download to controller** with **upload from controller** (direction matters).\n")
		return b.String()
	}
	b.WriteString("- Prefer header/API pages for signatures when sample bodies are truncated or OCR-noisy.\n")
	b.WriteString("- Keep init/cleanup order from cited workflow sections; do not invent call sequences.\n")
	b.WriteString("- Match the cited knowledge root/SDK — do not apply patterns from unrelated products.\n")
	rootHint := playbookDomain(docs)
	switch rootHint {
	case "netburner":
		b.WriteString("- Each task that uses EFFS typically needs its own `f_enterFS` / `f_releaseFS` pair.\n")
		b.WriteString("- Mount before open/read/write; unmount with `f_delvolume` when the guide says so.\n")
		if strings.Contains(sub, "sd") || strings.Contains(sub, "mmc") || strings.Contains(sub, "effs") {
			b.WriteString("- SD/MMC EFFS often assumes exclusive QSPI use — check platform notes before sharing the bus.\n")
		}
	case "creo":
		b.WriteString("- Don’t confuse menubar registration APIs with custom dialog UI menus.\n")
		b.WriteString("- Sync DLL apps use the toolkit entrypoint; async spawn demos are a different path.\n")
	}
	if strings.Contains(sub, "plugin") {
		b.WriteString("- Plugin hosts often require session/command registration beyond a single UI helper.\n")
	}
	for _, d := range docs {
		u := strings.ToLower(d.URI)
		if strings.Contains(u, "async") {
			b.WriteString("- Cited async samples are for connectivity mode — skip them for a normal application DLL.\n")
			break
		}
	}
	return b.String()
}

func playbookDomain(docs []expandedDoc) string {
	for _, d := range docs {
		r := strings.ToLower(d.RootName + " " + d.URI + " " + d.Title)
		switch {
		case strings.Contains(r, "netburner") || strings.Contains(r, "effs") || strings.Contains(r, "nburn"):
			return "netburner"
		case strings.Contains(r, "creo") || strings.Contains(r, "protoolkit") || strings.Contains(r, "otk"):
			return "creo"
		case strings.Contains(r, "example-control-app") || strings.Contains(r, "control-app"):
			return "control-app"
		}
	}
	return ""
}

func buildChecklist(subject string, apis, steps []string, controlApp bool) string {
	var b strings.Builder
	if controlApp {
		b.WriteString("- [ ] Confirm example-control-app + device/firmware stack for **")
		b.WriteString(subject)
		b.WriteString("**\n")
		b.WriteString("- [ ] Create/open project and add the correct controller type\n")
		b.WriteString("- [ ] Configure programs/variables needed for the task\n")
		b.WriteString("- [ ] Build with no blocking errors\n")
		b.WriteString("- [ ] Connect using the right path, then download\n")
		b.WriteString("- [ ] Verify online / monitor expected tags or I/O\n")
		for i, step := range steps {
			if i >= 6 {
				break
			}
			b.WriteString("- [ ] ")
			b.WriteString(step)
			b.WriteString("\n")
		}
		return b.String()
	}
	b.WriteString("- [ ] Confirm the correct SDK / knowledge root for **")
	b.WriteString(subject)
	b.WriteString("**\n")
	b.WriteString("- [ ] Add required headers / link libs from citations\n")
	b.WriteString("- [ ] Implement the entrypoint and call order cited in the sources\n")
	b.WriteString("- [ ] Apply any host/device/startup config shown in the samples\n")
	b.WriteString("- [ ] Smoke-test on real hardware or a real host session\n")
	for i, api := range apis {
		if i >= 6 {
			break
		}
		b.WriteString("- [ ] Use `")
		b.WriteString(api)
		b.WriteString("` correctly (args / error check)\n")
	}
	return b.String()
}

var (
	realHTMLTagRe    = regexp.MustCompile(`(?i)</?[a-z][a-z0-9]*\b[^>]*>`)
	includeAngleRe   = regexp.MustCompile(`(?m)^([ \t]*#[ \t]*include[ \t]*)<([^>\r\n]+)>`)
	bareIncludeRunRe = regexp.MustCompile(`(?m)(?:^[ \t]*#[ \t]*include[ \t]*\r?\n){2,}`)
	mdImageRe        = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	multiBlank       = regexp.MustCompile(`\n{3,}`)
)

func cleanupBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")

	type slot struct{ token, value string }
	var slots []slot
	s = includeAngleRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := includeAngleRe.FindStringSubmatch(m)
		if len(sub) != 3 {
			return m
		}
		tok := fmt.Sprintf("@@INCLUDE_%d@@", len(slots))
		slots = append(slots, slot{tok, sub[1] + "<" + sub[2] + ">"})
		return tok
	})

	s = realHTMLTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = mdImageRe.ReplaceAllString(s, "")

	for _, sl := range slots {
		s = strings.ReplaceAll(s, sl.token, sl.value)
	}

	s = bareIncludeRunRe.ReplaceAllString(s, "/* #include … */\n")
	s = multiBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func escapeTable(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func resolveVomitOutPath(outputRoot, outPath, subject string) (string, error) {
	root := strings.TrimSpace(outputRoot)
	if root == "" {
		root = "vomit-output"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel := strings.TrimSpace(outPath)
	if rel == "" {
		rel = slugify(subject) + ".md"
	}
	// Strip a leading "vomit/" for callers still using the old default shape.
	rel = strings.TrimPrefix(rel, "vomit/")
	rel = strings.TrimPrefix(rel, `vomit\`)
	full, err := safePath.ResolveUnderRoot(absRoot, rel)
	if err != nil {
		return "", fmt.Errorf("output path: %w", err)
	}
	if _, err := safePath.EvalAndContain(absRoot, full); err != nil {
		return "", fmt.Errorf("output path: %w", err)
	}
	return full, nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "subject"
	}
	if len(out) > 80 {
		out = out[:80]
		out = strings.Trim(out, "-")
	}
	return out
}

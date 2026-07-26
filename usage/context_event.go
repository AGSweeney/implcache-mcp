// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package usage

import (
	"strings"
	"time"

	"implcache-mcp/implctx"
	"implcache-mcp/store"
)

// FromImplementationContext builds a RequestEvent from an implctx package.
func FromImplementationContext(tool, task string, resp *implctx.Response, latency time.Duration) RequestEvent {
	ev := RequestEvent{
		RequestID:             NewRequestID(),
		OccurredAt:            time.Now().UTC(),
		ToolName:              tool,
		TaskHash:              HashTask(task),
		LatencyMS:             int(latency.Milliseconds()),
		RequestClass:          ClassifyTool(tool),
		TokenEstimatorVersion: TokenEstimatorVersion,
	}
	covApp := CoverageApplicableForTool(tool)
	ev.CoverageApplicable = &covApp

	if resp == nil {
		ev.ResultStatus = StatusNoLocalMatch
		if !covApp {
			ev.Coverage = CoverageNotApplicable
		}
		return ev
	}
	ev.Coverage = normalizeCoverage(resp.Coverage, covApp)
	ev.Freshness = resp.Freshness
	ev.EstimatedTokens = resp.EstimatedTokens
	ev.ReturnedTokens = resp.EstimatedTokens
	ev.ContextFingerprint = resp.ContextFingerprint
	ev.AdditionalRetrievalRecommended = resp.WebSearchRecommended
	ev.CitationCount = len(resp.Citations)
	ev.SymbolCount = len(resp.RelevantSymbols)
	ev.Roots = rootsFromNames(resp.RootsUsed)
	ev.RootCount = len(ev.Roots)

	curated, recipe := 0, 0
	structuredShare, rawShare := 0, 0
	nCite := len(resp.Citations)
	nSym := len(resp.RelevantSymbols)
	pieces := nCite + nSym
	if pieces == 0 {
		pieces = 1
	}
	perPiece := 0
	if resp.EstimatedTokens > 0 {
		perPiece = resp.EstimatedTokens / pieces
		if perPiece < 1 {
			perPiece = 1
		}
	}

	for i, c := range resp.Citations {
		et, key := classifyCitation(c)
		tok := perPiece
		if et == EvidenceCurated || et == EvidenceRecipe {
			curated++
			structuredShare += tok
		} else if et == EvidenceDocument {
			rawShare += tok
		} else {
			// Generic citation = raw document contribution.
			rawShare += tok
			et = EvidenceCitation
		}
		if et == EvidenceRecipe {
			recipe++
		}
		ev.Evidence = append(ev.Evidence, EvidenceEvent{
			EvidenceType:          et,
			EvidenceKey:           key,
			RootKey:               c.RootName,
			SourceURI:             c.URI,
			Authority:             c.Authority,
			RankPosition:          i + 1,
			SelectedForPackage:    true,
			IncludedAfterTrimming: true,
			EstimatedTokens:       tok,
		})
	}
	for i, sym := range resp.RelevantSymbols {
		key := SymbolKey(sym.NameNorm, sym.RootName)
		tok := perPiece
		structuredShare += tok
		ev.Evidence = append(ev.Evidence, EvidenceEvent{
			EvidenceType:          EvidenceSymbol,
			EvidenceKey:           key,
			RootKey:               sym.RootName,
			Authority:             "",
			RankPosition:          i + 1,
			SelectedForPackage:    true,
			IncludedAfterTrimming: true,
			EstimatedTokens:       tok,
		})
	}
	if resp.RecipeReviewStatus != "" {
		recipe++
		if curated == 0 {
			curated++
		}
	}
	ev.CuratedCount = curated
	ev.RecipeCount = recipe
	ev.SourceCount = ev.CitationCount
	ev.ResultStatus = statusFromCounts(ev)

	// Normalize structured/raw to sum to returned when possible.
	ev.StructuredTokens = structuredShare
	ev.RawDocumentTokens = rawShare
	if ev.ReturnedTokens > 0 && ev.StructuredTokens+ev.RawDocumentTokens == 0 {
		if curated+recipe+ev.SymbolCount > 0 && ev.CitationCount == 0 {
			ev.StructuredTokens = ev.ReturnedTokens
		} else if curated+recipe+ev.SymbolCount == 0 {
			ev.RawDocumentTokens = ev.ReturnedTokens
		} else {
			ev.StructuredTokens = ev.ReturnedTokens / 2
			ev.RawDocumentTokens = ev.ReturnedTokens - ev.StructuredTokens
		}
	} else if total := ev.StructuredTokens + ev.RawDocumentTokens; total > 0 && ev.ReturnedTokens > 0 && total != ev.ReturnedTokens {
		// Scale shares to returned package size.
		ev.StructuredTokens = ev.ReturnedTokens * ev.StructuredTokens / total
		ev.RawDocumentTokens = ev.ReturnedTokens - ev.StructuredTokens
	}

	applySourceEstimate(&ev)
	return ev
}

// RootSelectionEvent records a needsChoice / ErrNeedsRoot outcome.
func RootSelectionEvent(tool, task string, roots []string, latency time.Duration) RequestEvent {
	covApp := CoverageApplicableForTool(tool)
	ev := RequestEvent{
		RequestID:             NewRequestID(),
		OccurredAt:            time.Now().UTC(),
		ToolName:              tool,
		TaskHash:              HashTask(task),
		LatencyMS:             int(latency.Milliseconds()),
		ResultStatus:          StatusRootSelectionRequired,
		RootSelectionRequired: true,
		Roots:                 rootsFromNames(roots),
		RequestClass:          ClassifyTool(tool),
		TokenEstimatorVersion: TokenEstimatorVersion,
		CoverageApplicable:    &covApp,
	}
	if !covApp {
		ev.Coverage = CoverageNotApplicable
	}
	ev.RootCount = len(ev.Roots)
	return ev
}

// ErrorEvent records a failed retrieval request.
func ErrorEvent(tool, task, category, message string, latency time.Duration) RequestEvent {
	covApp := CoverageApplicableForTool(tool)
	ev := RequestEvent{
		RequestID:             NewRequestID(),
		OccurredAt:            time.Now().UTC(),
		ToolName:              tool,
		TaskHash:              HashTask(task),
		LatencyMS:             int(latency.Milliseconds()),
		ResultStatus:          StatusRequestError,
		ErrorCategory:         category,
		ErrorMessage:          truncate(message, 240),
		RequestClass:          ClassifyTool(tool),
		TokenEstimatorVersion: TokenEstimatorVersion,
		CoverageApplicable:    &covApp,
	}
	if !covApp {
		ev.Coverage = CoverageNotApplicable
	}
	return ev
}

// NoMatchEvent records a completed request with no local package.
func NoMatchEvent(tool, task string, latency time.Duration) RequestEvent {
	covApp := CoverageApplicableForTool(tool)
	ev := RequestEvent{
		RequestID:             NewRequestID(),
		OccurredAt:            time.Now().UTC(),
		ToolName:              tool,
		TaskHash:              HashTask(task),
		LatencyMS:             int(latency.Milliseconds()),
		ResultStatus:          StatusNoLocalMatch,
		RequestClass:          ClassifyTool(tool),
		TokenEstimatorVersion: TokenEstimatorVersion,
		CoverageApplicable:    &covApp,
	}
	if !covApp {
		ev.Coverage = CoverageNotApplicable
	}
	return ev
}

// ClassifyTool maps a tool name to a request_class.
func ClassifyTool(tool string) string {
	switch strings.TrimSpace(tool) {
	case "get_implementation_context", "implementation_context":
		return ClassImplementationContext
	case "search_knowledge", "search":
		return ClassKnowledgeSearch
	case "search_symbols", "get_symbol", "symbol_search":
		return ClassSymbolSearch
	case "get_document", "get_document_chunk", "document_fetch":
		return ClassDocumentFetch
	case "resolve_roots", "list_roots":
		return ClassRootResolution
	case "report_outcome", "report_implementation_outcome":
		return ClassOutcomeReport
	default:
		return ClassOther
	}
}

// CoverageApplicableForTool reports whether coverage rating is expected.
func CoverageApplicableForTool(tool string) bool {
	switch ClassifyTool(tool) {
	case ClassImplementationContext:
		return true
	case ClassKnowledgeSearch, ClassSymbolSearch:
		// May produce packages; treat as applicable so missing coverage is unclassified.
		return true
	case ClassDocumentFetch, ClassRootResolution, ClassOutcomeReport:
		return false
	default:
		return false
	}
}

// SymbolKey builds a stable symbol analytics key.
func SymbolKey(nameNorm, root string) string {
	return strings.TrimSpace(nameNorm) + "|" + strings.TrimSpace(root)
}

func classifyCitation(c implctx.Citation) (evidenceType, key string) {
	uri := strings.TrimSpace(c.URI)
	auth := strings.TrimSpace(c.Authority)
	if auth == store.AuthorityCuratedRecipe || strings.Contains(uri, "/_recipes/") {
		return EvidenceRecipe, uri
	}
	if auth == store.AuthorityGeneratedSummary || strings.Contains(uri, "curated") {
		return EvidenceCurated, uri
	}
	return EvidenceCitation, uri
}

func statusFromCounts(ev RequestEvent) string {
	if ev.CitationCount == 0 && ev.SymbolCount == 0 && ev.RecipeCount == 0 {
		if ev.Coverage == "low" || ev.Coverage == "" {
			return StatusNoLocalMatch
		}
		return StatusLocalInsufficient
	}
	hasCurated := ev.CuratedCount > 0 || ev.RecipeCount > 0
	hasRaw := ev.CitationCount > ev.CuratedCount || (ev.CitationCount > 0 && ev.CuratedCount == 0)
	if hasCurated && hasRaw && ev.CitationCount > ev.RecipeCount {
		return StatusGroundedMixed
	}
	if hasCurated {
		return StatusGroundedCurated
	}
	return StatusGroundedLocal
}

func normalizeCoverage(cov string, applicable bool) string {
	cov = strings.ToLower(strings.TrimSpace(cov))
	if !applicable {
		return CoverageNotApplicable
	}
	switch cov {
	case CoverageHigh, CoverageMedium, CoverageLow:
		return cov
	case CoverageNotApplicable:
		return CoverageUnclassified
	case "":
		return CoverageUnclassified
	default:
		return CoverageUnclassified
	}
}

// applySourceEstimate fills source/avoided/reduction from returned tokens.
// Without full-source sizes, estimate source as ~4× returned for mixed packages
// and ~2× for structured-only; avoided = max(source-returned, 0).
func applySourceEstimate(ev *RequestEvent) {
	if ev.ReturnedTokens <= 0 {
		return
	}
	mult := 3
	if ev.RawDocumentTokens > ev.StructuredTokens && ev.StructuredTokens == 0 {
		mult = 2
	} else if ev.StructuredTokens > 0 && ev.RawDocumentTokens == 0 {
		mult = 4
	}
	ev.EstimatedSource = ev.ReturnedTokens * mult
	ev.TokensAvoided = ev.EstimatedSource - ev.ReturnedTokens
	if ev.TokensAvoided < 0 {
		ev.TokensAvoided = 0
	}
	if ev.EstimatedSource > 0 {
		pct := float64(ev.TokensAvoided) / float64(ev.EstimatedSource) * 100
		ev.ReductionPct = &pct
	}
}

func rootsFromNames(names []string) []RootRef {
	out := make([]RootRef, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, RootRef{RootKey: n, RootName: n, Selected: true})
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

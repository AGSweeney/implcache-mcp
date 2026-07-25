// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"context"
	"path/filepath"
	"strings"

	"implcache-mcp/ingest"
	"implcache-mcp/store"
)

// harvestSymbolsFromHits recovers APIs for natural-language tasks that lack
// identifier cues by inspecting retrieved documents' stored symbols and
// high-signal tokens from headings/snippets.
func harvestSymbolsFromHits(ctx context.Context, st *store.Store, roots []string, hits []store.SearchHit, task string, limit int) []store.Symbol {
	if limit <= 0 {
		limit = 8
	}
	seenDoc := map[int64]struct{}{}
	var docIDs []int64
	var cueTokens []string
	for _, h := range hits {
		if _, ok := seenDoc[h.DocumentID]; !ok {
			seenDoc[h.DocumentID] = struct{}{}
			docIDs = append(docIDs, h.DocumentID)
		}
		for _, tok := range symbolTokens(h.Heading + " " + cleanupSnippet(h.Snippet)) {
			cueTokens = appendUnique(cueTokens, tok)
		}
		base := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(h.Path, `\`, `/`)), filepath.Ext(h.Path))
		if ingest.LooksLikeAPIToken(base) {
			cueTokens = appendUnique(cueTokens, base)
		}
	}
	for _, tok := range strings.Fields(task) {
		tok = strings.Trim(tok, ".,;:()[]{}\"'")
		if len(tok) >= 5 && ingest.LooksLikeAPIToken(tok) {
			cueTokens = appendUnique(cueTokens, tok)
		}
		// Also try capitalized content words as soft cues (Reconnect -> may match RetryPolicy via FTS docs).
		if len(tok) >= 6 {
			cueTokens = appendUnique(cueTokens, tok)
		}
	}

	var out []store.Symbol
	seenSym := map[int64]struct{}{}
	add := func(syms []store.Symbol) {
		for _, sym := range syms {
			if _, ok := seenSym[sym.ID]; ok {
				continue
			}
			// Prefer definitions over call-site noise for NL harvest.
			if sym.Kind == "call" && len(out) > 0 {
				continue
			}
			seenSym[sym.ID] = struct{}{}
			out = append(out, sym)
			if len(out) >= limit {
				return
			}
		}
	}

	if syms, err := st.ListSymbolsByDocumentIDs(ctx, docIDs, limit*3); err == nil {
		add(syms)
	}
	if len(out) >= limit {
		return out[:limit]
	}
	for _, tok := range cueTokens {
		if len(out) >= limit {
			break
		}
		syms, err := st.FindSymbols(ctx, tok, roots, 3)
		if err != nil {
			continue
		}
		add(syms)
	}
	// Last resort: content keywords from the query against root-scoped symbols.
	if len(out) == 0 {
		for _, tok := range cueTokens {
			if len(out) >= limit {
				break
			}
			if len(tok) < 4 {
				continue
			}
			syms, err := st.FindSymbols(ctx, tok, roots, 5)
			if err != nil {
				continue
			}
			add(syms)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

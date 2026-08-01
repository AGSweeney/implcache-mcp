// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package implctx

import (
	"encoding/json"

	"implcache-mcp/store"
)

// serializeTokens marshals the response and estimates tokens from the actual payload.
// EstimatedTokens/Chars are zeroed before marshal so the estimate is not self-referential.
func serializeTokens(resp *Response) (chars, tokens int, payload []byte) {
	cp := *resp
	cp.EstimatedTokens = 0
	cp.Chars = 0
	cp.SelectionTrace = nil    // diagnostics excluded from budget
	cp.RootContribution = nil // searched-vs-contributing metrics excluded from budget
	b, err := json.Marshal(&cp)
	if err != nil {
		s := resp.Summary
		return len([]rune(s)), store.EstimateTokens(s), nil
	}
	chars = len([]rune(string(b)))
	tokens = store.EstimateTokens(string(b))
	return chars, tokens, b
}

// trimToBudget shrinks optional fields until the serialized payload is within budget.
// Citations for retained claims are never removed wholesale; trimming prefers examples/symbols.
func trimToBudget(resp *Response, maxTokens int) {
	if maxTokens <= 0 {
		return
	}
	// Bulk-cap API/symbol vomit before the iterative trim — one-at-a-time
	// cannot drain hundreds of harvested names within the iteration budget.
	const maxAPIsSoft = 8
	const maxSymbolsSoft = 6
	if len(resp.RequiredAPIs) > maxAPIsSoft {
		resp.RequiredAPIs = resp.RequiredAPIs[:maxAPIsSoft]
	}
	if len(resp.RelevantSymbols) > maxSymbolsSoft {
		resp.RelevantSymbols = resp.RelevantSymbols[:maxSymbolsSoft]
	}
	for i := 0; i < 40; i++ {
		_, tokens, _ := serializeTokens(resp)
		if tokens <= maxTokens {
			resp.EstimatedTokens = tokens
			return
		}
		switch {
		case len(resp.RecommendedFollowUp) > 0:
			resp.RecommendedFollowUp = resp.RecommendedFollowUp[:len(resp.RecommendedFollowUp)-1]
		case len(resp.ProjectConventions) > 0:
			resp.ProjectConventions = resp.ProjectConventions[:len(resp.ProjectConventions)-1]
		case len(resp.RequiredAPIs) > 4:
			resp.RequiredAPIs = resp.RequiredAPIs[:len(resp.RequiredAPIs)-1]
		case len(resp.RelevantSymbols) > 2:
			resp.RelevantSymbols = resp.RelevantSymbols[:len(resp.RelevantSymbols)-1]
		case len(resp.Citations) > 3:
			// Drop weakest trailing citations before destroying package sufficiency fields.
			resp.Citations = resp.Citations[:len(resp.Citations)-1]
		case len(resp.Examples) > 1:
			resp.Examples = resp.Examples[:len(resp.Examples)-1]
		case len(resp.Pitfalls) > 0:
			resp.Pitfalls = resp.Pitfalls[:len(resp.Pitfalls)-1]
		case len(resp.Constraints) > 1:
			resp.Constraints = resp.Constraints[:len(resp.Constraints)-1]
		case len(resp.Prerequisites) > 0:
			resp.Prerequisites = resp.Prerequisites[:len(resp.Prerequisites)-1]
		case len(resp.Examples) == 1 && len(resp.Examples[0].Excerpt) > 80:
			resp.Examples[0].Excerpt = store.ClipExcerpt(resp.Examples[0].Excerpt, len(resp.Examples[0].Excerpt)/2)
		case len(resp.Constraints) == 1 && len(resp.Constraints[0]) > 80:
			resp.Constraints[0] = store.ClipExcerpt(resp.Constraints[0], len(resp.Constraints[0])/2)
		case len(resp.RequiredAPIs) > 2:
			resp.RequiredAPIs = resp.RequiredAPIs[:len(resp.RequiredAPIs)-1]
		case len(resp.RelevantSymbols) > 1:
			resp.RelevantSymbols = resp.RelevantSymbols[:len(resp.RelevantSymbols)-1]
		case len(resp.MissingInformation) > 0:
			resp.MissingInformation = resp.MissingInformation[:len(resp.MissingInformation)-1]
		case len(resp.Summary) > 60:
			r := []rune(firstSentence(resp.Summary))
			if len(r) > 60 {
				r = r[:60]
			}
			resp.Summary = string(r) + "…"
		case resp.TokenEstimateNote != "":
			resp.TokenEstimateNote = ""
		// Only as a last resort drop the final example/constraint (keep sequence).
		case len(resp.Examples) == 1:
			resp.Examples = nil
		case len(resp.Constraints) == 1:
			resp.Constraints = nil
		default:
			resp.EstimatedTokens = tokens
			return
		}
	}
	_, tokens, _ := serializeTokens(resp)
	resp.EstimatedTokens = tokens
}

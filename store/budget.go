// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import "unicode/utf8"

// ContextBudget limits how much text an implementation-context response may carry.
type ContextBudget struct {
	MaxResults        int // primary hits / sections
	MaxExamples       int
	MaxExcerptChars   int
	MaxTotalChars     int
	MaxTokensEstimate int // soft budget; ~4 chars/token
	MaxPerDocument    int
}

// DefaultContextBudget is sized for compact agent packages (~1.5–3k tokens).
func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxResults:        5,
		MaxExamples:       2,
		MaxExcerptChars:   600,
		MaxTotalChars:     10000,
		MaxTokensEstimate: 2500,
		MaxPerDocument:    2,
	}
}

// EstimateTokens is a rough chars/4 estimate (labeled as estimate in API responses).
func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// ClipExcerpt trims an excerpt to maxChars on a rune boundary.
func ClipExcerpt(s string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	runes := []rune(s)
	if len(runes) > maxChars {
		runes = runes[:maxChars]
	}
	out := string(runes)
	if i := len(out) - 1; i > 40 {
		// Prefer ending near whitespace
		for j := i; j > i-40 && j >= 0; j-- {
			if out[j] == ' ' || out[j] == '\n' {
				return out[:j] + "…"
			}
		}
	}
	return out + "…"
}

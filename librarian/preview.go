// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"implcache-mcp/store"
)

// PreviewOptions configures a bounded document preview.
type PreviewOptions struct {
	URI         string
	ID          int64
	MaxChunks   int
	MaxChars    int
	IncludeBody bool
}

// PreviewDocument returns a truncated document/chunk view for the GUI.
func PreviewDocument(ctx context.Context, st *store.Store, opt PreviewOptions) (*PreviewResult, error) {
	if opt.MaxChunks <= 0 {
		opt.MaxChunks = 3
	}
	if opt.MaxChunks > 20 {
		opt.MaxChunks = 20
	}
	if opt.MaxChars <= 0 {
		opt.MaxChars = 2000
	}
	if opt.MaxChars > 20000 {
		opt.MaxChars = 20000
	}

	var (
		doc    *store.Document
		chunks []store.Chunk
		err    error
	)
	switch {
	case strings.TrimSpace(opt.URI) != "":
		doc, chunks, err = st.GetDocumentByURI(ctx, opt.URI)
	case opt.ID > 0:
		doc, chunks, err = st.GetDocumentByID(ctx, opt.ID)
	default:
		return nil, fmt.Errorf("uri or id is required")
	}
	if err != nil {
		return nil, err
	}

	total := len(chunks)
	truncated := false
	if len(chunks) > opt.MaxChunks {
		chunks = chunks[:opt.MaxChunks]
		truncated = true
	}
	for i := range chunks {
		if utf8.RuneCountInString(chunks[i].Body) > opt.MaxChars {
			chunks[i].Body = truncateRunes(chunks[i].Body, opt.MaxChars)
			truncated = true
		}
	}
	res := &PreviewResult{
		Document: *doc, Chunks: chunks, Truncated: truncated, TotalChunks: total,
	}
	if opt.IncludeBody {
		var b strings.Builder
		for i, c := range chunks {
			if i > 0 {
				b.WriteString("\n\n")
			}
			if c.Heading != "" {
				b.WriteString("# ")
				b.WriteString(c.Heading)
				b.WriteString("\n\n")
			}
			b.WriteString(c.Body)
		}
		res.Body = b.String()
	}
	return res, nil
}

// SearchPlaygroundOptions configures the admin search playground.
type SearchPlaygroundOptions struct {
	Query    string
	Roots    []string
	RootName string
	Limit    int
	Semantic bool
	Explain  bool
	// AllRoots opts into cross-root search. Default false: resolve or needsChoice.
	AllRoots bool
}

// SearchPlayground runs a retrieval query with optional EXPLAIN QUERY PLAN.
func SearchPlayground(ctx context.Context, st *store.Store, opt SearchPlaygroundOptions) (*SearchPlaygroundResult, error) {
	q := strings.TrimSpace(opt.Query)
	if q == "" {
		return nil, fmt.Errorf("query is required")
	}
	roots := append([]string{}, opt.Roots...)
	if rn := strings.TrimSpace(opt.RootName); rn != "" {
		roots = append(roots, rn)
	}
	if opt.Limit <= 0 {
		opt.Limit = 10
	}

	if !opt.AllRoots {
		if len(roots) == 0 {
			inf, err := st.ResolveRoots(ctx, q, nil)
			if err != nil {
				return nil, err
			}
			if inf.NeedsChoice {
				return nil, &store.ErrNeedsRoot{Inference: inf}
			}
			roots = inf.Roots
		} else {
			available, err := st.ListRootNames(ctx)
			if err != nil {
				return nil, err
			}
			inf := store.ValidateRootScope(roots, available)
			if inf.NeedsChoice {
				return nil, &store.ErrNeedsRoot{Inference: inf}
			}
			roots = inf.Roots
		}
	}

	hits, err := st.SearchOpts(ctx, store.SearchOptions{
		Query: q, Roots: roots, Limit: opt.Limit, Semantic: opt.Semantic,
	})
	if err != nil {
		return nil, err
	}
	if opt.Explain {
		store.AttachScoreBreakdown(hits, q)
	}
	res := &SearchPlaygroundResult{Query: q, Roots: roots, Hits: hits, Count: len(hits)}
	if opt.Explain {
		plan, err := st.ExplainSearchPlan(ctx, q, roots)
		if err == nil {
			res.Explain = plan
		}
	}
	return res, nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	b.WriteString("…")
	return b.String()
}

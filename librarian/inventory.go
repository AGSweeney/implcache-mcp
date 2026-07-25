// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarian

import (
	"context"
	"fmt"
	"strings"

	"implcache-mcp/store"
)

// ListSources returns a unified inventory across web, pdf, repo, and local roots.
func ListSources(ctx context.Context, st *store.Store) ([]SourceSummary, error) {
	ownedRoots := map[string]struct{}{}
	var out []SourceSummary

	webs, err := st.ListWebSources(ctx)
	if err != nil {
		return nil, err
	}
	for _, ws := range webs {
		ownedRoots[ws.RootName] = struct{}{}
		counts, _ := st.CountByRoot(ctx, ws.RootName)
		out = append(out, SourceSummary{
			SourceRef: SourceRef{Kind: KindWeb, ID: ws.Name, RootName: ws.RootName, Title: ws.StartURL},
			Enabled:   ws.Enabled, LastStatus: ws.LastStatus,
			LastAttemptAt: ws.LastAttemptAt, LastSuccessAt: ws.LastSuccessAt,
			DocumentCount: counts.Documents, ChunkCount: counts.Chunks, SymbolCount: counts.Symbols,
			Detail: map[string]any{
				"profile": ws.Profile, "startUrl": ws.StartURL,
				"product": ws.Product, "declaredVersion": ws.DeclaredVersion,
				"detectedVersion": ws.DetectedVersion,
			},
		})
	}

	pdfs, err := st.ListPDFSources(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range pdfs {
		ownedRoots[p.RootName] = struct{}{}
		counts, _ := st.CountByRoot(ctx, p.RootName)
		// PDF roots may share documents with other PDFs; count this URI specifically when possible.
		out = append(out, SourceSummary{
			SourceRef:     SourceRef{Kind: KindPDF, ID: p.DocumentURI, RootName: p.RootName, Title: firstNonEmpty(p.Title, p.FileName)},
			LastStatus:    p.ExtractionStatus,
			LastAttemptAt: p.UpdatedAt, LastSuccessAt: p.UpdatedAt,
			DocumentCount: counts.Documents, ChunkCount: counts.Chunks, SymbolCount: counts.Symbols,
			Detail: map[string]any{
				"sourcePath": p.SourcePath, "pageCount": p.PageCount,
				"version": p.Version, "product": p.Product, "fileHash": p.FileHash,
			},
		})
	}

	repos, err := st.ListRepoSources(ctx)
	if err != nil {
		return nil, err
	}
	for _, rs := range repos {
		ownedRoots[rs.RootName] = struct{}{}
		counts, _ := st.CountByRoot(ctx, rs.RootName)
		out = append(out, SourceSummary{
			SourceRef: SourceRef{Kind: KindRepo, ID: rs.Name, RootName: rs.RootName, Title: firstNonEmpty(rs.RemoteURL, rs.LocalPath)},
			Enabled:   rs.Enabled, LastStatus: rs.LastStatus,
			LastAttemptAt: rs.LastAttemptAt, LastSuccessAt: rs.LastSuccessAt,
			DocumentCount: counts.Documents, ChunkCount: counts.Chunks, SymbolCount: counts.Symbols,
			Detail: map[string]any{
				"acquisitionMode": rs.AcquisitionMode, "requestedRef": rs.RequestedRef,
				"resolvedCommit": rs.ResolvedCommitSHA, "remoteUrl": rs.RemoteURL,
			},
		})
	}

	roots, err := st.ListRootNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, root := range roots {
		if _, owned := ownedRoots[root]; owned {
			continue
		}
		counts, _ := st.CountByRoot(ctx, root)
		out = append(out, SourceSummary{
			SourceRef:     SourceRef{Kind: KindLocal, ID: root, RootName: root, Title: root},
			DocumentCount: counts.Documents, ChunkCount: counts.Chunks, SymbolCount: counts.Symbols,
			Detail: map[string]any{"synthesized": true},
		})
	}
	return out, nil
}

// GetSource returns one source by kind+id.
func GetSource(ctx context.Context, st *store.Store, kind SourceKind, id string) (*SourceSummary, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	list, err := ListSources(ctx, st)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Kind == kind && list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("source %s/%s not found", kind, id)
}

// GetSourceHealth builds a health snapshot for one source.
func GetSourceHealth(ctx context.Context, st *store.Store, kind SourceKind, id string) (*SourceHealth, error) {
	sum, err := GetSource(ctx, st, kind, id)
	if err != nil {
		return nil, err
	}
	h := &SourceHealth{
		SourceRef:     sum.SourceRef,
		LastStatus:    sum.LastStatus,
		DocumentCount: sum.DocumentCount,
		ChunkCount:    sum.ChunkCount,
		SymbolCount:   sum.SymbolCount,
		State:         classifyState(sum.LastStatus),
	}
	errs, err := RecentErrors(ctx, st, kind, id, 10)
	if err == nil {
		h.RecentErrors = errs
		h.ErrorCount = len(errs)
		if h.ErrorCount > 0 && h.State == "ok" {
			h.State = "degraded"
		}
	}
	return h, nil
}

// RecentErrors returns recent error strings for a source.
func RecentErrors(ctx context.Context, st *store.Store, kind SourceKind, id string, limit int) ([]string, error) {
	switch kind {
	case KindWeb:
		ws, err := st.GetWebSourceByName(ctx, id)
		if err != nil {
			return nil, err
		}
		pages, err := st.ListWebPageErrors(ctx, ws.ID, limit)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(pages))
		for _, p := range pages {
			out = append(out, fmt.Sprintf("%s: %s", p.SourceURL, p.LastError))
		}
		if ws.LastStatus != "" && (strings.HasPrefix(ws.LastStatus, "failed") || strings.HasPrefix(ws.LastStatus, "partial")) {
			out = append([]string{ws.LastStatus}, out...)
		}
		return out, nil
	case KindRepo:
		rs, err := st.GetRepoSourceByName(ctx, id)
		if err != nil {
			return nil, err
		}
		if rs.LastStatus != "" && strings.HasPrefix(rs.LastStatus, "failed") {
			return []string{rs.LastStatus}, nil
		}
		return nil, nil
	case KindPDF:
		p, err := st.GetPDFSourceByURI(ctx, id)
		if err != nil {
			return nil, err
		}
		if p.ExtractionStatus != "" && p.ExtractionStatus != "text" && p.ExtractionStatus != "ok" {
			return []string{p.ExtractionStatus}, nil
		}
		return nil, nil
	case KindLocal:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown kind %q", kind)
	}
}

func classifyState(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch {
	case s == "" || s == "idle":
		return "idle"
	case s == "running":
		return "running"
	case s == "ok" || s == "ingested" || s == "text" || s == "unchanged":
		return "ok"
	case strings.HasPrefix(s, "partial"):
		return "degraded"
	case strings.HasPrefix(s, "failed") || s == "corrupt" || s == "encrypted" || s == "image-only":
		return "failed"
	default:
		return "unknown"
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/internal/netsafe"
	"implcache-mcp/store"
)

// CrawlDefaults from the mirroring PRD.
const (
	DefaultMaxPages      = 5000
	DefaultMaxDepth      = 16
	DefaultMaxCrawlBytes = 1 << 30
	DefaultCrawlDelay    = 100 * time.Millisecond
)

// ProgressFunc receives crawl progress updates (optional).
type ProgressFunc func(done, total int, bytes int64, currentURL, message string)

// CrawlOptions configures a site crawl / refresh.
type CrawlOptions struct {
	SourceName        string
	MaxPages          int
	MaxDepth          int
	MaxCrawlBytes     int64
	MaxResponseBytes  int64
	CrawlDelay        time.Duration
	AllowInsecureHTTP bool
	ExtraAllowedHosts map[string]struct{}
	RefreshOnly       bool // use conditional requests when possible
	Progress          ProgressFunc
}

// CrawlReport summarizes a crawl.
type CrawlReport struct {
	SourceName    string    `json:"sourceName"`
	RootName      string    `json:"rootName"`
	Generation    int64     `json:"generation"`
	StartedAt     time.Time `json:"startedAt"`
	FinishedAt    time.Time `json:"finishedAt"`
	DurationMS    int64     `json:"durationMs"`
	New           int       `json:"new"`
	Changed       int       `json:"changed"`
	Unchanged     int       `json:"unchanged"`
	MissingMarked int       `json:"missingMarked"`
	Failed        int       `json:"failed"`
	Skipped       int       `json:"skipped"`
	Bytes         int64     `json:"bytesDownloaded"`
	LimitReached  string    `json:"limitReached,omitempty"`
	FatalError    string    `json:"fatalError,omitempty"`
	PageErrors    []string  `json:"pageErrors,omitempty"`
	OpID          string    `json:"opId,omitempty"` // set by admin tool progress tracking
}

var reHref = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)

// CrawlSite runs a bounded crawl for a registered web source.
func CrawlSite(ctx context.Context, st *store.Store, opt CrawlOptions) (*CrawlReport, error) {
	ws, err := st.GetWebSourceByName(ctx, opt.SourceName)
	if err != nil {
		return nil, fmt.Errorf("web source: %w", err)
	}
	if opt.MaxPages <= 0 {
		opt.MaxPages = DefaultMaxPages
	}
	if opt.MaxDepth <= 0 {
		opt.MaxDepth = DefaultMaxDepth
	}
	if opt.MaxCrawlBytes <= 0 {
		opt.MaxCrawlBytes = DefaultMaxCrawlBytes
	}
	if opt.MaxResponseBytes <= 0 {
		opt.MaxResponseBytes = DefaultMaxBytes
	}
	if opt.CrawlDelay <= 0 {
		opt.CrawlDelay = DefaultCrawlDelay
	}

	gen, err := st.NextCrawlGeneration(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	_ = st.SetWebSourceStatus(ctx, ws.ID, "running", false)

	rep := &CrawlReport{
		SourceName: ws.Name,
		RootName:   ws.RootName,
		Generation: gen,
		StartedAt:  time.Now().UTC(),
	}

	type item struct {
		url   string
		depth int
	}
	queue := []item{{ws.StartURL, 0}}
	seen := map[string]struct{}{ws.StartURL: {}}
	existing, _ := st.ListWebPages(ctx, ws.ID)
	byURL := map[string]store.WebPage{}
	for _, p := range existing {
		byURL[p.SourceURL] = p
	}

	safeHosts := opt.ExtraAllowedHosts
	allowHTTP := opt.AllowInsecureHTTP || strings.HasPrefix(strings.ToLower(ws.StartURL), "http://")
	baseFetch := FetchOptions{
		AllowInsecureHTTP: allowHTTP,
		MaxBytes:          opt.MaxResponseBytes,
		ExtraAllowedHosts: safeHosts,
	}
	robots := loadRobots(ctx, ws.StartURL, baseFetch)
	for _, loc := range discoverSitemapURLs(ctx, ws.StartURL, ws.AllowedPrefixes, baseFetch) {
		if _, ok := seen[loc]; ok {
			continue
		}
		if !robots.allowed(loc) {
			continue
		}
		seen[loc] = struct{}{}
		queue = append(queue, item{loc, 0})
	}

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			rep.FatalError = err.Error()
			break
		}
		if rep.New+rep.Changed+rep.Unchanged+rep.Failed+rep.Skipped >= opt.MaxPages {
			rep.LimitReached = "maxPages"
			break
		}
		if rep.Bytes >= opt.MaxCrawlBytes {
			rep.LimitReached = "maxCrawlBytes"
			break
		}

		cur := queue[0]
		queue = queue[1:]
		if opt.Progress != nil {
			done := rep.New + rep.Changed + rep.Unchanged + rep.Failed + rep.Skipped
			opt.Progress(done, opt.MaxPages, rep.Bytes, cur.url, "fetch")
		}
		if cur.depth > opt.MaxDepth {
			rep.Skipped++
			continue
		}
		if !netsafe.PrefixAllowed(cur.url, ws.AllowedPrefixes) {
			rep.Skipped++
			continue
		}
		if skipByProfile(ws.Profile, RelativePathFromURL(cur.url)) {
			rep.Skipped++
			continue
		}
		if !robots.allowed(cur.url) {
			rep.Skipped++
			continue
		}

		time.Sleep(opt.CrawlDelay)

		fetchOpt := baseFetch
		prev, hasPrev := byURL[cur.url]
		if opt.RefreshOnly && hasPrev {
			fetchOpt.ETag = prev.ETag
			fetchOpt.LastModified = prev.LastModified
		}

		page, err := FetchURL(ctx, cur.url, fetchOpt)
		now := time.Now().Unix()
		if err != nil {
			rep.Failed++
			if len(rep.PageErrors) < 50 {
				rep.PageErrors = append(rep.PageErrors, fmt.Sprintf("%s: %v", cur.url, err))
			}
			wp := store.WebPage{
				WebSourceID: ws.ID, SourceURL: cur.url, LastError: err.Error(),
				HTTPStatus: 0, LastSeenGeneration: gen, CrawlGeneration: gen, CrawlDepth: cur.depth,
				VerifiedAt: now,
			}
			if hasPrev {
				wp.DocumentID = prev.DocumentID
				wp.ContentHash = prev.ContentHash
				wp.ETag = prev.ETag
				wp.LastModified = prev.LastModified
				wp.MissingCount = prev.MissingCount
			}
			_, _ = st.UpsertWebPage(ctx, wp)
			continue
		}
		if page.NotModified {
			rep.Unchanged++
			prev.LastSeenGeneration = gen
			prev.VerifiedAt = now
			prev.HTTPStatus = 304
			prev.LastError = ""
			_, _ = st.UpsertWebPage(ctx, prev)
			continue
		}
		rep.Bytes += int64(len(page.Body))

		title, md, err := CleanHTML(page.ContentType, page.Body, ws.Profile)
		if err != nil || strings.TrimSpace(md) == "" {
			rep.Failed++
			if len(rep.PageErrors) < 50 {
				msg := "empty content"
				if err != nil {
					msg = err.Error()
				}
				rep.PageErrors = append(rep.PageErrors, fmt.Sprintf("%s: %s", cur.url, msg))
			}
			continue
		}
		rawTitle := ""
		if m := reTitle.FindStringSubmatch(string(page.Body)); len(m) == 2 {
			rawTitle = m[1]
		}
		ver := DetectDocVersion(rawTitle, ws.Profile)
		if ver == "" {
			ver = DetectDocVersion(title, ws.Profile)
		}
		if ver != "" && ws.DetectedVersion == "" {
			ws.DetectedVersion = ver
			_ = st.SetWebSourceDetectedVersion(ctx, ws.ID, ver)
		}
		rel := RelativePathFromURL(page.CanonicalURL)
		hash := sha256Hex(page.Body)
		uri := ingest.ProjectURI(ws.RootName, rel)
		outcome := "new"
		if hasPrev {
			if prev.ContentHash == hash {
				outcome = "unchanged"
			} else {
				outcome = "changed"
			}
		}
		var docID int64
		if outcome == "unchanged" {
			rep.Unchanged++
			docID = prev.DocumentID
		} else {
			chunks := ingest.ChunkMarkdown(md)
			if title == "" {
				title = rel
			}
			_, err := st.UpsertDocument(ctx, store.UpsertInput{
				URI: uri, Title: title, SourceType: store.SourceWeb, Path: page.CanonicalURL,
				RootName: ws.RootName, Authority: ws.Authority, Technology: ws.Product,
				Language: ws.Language, ProductVersion: firstNonEmpty(ws.DetectedVersion, ws.DeclaredVersion, ws.Target),
				Hash: hash, Mtime: now, Chunks: chunks,
			})
			if err != nil {
				rep.Failed++
				rep.PageErrors = append(rep.PageErrors, fmt.Sprintf("%s: upsert: %v", cur.url, err))
				continue
			}
			if d, _, err := st.GetDocumentByURI(ctx, uri); err == nil {
				docID = d.ID
			}
			if outcome == "new" {
				rep.New++
			} else {
				rep.Changed++
			}
		}
		_, _ = st.UpsertWebPage(ctx, store.WebPage{
			WebSourceID: ws.ID, DocumentID: docID, SourceURL: cur.url, CanonicalURL: page.CanonicalURL,
			RelativePath: rel, PageTitle: title, ETag: page.ETag, LastModified: page.LastModified,
			ContentHash: hash, HTTPStatus: page.StatusCode, ContentType: page.ContentType,
			ContentLength: int64(len(page.Body)), FetchedAt: now, VerifiedAt: now,
			CrawlGeneration: gen, CrawlDepth: cur.depth, LastSeenGeneration: gen, MissingCount: 0,
		})

		if cur.depth < opt.MaxDepth {
			for _, link := range extractLinks(page.CanonicalURL, string(page.Body)) {
				if !netsafe.PrefixAllowed(link, ws.AllowedPrefixes) {
					continue
				}
				if _, ok := seen[link]; ok {
					continue
				}
				seen[link] = struct{}{}
				queue = append(queue, item{link, cur.depth + 1})
			}
		}
	}

	if rep.FatalError == "" && rep.LimitReached == "" {
		n, err := st.MarkMissingWebPages(ctx, ws.ID, gen)
		if err == nil {
			rep.MissingMarked = int(n)
		}
		_ = st.SetWebSourceStatus(ctx, ws.ID, "ok", true)
	} else {
		status := "failed"
		if rep.LimitReached != "" {
			status = "partial:" + rep.LimitReached
		}
		_ = st.SetWebSourceStatus(ctx, ws.ID, status, false)
	}
	rep.FinishedAt = time.Now().UTC()
	rep.DurationMS = rep.FinishedAt.Sub(rep.StartedAt).Milliseconds()
	return rep, nil
}

func skipByProfile(profile, rel string) bool {
	switch strings.ToLower(profile) {
	case ProfileSphinx:
		return SkipSphinxPath(rel)
	case ProfileDoxygen:
		return SkipDoxygenPath(rel)
	default:
		return false
	}
}

func extractLinks(baseURL, htmlStr string) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range reHref.FindAllStringSubmatch(htmlStr, -1) {
		if len(m) < 2 {
			continue
		}
		ref := strings.TrimSpace(m[1])
		if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(strings.ToLower(ref), "javascript:") {
			continue
		}
		if skipCrawlHref(ref) {
			continue
		}
		ru, err := url.Parse(ref)
		if err != nil {
			continue
		}
		abs := base.ResolveReference(ru)
		if abs.Scheme != "http" && abs.Scheme != "https" {
			continue
		}
		abs.Fragment = ""
		abs.RawQuery = "" // drop cache-busting ?v=… on static assets / permalinks
		if skipCrawlPath(abs.Path) {
			continue
		}
		out = append(out, abs.String())
	}
	return out
}

func skipCrawlHref(ref string) bool {
	lower := strings.ToLower(strings.TrimSpace(ref))
	if strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") {
		return true
	}
	// Scheme-less hostnames (e.g. "www.netburner.com") resolve into the docs tree.
	if !strings.Contains(lower, "://") && !strings.HasPrefix(lower, "/") && !strings.HasPrefix(lower, "./") &&
		!strings.HasPrefix(lower, "../") && looksLikeHostname(lower) {
		return true
	}
	return false
}

func looksLikeHostname(s string) bool {
	if s == "" || strings.ContainsAny(s, `/\?#`) {
		return false
	}
	if strings.HasSuffix(s, ".html") || strings.HasSuffix(s, ".htm") {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func skipCrawlPath(p string) bool {
	p = strings.ToLower(p)
	base := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i+1:]
	}
	switch {
	case strings.HasSuffix(p, ".css"), strings.HasSuffix(p, ".js"),
		strings.HasSuffix(p, ".png"), strings.HasSuffix(p, ".jpg"),
		strings.HasSuffix(p, ".jpeg"), strings.HasSuffix(p, ".gif"),
		strings.HasSuffix(p, ".svg"), strings.HasSuffix(p, ".ico"),
		strings.HasSuffix(p, ".woff"), strings.HasSuffix(p, ".woff2"),
		strings.HasSuffix(p, ".ttf"), strings.HasSuffix(p, ".map"),
		strings.HasSuffix(p, ".pdf"), strings.HasSuffix(p, ".zip"),
		strings.HasSuffix(p, ".php"):
		return true
	case base == "doxygen_crawl.html", strings.HasPrefix(base, "search_"):
		return true
	case strings.Contains(p, "/_static/"), strings.Contains(p, "/_images/"):
		return true
	case looksLikeHostname(base):
		return true
	default:
		return false
	}
}

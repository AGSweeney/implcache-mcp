// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"implcache-mcp/internal/netsafe"
)

const (
	DefaultTimeout       = 30 * time.Second
	DefaultMaxBytes      = 5 << 20
	DefaultRedirectLimit = 5
	userAgent            = "implcache-mcp-web/1.0"
)

// FetchOptions configures a single HTTP fetch.
type FetchOptions struct {
	AllowInsecureHTTP bool
	Timeout           time.Duration
	MaxBytes          int64
	RedirectLimit     int
	ExtraAllowedHosts map[string]struct{}
	// Conditional request headers
	ETag         string
	LastModified string
}

// FetchedPage is a validated HTTP response body.
type FetchedPage struct {
	URL          string
	CanonicalURL string
	StatusCode   int
	ContentType  string
	Body         []byte
	ETag         string
	LastModified string
	NotModified  bool
}

// FetchURL retrieves a URL with SSRF controls and size limits.
func FetchURL(ctx context.Context, raw string, opt FetchOptions) (*FetchedPage, error) {
	if opt.Timeout <= 0 {
		opt.Timeout = DefaultTimeout
	}
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = DefaultMaxBytes
	}
	if opt.RedirectLimit <= 0 {
		opt.RedirectLimit = DefaultRedirectLimit
	}
	safeOpt := netsafe.Options{
		AllowInsecureHTTP: opt.AllowInsecureHTTP,
		ExtraAllowedHosts: opt.ExtraAllowedHosts,
	}
	u, err := netsafe.ValidateURL(raw, safeOpt)
	if err != nil {
		return nil, err
	}
	if err := netsafe.ValidateResolvedHost(u.Hostname(), safeOpt); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: opt.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opt.RedirectLimit {
				return fmt.Errorf("stopped after %d redirects", opt.RedirectLimit)
			}
			if _, err := netsafe.ValidateURL(req.URL.String(), safeOpt); err != nil {
				return err
			}
			if err := netsafe.ValidateResolvedHost(req.URL.Hostname(), safeOpt); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/markdown,text/plain;q=0.9,*/*;q=0.1")
	if opt.ETag != "" {
		req.Header.Set("If-None-Match", opt.ETag)
	}
	if opt.LastModified != "" {
		req.Header.Set("If-Modified-Since", opt.LastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()
	if _, err := netsafe.ValidateURL(finalURL, safeOpt); err != nil {
		return nil, fmt.Errorf("final url: %w", err)
	}
	if err := netsafe.ValidateResolvedHost(resp.Request.URL.Hostname(), safeOpt); err != nil {
		return nil, fmt.Errorf("final host: %w", err)
	}

	page := &FetchedPage{
		URL:          raw,
		CanonicalURL: finalURL,
		StatusCode:   resp.StatusCode,
		ContentType:  resp.Header.Get("Content-Type"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	if resp.StatusCode == http.StatusNotModified {
		page.NotModified = true
		return page, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d for %s", resp.StatusCode, finalURL)
	}
	if !allowedContentType(page.ContentType) {
		return nil, fmt.Errorf("unsupported content-type %q", page.ContentType)
	}
	limited := io.LimitReader(resp.Body, opt.MaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > opt.MaxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", opt.MaxBytes)
	}
	page.Body = body
	return page, nil
}

func allowedContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch ct {
	case "text/html", "application/xhtml+xml", "text/markdown", "text/plain", "":
		return true
	default:
		return false
	}
}

// RelativePathFromURL derives a stable relative path for project:// URIs.
func RelativePathFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "index.html"
	}
	p := strings.TrimPrefix(u.EscapedPath(), "/")
	if p == "" || strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/") + "/index.html"
		p = strings.TrimPrefix(p, "/")
	}
	if p == "" {
		return "index.html"
	}
	return p
}

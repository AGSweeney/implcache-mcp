// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"encoding/xml"
	"net/url"
	"strings"

	"implcache-mcp/internal/netsafe"
)

type urlSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// discoverSitemapURLs loads /sitemap.xml from the start origin and returns
// same-host URLs that match allowed prefixes (best-effort; failures are ignored).
func discoverSitemapURLs(ctx context.Context, startURL string, prefixes []string, opt FetchOptions) []string {
	u, err := url.Parse(startURL)
	if err != nil || u.Host == "" {
		return nil
	}
	sitemapURL := u.Scheme + "://" + u.Host + "/sitemap.xml"
	page, err := FetchURL(ctx, sitemapURL, opt)
	if err != nil || page.NotModified || len(page.Body) == 0 {
		return nil
	}
	var set urlSet
	if err := xml.Unmarshal(page.Body, &set); err != nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	var out []string
	for _, e := range set.URLs {
		loc := strings.TrimSpace(e.Loc)
		if loc == "" {
			continue
		}
		lu, err := url.Parse(loc)
		if err != nil {
			continue
		}
		if strings.ToLower(lu.Hostname()) != host {
			continue
		}
		if !netsafe.PrefixAllowed(loc, prefixes) {
			continue
		}
		out = append(out, loc)
	}
	return out
}

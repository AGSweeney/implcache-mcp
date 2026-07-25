// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bufio"
	"context"
	"net/url"
	"strings"
)

// robotsRules is a best-effort Disallow set for our user-agent (and *).
type robotsRules struct {
	disallow []string
	loaded   bool
}

func (r *robotsRules) allowed(raw string) bool {
	if r == nil || !r.loaded || len(r.disallow) == 0 {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	for _, d := range r.disallow {
		if d == "" {
			continue
		}
		if d == "/" {
			return false
		}
		if strings.HasPrefix(path, d) {
			return false
		}
	}
	return true
}

// loadRobots fetches robots.txt from the start URL's origin (same-host only).
func loadRobots(ctx context.Context, startURL string, opt FetchOptions) *robotsRules {
	u, err := url.Parse(startURL)
	if err != nil || u.Host == "" {
		return &robotsRules{}
	}
	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"
	page, err := FetchURL(ctx, robotsURL, opt)
	if err != nil || page.NotModified || len(page.Body) == 0 {
		return &robotsRules{loaded: true}
	}
	return parseRobots(string(page.Body), userAgent)
}

type robotsGroup struct {
	agents   []string
	disallow []string
}

func parseRobots(body, ua string) *robotsRules {
	ua = strings.ToLower(ua)
	sc := bufio.NewScanner(strings.NewReader(body))
	var groups []robotsGroup
	var cur *robotsGroup

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "user-agent":
			agent := strings.ToLower(val)
			// New group if current already has disallow rules, else accumulate agents.
			if cur == nil || len(cur.disallow) > 0 {
				groups = append(groups, robotsGroup{})
				cur = &groups[len(groups)-1]
			}
			cur.agents = append(cur.agents, agent)
		case "disallow":
			if cur == nil || val == "" {
				continue
			}
			cur.disallow = append(cur.disallow, val)
		}
	}

	var specific, star []string
	for _, g := range groups {
		matchUA, matchStar := false, false
		for _, a := range g.agents {
			if a == "*" {
				matchStar = true
			}
			if a != "*" && (strings.Contains(ua, a) || strings.Contains(a, "implcache")) {
				matchUA = true
			}
		}
		if matchUA {
			specific = append(specific, g.disallow...)
		} else if matchStar {
			star = append(star, g.disallow...)
		}
	}
	disallow := star
	if len(specific) > 0 {
		disallow = specific
	}
	return &robotsRules{disallow: disallow, loaded: true}
}

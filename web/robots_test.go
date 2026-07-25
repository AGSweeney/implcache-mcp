// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import "testing"

func TestParseRobotsPrefersSpecificAgent(t *testing.T) {
	body := `User-agent: *
Disallow: /private/

User-agent: implcache-mcp-web
Disallow: /admin/
`
	r := parseRobots(body, userAgent)
	// Specific agent group replaces *; /private/ is only in the * group.
	if !r.allowed("https://ex.test/private/x") {
		t.Fatal("specific agent rules replace *; /private/ should be allowed")
	}
	if r.allowed("https://ex.test/admin/x") {
		t.Fatal("specific disallow should block /admin/")
	}
	if !r.allowed("https://ex.test/docs/ok") {
		t.Fatal("docs should be allowed")
	}
}

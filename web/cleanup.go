// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"regexp"
	"strings"

	"implcache-mcp/ingest"
)

// Profile names for extraction.
const (
	ProfileGeneric = "generic"
	ProfileSphinx  = "sphinx"
	ProfileDoxygen = "doxygen"
)

var (
	reTitle = regexp.MustCompile(`(?i)<title[^>]*>([^<]*)</title>`)
	// Drop common documentation chrome that survives main-content extraction.
	// Go's RE2 engine has no backreferences, so each tag is listed explicitly.
	reChromeBlocks = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<nav\b[^>]*>.*?</nav>`),
		regexp.MustCompile(`(?is)<header\b[^>]*>.*?</header>`),
		regexp.MustCompile(`(?is)<footer\b[^>]*>.*?</footer>`),
		regexp.MustCompile(`(?is)<aside\b[^>]*>.*?</aside>`),
		regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`),
		regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`),
	}
	reChromeDivs = regexp.MustCompile(`(?is)<div[^>]+class=["'][^"']*\b(` +
		`sphinxsidebar|wy-nav-side|wy-side-scroll|wy-menu|related|toctree-wrapper|` +
		`breadcrumb|breadcrumbs|header|footer|navpath|titlearea|tabs|nav-tree|` +
		`side-nav|tableofcontents|toc-tree|local-toc` +
		`)[^"']*["'][^>]*>.*?</div>`)
)

// CleanHTML converts fetched HTML/text into markdown using the named profile.
func CleanHTML(contentType string, body []byte, profile string) (title, markdown string, err error) {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "text/markdown", "text/plain":
		text := string(body)
		return firstHeading(text), text, nil
	}

	htmlStr := string(body)
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case ProfileSphinx:
		htmlStr = string(extractSphinxMain(body))
	case ProfileDoxygen:
		htmlStr = string(extractDoxygenMain(body))
	}
	htmlStr = stripDocChrome(htmlStr)
	if m := reTitle.FindStringSubmatch(string(body)); len(m) == 2 {
		title = strings.TrimSpace(m[1])
	}
	md, err := ingest.HTMLToMarkdown(htmlStr)
	if err != nil {
		return "", "", err
	}
	if title == "" {
		title = firstHeading(md)
	}
	title = NormalizeDocTitle(title, profile)
	return title, md, nil
}

func stripDocChrome(htmlStr string) string {
	for _, re := range reChromeBlocks {
		htmlStr = re.ReplaceAllString(htmlStr, "")
	}
	// Nested chrome divs may need a few passes.
	for i := 0; i < 3; i++ {
		next := reChromeDivs.ReplaceAllString(htmlStr, "")
		if next == htmlStr {
			break
		}
		htmlStr = next
	}
	return htmlStr
}

func firstHeading(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return ""
}

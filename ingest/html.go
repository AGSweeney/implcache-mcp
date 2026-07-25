package ingest

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

// IsHTMLExt reports whether the path looks like HTML.
func IsHTMLExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".html" || ext == ".htm"
}

// IsMarkdownExt reports whether the path is markdown (not HTML).
func IsMarkdownExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".mdx"
}

// IsDocExt reports whether the path is markdown or HTML for doc-style ingest.
func IsDocExt(name string) bool {
	return IsMarkdownExt(name) || IsHTMLExt(name)
}

// ShouldSkipHelpPath skips help shells, TOC indexes, and non-content trees
// (WebWorks / Creo online help and Rockwell CCW help).
func ShouldSkipHelpPath(relOrAbs string) bool {
	p := filepath.ToSlash(strings.ToLower(relOrAbs))
	base := filepath.Base(p)

	for _, seg := range []string{
		"/connect/", "/wwhelp/", "/scripts/", "/css/",
		"/mft/", // Rockwell mobile framework assets
	} {
		if strings.Contains(p, seg) {
			return true
		}
	}
	switch base {
	case "index.html", "index.htm", "creo_toolkit.html", "creo_toolkit_sx.js",
		"heading.htm", "search_results.htm", "search_results.html":
		return true
	}
	return false
}

// DocContent is cleaned text ready for chunking.
type DocContent struct {
	Title    string
	Markdown string
}

type preparedHTML struct {
	Title        string
	MarkdownHTML string
}

// HTMLToMarkdown converts HTML to Markdown after extracting main content.
func HTMLToMarkdown(htmlStr string) (string, error) {
	doc, err := PrepareHTML(htmlStr)
	if err != nil {
		return "", err
	}
	md, err := htmltomarkdown.ConvertString(doc.MarkdownHTML)
	if err != nil {
		return "", fmt.Errorf("html to markdown: %w", err)
	}
	return cleanMarkdown(md), nil
}

// PrepareHTML pulls real article content out of WebWorks chrome.
func PrepareHTML(htmlStr string) (preparedHTML, error) {
	root, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return preparedHTML{}, fmt.Errorf("parse html: %w", err)
	}

	title := strings.TrimSpace(textOfFirst(root, "title"))
	content := findByID(root, "page_content")
	if content == nil {
		content = findByID(root, "ww_content_container")
	}
	if content == nil {
		content = findByID(root, "page_content_container")
	}
	// Rockwell Connected Components Workbench (CCW) help.
	if content == nil {
		content = findByID(root, "content_section")
	}
	if content == nil {
		if body := findByTag(root, "body"); body != nil {
			stripChrome(body)
			content = body
		}
	} else {
		stripChrome(content)
	}
	if content == nil {
		return preparedHTML{Title: title, MarkdownHTML: htmlStr}, nil
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, content); err != nil {
		return preparedHTML{}, fmt.Errorf("render html: %w", err)
	}
	return preparedHTML{Title: title, MarkdownHTML: buf.String()}, nil
}

// ContentForDocIngest returns cleaned markdown (and preferred title) for ingest.
func ContentForDocIngest(path string, data []byte) (DocContent, error) {
	raw := string(data)
	if !IsHTMLExt(path) {
		return DocContent{Title: TitleFromPath(path), Markdown: raw}, nil
	}
	prep, err := PrepareHTML(raw)
	if err != nil {
		return DocContent{}, err
	}
	md, err := htmltomarkdown.ConvertString(prep.MarkdownHTML)
	if err != nil {
		return DocContent{}, fmt.Errorf("html to markdown: %w", err)
	}
	md = cleanMarkdown(md)
	title := prep.Title
	if title == "" {
		title = TitleFromPath(path)
	}
	return DocContent{Title: title, Markdown: md}, nil
}

func findByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findByID(c, id); found != nil {
			return found
		}
	}
	return nil
}

func findByTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findByTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func textOfFirst(n *html.Node, tag string) string {
	el := findByTag(n, tag)
	if el == nil {
		return ""
	}
	return strings.TrimSpace(collectText(el))
}

func collectText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(collectText(c))
	}
	return b.String()
}

func stripChrome(n *html.Node) {
	var remove []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			id := attr(node, "id")
			class := attr(node, "class")
			switch {
			case id == "wwconnect_header", id == "dropdown_ids",
				id == "page_header", id == "page_footer",
				id == "related_and_nav_column", id == "search_form",
				id == "TOCBoxContents",
				strings.Contains(class, "ww_skin_breadcrumbs"),
				strings.Contains(class, "ww_skin_page_toolbar"),
				strings.Contains(class, "relatedtopics"),
				strings.Contains(class, "breadcrumbs"),
				strings.Contains(class, "social"),
				strings.Contains(class, "feedback"),
				node.Data == "header",
				node.Data == "script",
				node.Data == "style",
				node.Data == "noscript":
				remove = append(remove, node)
				return
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	for _, node := range remove {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

var (
	reMDLink     = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
	reMultiBlank = regexp.MustCompile(`\n{3,}`)
)

func cleanMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			out = append(out, "")
			continue
		}
		if strings.HasPrefix(trim, "[](") {
			continue
		}
		if isBreadcrumbLine(trim) {
			continue
		}
		out = append(out, line)
	}
	md = strings.Join(out, "\n")
	md = reMultiBlank.ReplaceAllString(md, "\n\n")
	// html-to-markdown escapes underscores; restore for prose/API identifiers and FTS.
	md = strings.ReplaceAll(md, "\\_", "_")
	return strings.TrimSpace(md)
}

func isBreadcrumbLine(line string) bool {
	if !strings.Contains(line, "](") || !strings.Contains(line, ">") {
		return false
	}
	stripped := reMDLink.ReplaceAllString(line, "")
	stripped = strings.ReplaceAll(stripped, ">", "")
	stripped = strings.TrimSpace(stripped)
	return len(stripped) < 8
}

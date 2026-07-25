// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package pdf

import (
	"os"
	"strings"
	"unicode/utf8"

	ledong "github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"

	"implcache-mcp/store"
)

// PageText is extracted text for one PDF page.
type PageText struct {
	Number int
	Text   string
	Type   string // text|image-only
	Hash   string
}

// Bookmark is a flattened outline entry with page range.
type Bookmark struct {
	Title    string
	PageFrom int
	PageThru int
	Level    int
}

// ExtractPages returns cleaned page text for the inclusive range.
func ExtractPages(path string, pageStart, pageEnd int) ([]PageText, error) {
	f, r, err := ledong.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n := r.NumPage()
	start, end := pageRange(pageStart, pageEnd, n)
	raw := make([]string, 0, end-start+1)
	pages := make([]PageText, 0, end-start+1)
	for p := start; p <= end; p++ {
		text, err := pageText(r, p)
		if err != nil {
			pages = append(pages, PageText{Number: p, Type: "image-only"})
			raw = append(raw, "")
			continue
		}
		raw = append(raw, text)
		typ := "text"
		if utf8.RuneCountInString(strings.TrimSpace(text)) < minTextRunesPerPage {
			typ = "image-only"
		}
		pages = append(pages, PageText{Number: p, Text: text, Type: typ})
	}
	cleaned := suppressHeadersFooters(raw)
	for i := range pages {
		pages[i].Text = strings.TrimSpace(cleaned[i])
		pages[i].Hash = sha256Hex([]byte(pages[i].Text))
		if utf8.RuneCountInString(pages[i].Text) < minTextRunesPerPage {
			pages[i].Type = "image-only"
		} else {
			pages[i].Type = "text"
		}
	}
	return pages, nil
}

// LoadBookmarks returns outline entries with page ranges when available.
func LoadBookmarks(path string, pageCount int) ([]Bookmark, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	bms, err := api.Bookmarks(f, nil)
	if err != nil {
		return nil, nil
	}
	var out []Bookmark
	var walk func([]pdfcpu.Bookmark, int)
	walk = func(list []pdfcpu.Bookmark, level int) {
		for _, b := range list {
			from := b.PageFrom
			thru := b.PageThru
			if thru < from {
				thru = from
			}
			if thru > pageCount {
				thru = pageCount
			}
			if from > 0 {
				out = append(out, Bookmark{
					Title:    strings.TrimSpace(b.Title),
					PageFrom: from,
					PageThru: thru,
					Level:    level,
				})
			}
			if len(b.Kids) > 0 {
				walk(b.Kids, level+1)
			}
		}
	}
	walk(bms, 1)
	return out, nil
}

// BuildSections turns pages (+ optional bookmarks) into markdown-like sections.
func BuildSections(pages []PageText, bookmarks []Bookmark) []section {
	if len(pages) == 0 {
		return nil
	}
	first, last := pages[0].Number, pages[len(pages)-1].Number
	byPage := map[int]PageText{}
	for _, p := range pages {
		byPage[p.Number] = p
	}

	if len(bookmarks) == 0 {
		var body strings.Builder
		for i, p := range pages {
			if i > 0 {
				body.WriteByte('\n')
			}
			body.WriteString(p.Text)
		}
		return []section{{
			Heading:   "",
			StartPage: first,
			EndPage:   last,
			Body:      strings.TrimSpace(body.String()),
		}}
	}

	// Assign page ranges; fill gaps before first bookmark.
	var secs []section
	if bookmarks[0].PageFrom > first {
		secs = append(secs, section{
			Heading: "", StartPage: first, EndPage: bookmarks[0].PageFrom - 1,
			Body: joinPages(byPage, first, bookmarks[0].PageFrom-1),
		})
	}
	for i, bm := range bookmarks {
		end := bm.PageThru
		if end < bm.PageFrom {
			end = bm.PageFrom
		}
		if i+1 < len(bookmarks) && bookmarks[i+1].PageFrom-1 >= bm.PageFrom {
			// Prefer explicit PageThru; otherwise extend to next bookmark.
			if bm.PageThru <= bm.PageFrom {
				end = bookmarks[i+1].PageFrom - 1
			}
		}
		if end > last {
			end = last
		}
		if bm.PageFrom > last || end < first {
			continue
		}
		from := bm.PageFrom
		if from < first {
			from = first
		}
		heading := bm.Title
		body := joinPages(byPage, from, end)
		if heading != "" {
			body = "# " + heading + "\n\n" + body
		}
		secs = append(secs, section{
			Heading: heading, StartPage: from, EndPage: end, Body: strings.TrimSpace(body),
		})
	}
	if len(secs) == 0 {
		return BuildSections(pages, nil)
	}
	return secs
}

type section struct {
	Heading   string
	StartPage int
	EndPage   int
	Body      string
}

func joinPages(byPage map[int]PageText, from, to int) string {
	var b strings.Builder
	for p := from; p <= to; p++ {
		pt, ok := byPage[p]
		if !ok || strings.TrimSpace(pt.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(pt.Text)
	}
	return b.String()
}

// ChunkSections size-bounds section bodies and stamps page citations.
func ChunkSections(secs []section) []store.Chunk {
	var out []store.Chunk
	for _, sec := range secs {
		if strings.TrimSpace(sec.Body) == "" {
			continue
		}
		parts := sizeChunkBody(sec.Heading, sec.Body, defaultMaxChunkBytes)
		for _, p := range parts {
			p.StartPage = sec.StartPage
			p.EndPage = sec.EndPage
			out = append(out, p)
		}
	}
	return out
}

const defaultMaxChunkBytes = 3500

func sizeChunkBody(heading, body string, maxBytes int) []store.Chunk {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	if len(body) <= maxBytes {
		return []store.Chunk{{Heading: heading, Body: body, StartLine: 1, EndLine: 1}}
	}
	paras := strings.Split(body, "\n")
	var (
		out      []store.Chunk
		buf      []string
		bufBytes int
	)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		out = append(out, store.Chunk{
			Heading:   heading,
			Body:      strings.Join(buf, "\n"),
			StartLine: 1,
			EndLine:   1,
		})
		buf = nil
		bufBytes = 0
	}
	for _, line := range paras {
		lineBytes := len(line) + 1
		if bufBytes > 0 && bufBytes+lineBytes > maxBytes {
			flush()
		}
		buf = append(buf, line)
		bufBytes += lineBytes
		if bufBytes >= maxBytes {
			flush()
		}
	}
	flush()
	return out
}

// suppressHeadersFooters strips lines that repeat on most pages (headers/footers).
func suppressHeadersFooters(pages []string) []string {
	if len(pages) < 2 {
		return pages
	}
	type edge struct {
		first string
		last  string
	}
	edges := make([]edge, len(pages))
	counts := map[string]int{}
	for i, page := range pages {
		lines := nonEmptyLines(page)
		if len(lines) == 0 {
			continue
		}
		edges[i] = edge{first: lines[0], last: lines[len(lines)-1]}
		counts[lines[0]]++
		if lines[len(lines)-1] != lines[0] {
			counts[lines[len(lines)-1]]++
		}
	}
	threshold := (len(pages) + 1) / 2
	if threshold < 2 {
		threshold = 2
	}
	drop := map[string]bool{}
	for line, n := range counts {
		if n >= threshold && utf8.RuneCountInString(line) <= 120 {
			drop[line] = true
		}
	}
	if len(drop) == 0 {
		return pages
	}
	out := make([]string, len(pages))
	for i, page := range pages {
		var keep []string
		for _, line := range splitKeepEmpty(page) {
			trim := strings.TrimSpace(line)
			if trim != "" && drop[trim] {
				continue
			}
			keep = append(keep, line)
		}
		out[i] = strings.TrimSpace(strings.Join(keep, "\n"))
	}
	return out
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitKeepEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

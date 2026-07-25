// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	ledong "github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

const (
	DefaultMaxFileBytes = 64 << 20
	DefaultMaxPages     = 5000
	minTextRunesPerPage = 40
)

// InspectReport describes a PDF without modifying the database.
type InspectReport struct {
	SourcePath        string            `json:"sourcePath"`
	FileName          string            `json:"fileName"`
	FileSize          int64             `json:"fileSize"`
	PageCount         int               `json:"pageCount"`
	Title             string            `json:"title"`
	Author            string            `json:"author,omitempty"`
	Subject           string            `json:"subject,omitempty"`
	Creator           string            `json:"creator,omitempty"`
	Producer          string            `json:"producer,omitempty"`
	Encrypted         bool              `json:"encrypted"`
	Classification    string            `json:"classification"` // text|image-only|mixed|encrypted|corrupt
	TextPages         int               `json:"textPages"`
	ImageOnlyPages    int               `json:"imageOnlyPages"`
	EstimatedOCRPages int               `json:"estimatedOcrPages"`
	Bookmarks         int               `json:"bookmarks"`
	BookmarkTitles    []string          `json:"bookmarkTitles,omitempty"`
	Warnings          []string          `json:"warnings,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// InspectOptions configures inspection limits.
type InspectOptions struct {
	MaxFileBytes int64
	MaxPages     int
	PageStart    int // 1-based inclusive; 0 = first
	PageEnd      int // 1-based inclusive; 0 = last
}

// InspectPDF analyzes a local PDF without database writes.
func InspectPDF(path string, opt InspectOptions) (*InspectReport, error) {
	if opt.MaxFileBytes <= 0 {
		opt.MaxFileBytes = DefaultMaxFileBytes
	}
	if opt.MaxPages <= 0 {
		opt.MaxPages = DefaultMaxPages
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(abs), ".pdf") {
		return nil, fmt.Errorf("unsupported file type %q", filepath.Ext(abs))
	}
	if info.Size() > opt.MaxFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", opt.MaxFileBytes)
	}

	rep := &InspectReport{
		SourcePath: abs,
		FileName:   filepath.Base(abs),
		FileSize:   info.Size(),
		Metadata:   map[string]string{},
	}

	ctx, err := api.ReadContextFile(abs)
	if err != nil {
		rep.Classification = "corrupt"
		rep.Warnings = append(rep.Warnings, err.Error())
		return rep, nil
	}
	if ctx.Encrypt != nil {
		rep.Encrypted = true
		rep.Classification = "encrypted"
		rep.Warnings = append(rep.Warnings, "password required or encrypted PDF")
		return rep, nil
	}
	rep.PageCount = ctx.PageCount
	if rep.PageCount > opt.MaxPages {
		return nil, fmt.Errorf("page count %d exceeds max %d", rep.PageCount, opt.MaxPages)
	}
	if ctx.Title != "" {
		rep.Title = ctx.Title
		rep.Metadata["title"] = ctx.Title
	}
	if ctx.Author != "" {
		rep.Author = ctx.Author
		rep.Metadata["author"] = ctx.Author
	}
	if ctx.Subject != "" {
		rep.Subject = ctx.Subject
		rep.Metadata["subject"] = ctx.Subject
	}
	if ctx.Creator != "" {
		rep.Creator = ctx.Creator
	}
	if ctx.Producer != "" {
		rep.Producer = ctx.Producer
	}

	if titles := bookmarkTitles(abs, 20); len(titles) > 0 {
		rep.Bookmarks = len(titles)
		rep.BookmarkTitles = titles
	}

	start, end := pageRange(opt.PageStart, opt.PageEnd, rep.PageCount)
	f, r, err := ledong.Open(abs)
	if err != nil {
		rep.Classification = "corrupt"
		rep.Warnings = append(rep.Warnings, err.Error())
		return rep, nil
	}
	defer f.Close()
	totalPages := r.NumPage()
	if totalPages < end {
		end = totalPages
	}
	for p := start; p <= end; p++ {
		text, err := pageText(r, p)
		if err != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("page %d: %v", p, err))
			rep.ImageOnlyPages++
			continue
		}
		if utf8.RuneCountInString(strings.TrimSpace(text)) < minTextRunesPerPage {
			rep.ImageOnlyPages++
		} else {
			rep.TextPages++
		}
	}
	rep.EstimatedOCRPages = rep.ImageOnlyPages
	switch {
	case rep.TextPages == 0 && rep.ImageOnlyPages > 0:
		rep.Classification = "image-only"
		rep.Warnings = append(rep.Warnings, "OCR required")
	case rep.ImageOnlyPages > 0 && rep.TextPages > 0:
		rep.Classification = "mixed"
	default:
		rep.Classification = "text"
	}
	if rep.Title == "" {
		rep.Title = strings.TrimSuffix(rep.FileName, filepath.Ext(rep.FileName))
	}
	return rep, nil
}

func bookmarkTitles(path string, limit int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	bms, err := api.Bookmarks(f, nil)
	if err != nil || len(bms) == 0 {
		return nil
	}
	var out []string
	var walk func([]pdfcpu.Bookmark)
	walk = func(list []pdfcpu.Bookmark) {
		for _, b := range list {
			if len(out) >= limit {
				return
			}
			if strings.TrimSpace(b.Title) != "" {
				out = append(out, b.Title)
			}
			if len(b.Kids) > 0 {
				walk(b.Kids)
			}
		}
	}
	walk(bms)
	return out
}

func pageRange(start, end, n int) (int, int) {
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > n {
		end = n
	}
	if start > end {
		start = end
	}
	return start, end
}

func pageText(r *ledong.Reader, page int) (string, error) {
	p := r.Page(page)
	if p.V.IsNull() {
		return "", fmt.Errorf("missing page")
	}
	fonts := map[string]*ledong.Font{}
	for _, name := range p.Fonts() {
		f := p.Font(name)
		fonts[name] = &f
	}
	text, err := p.GetPlainText(fonts)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

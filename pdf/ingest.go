// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"implcache-mcp/store"
)

// IngestOptions configures PDF ingestion.
type IngestOptions struct {
	Path         string
	RootName     string
	Authority    string
	Product      string
	Version      string
	Language     string
	OCRMode      string // off (default); OCR not implemented in Stage 1
	PageStart    int
	PageEnd      int
	MaxFileBytes int64
	MaxPages     int
	Force        bool // reingest even if hash matches
}

// IngestResult is the report for a PDF ingest.
type IngestResult struct {
	DocumentURI    string   `json:"documentUri"`
	RootName       string   `json:"rootName"`
	SourcePath     string   `json:"sourcePath"`
	Title          string   `json:"title"`
	FileHash       string   `json:"fileHash"`
	PageCount      int      `json:"pageCount"`
	Chunks         int      `json:"chunks"`
	Classification string   `json:"classification"`
	Skipped        bool     `json:"skipped"`
	Status         string   `json:"status"`
	Warnings       []string `json:"warnings,omitempty"`
	DurationMS     int64    `json:"durationMs"`
}

// IngestPDF extracts a local text PDF and indexes it with page citations.
func IngestPDF(ctx context.Context, st *store.Store, opt IngestOptions) (*IngestResult, error) {
	start := time.Now()
	if opt.OCRMode == "" {
		opt.OCRMode = "off"
	}
	if opt.OCRMode != "off" {
		return nil, fmt.Errorf("ocrMode %q not supported in Stage 1 (use off)", opt.OCRMode)
	}
	rep, err := InspectPDF(opt.Path, InspectOptions{
		MaxFileBytes: opt.MaxFileBytes,
		MaxPages:     opt.MaxPages,
		PageStart:    opt.PageStart,
		PageEnd:      opt.PageEnd,
	})
	if err != nil {
		return nil, err
	}
	root := strings.TrimSpace(opt.RootName)
	if root == "" {
		root = "pdf-docs"
	}
	authority := opt.Authority
	if authority == "" {
		authority = store.AuthorityOfficialDocs
	}
	hash, err := fileSHA256(rep.SourcePath)
	if err != nil {
		return nil, err
	}
	fileName := normalizeFileName(rep.FileName)
	uri := DocumentURI(root, fileName)
	res := &IngestResult{
		DocumentURI:    uri,
		RootName:       root,
		SourcePath:     rep.SourcePath,
		Title:          rep.Title,
		FileHash:       hash,
		PageCount:      rep.PageCount,
		Classification: rep.Classification,
		Warnings:       append([]string{}, rep.Warnings...),
		DurationMS:     time.Since(start).Milliseconds(),
	}

	switch rep.Classification {
	case "encrypted":
		return nil, fmt.Errorf("encrypted PDF cannot be ingested")
	case "corrupt":
		return nil, fmt.Errorf("corrupt PDF: %s", strings.Join(rep.Warnings, "; "))
	case "image-only":
		return nil, fmt.Errorf("image-only PDF requires OCR (not available in Stage 1)")
	}

	if !opt.Force {
		existing, err := st.GetHashByURI(ctx, uri)
		if err != nil {
			return nil, err
		}
		if existing == hash {
			res.Skipped = true
			res.Status = "unchanged"
			return res, nil
		}
	}

	pages, err := ExtractPages(rep.SourcePath, opt.PageStart, opt.PageEnd)
	if err != nil {
		return nil, err
	}
	textPages := 0
	for _, p := range pages {
		if p.Type == "text" {
			textPages++
		}
	}
	if textPages == 0 {
		return nil, fmt.Errorf("no extractable text in selected page range (OCR required)")
	}

	bookmarks, _ := LoadBookmarks(rep.SourcePath, rep.PageCount)
	secs := BuildSections(pages, bookmarks)
	chunks := ChunkSections(secs)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no indexable content extracted")
	}

	info, err := os.Stat(rep.SourcePath)
	if err != nil {
		return nil, err
	}
	written, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI:            uri,
		Title:          rep.Title,
		SourceType:     store.SourcePDF,
		Path:           rep.SourcePath,
		RootName:       root,
		Authority:      authority,
		Technology:     opt.Product,
		Language:       opt.Language,
		ProductVersion: opt.Version,
		Mtime:          info.ModTime().Unix(),
		Hash:           hash,
		Chunks:         chunks,
	})
	if err != nil {
		return nil, err
	}
	if !written && !opt.Force {
		res.Skipped = true
		res.Status = "unchanged"
		return res, nil
	}

	doc, _, err := st.GetDocumentByURI(ctx, uri)
	if err != nil {
		return nil, err
	}
	src := store.PDFSource{
		DocumentID:       doc.ID,
		RootName:         root,
		DocumentURI:      uri,
		SourcePath:       rep.SourcePath,
		FileName:         fileName,
		FileHash:         hash,
		FileSize:         rep.FileSize,
		PageCount:        rep.PageCount,
		Title:            rep.Title,
		Product:          opt.Product,
		Version:          opt.Version,
		Authority:        authority,
		Language:         opt.Language,
		Encrypted:        false,
		OCRMode:          opt.OCRMode,
		ExtractionStatus: rep.Classification,
	}
	pdfPages := make([]store.PDFPage, 0, len(pages))
	for _, p := range pages {
		pdfPages = append(pdfPages, store.PDFPage{
			PageNumber:           p.Number,
			TextHash:             p.Hash,
			TextLength:           len(p.Text),
			PageType:             p.Type,
			OCRUsed:              false,
			LayoutType:           "single",
			ExtractionConfidence: "unknown",
		})
	}
	if _, err := st.UpsertPDFSource(ctx, src, pdfPages); err != nil {
		return nil, err
	}

	res.Chunks = len(chunks)
	res.Status = "ingested"
	res.DurationMS = time.Since(start).Milliseconds()
	return res, nil
}

// RemovePDF deletes an ingested PDF document and its pdf_* rows.
func RemovePDF(ctx context.Context, st *store.Store, uri string) (bool, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return false, fmt.Errorf("uri is required")
	}
	if _, err := st.DeletePDFSourceByURI(ctx, uri); err != nil {
		return false, err
	}
	return st.DeleteDocument(ctx, uri)
}

// DocumentURI builds pdf://{root}/{normalized-file-name}.
func DocumentURI(rootName, fileName string) string {
	rootName = strings.Trim(rootName, "/")
	fileName = normalizeFileName(fileName)
	return "pdf://" + rootName + "/" + fileName
}

func normalizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "document.pdf"
	}
	return name
}

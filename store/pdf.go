// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PDFSource tracks an ingested PDF file.
type PDFSource struct {
	ID               int64  `json:"id"`
	DocumentID       int64  `json:"documentId,omitempty"`
	RootName         string `json:"rootName"`
	DocumentURI      string `json:"documentUri"`
	SourcePath       string `json:"sourcePath"`
	FileName         string `json:"fileName"`
	FileHash         string `json:"fileHash"`
	FileSize         int64  `json:"fileSize"`
	PageCount        int    `json:"pageCount"`
	Title            string `json:"title"`
	Product          string `json:"product"`
	Version          string `json:"version"`
	Authority        string `json:"authority"`
	Language         string `json:"language"`
	PDFVersion       string `json:"pdfVersion,omitempty"`
	Encrypted        bool   `json:"encrypted"`
	OCRMode          string `json:"ocrMode"`
	ExtractionStatus string `json:"extractionStatus"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

// PDFPage is per-page extraction metadata.
type PDFPage struct {
	ID                   int64  `json:"id"`
	PDFSourceID          int64  `json:"pdfSourceId"`
	PageNumber           int    `json:"pageNumber"`
	PageLabel            string `json:"pageLabel,omitempty"`
	TextHash             string `json:"textHash"`
	TextLength           int    `json:"textLength"`
	PageType             string `json:"pageType"`
	OCRUsed              bool   `json:"ocrUsed"`
	LayoutType           string `json:"layoutType"`
	ExtractionConfidence string `json:"extractionConfidence"`
	WarningFlags         string `json:"warningFlags,omitempty"`
}

// UpsertPDFSource creates or replaces a PDF source and its page rows.
func (s *Store) UpsertPDFSource(ctx context.Context, in PDFSource, pages []PDFPage) (int64, error) {
	uri := strings.TrimSpace(in.DocumentURI)
	if uri == "" {
		return 0, fmt.Errorf("documentUri is required")
	}
	if in.Authority == "" {
		in.Authority = AuthorityOfficialDocs
	}
	if in.OCRMode == "" {
		in.OCRMode = "off"
	}
	now := time.Now().Unix()
	enc := 0
	if in.Encrypted {
		enc = 1
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM pdf_sources WHERE document_uri = ?`, uri).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := tx.ExecContext(ctx, `
			INSERT INTO pdf_sources (
				document_id, root_name, document_uri, source_path, file_name, file_hash, file_size,
				page_count, title, product, version, authority, language, pdf_version, encrypted,
				ocr_mode, extraction_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			nullInt64(in.DocumentID), in.RootName, uri, in.SourcePath, in.FileName, in.FileHash, in.FileSize,
			in.PageCount, in.Title, in.Product, in.Version, in.Authority, in.Language, in.PDFVersion, enc,
			in.OCRMode, in.ExtractionStatus, now, now,
		)
		if err != nil {
			return 0, err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
	case err != nil:
		return 0, err
	default:
		if _, err := tx.ExecContext(ctx, `
			UPDATE pdf_sources SET
				document_id = ?, root_name = ?, source_path = ?, file_name = ?, file_hash = ?, file_size = ?,
				page_count = ?, title = ?, product = ?, version = ?, authority = ?, language = ?,
				pdf_version = ?, encrypted = ?, ocr_mode = ?, extraction_status = ?, updated_at = ?
			WHERE id = ?`,
			nullInt64(in.DocumentID), in.RootName, in.SourcePath, in.FileName, in.FileHash, in.FileSize,
			in.PageCount, in.Title, in.Product, in.Version, in.Authority, in.Language,
			in.PDFVersion, enc, in.OCRMode, in.ExtractionStatus, now, id,
		); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pdf_pages WHERE pdf_source_id = ?`, id); err != nil {
			return 0, err
		}
	}

	for _, p := range pages {
		ocr := 0
		if p.OCRUsed {
			ocr = 1
		}
		pageType := p.PageType
		if pageType == "" {
			pageType = "text"
		}
		layout := p.LayoutType
		if layout == "" {
			layout = "single"
		}
		conf := p.ExtractionConfidence
		if conf == "" {
			conf = "unknown"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pdf_pages (
				pdf_source_id, page_number, page_label, text_hash, text_length, page_type,
				ocr_used, layout_type, extraction_confidence, warning_flags)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, p.PageNumber, p.PageLabel, p.TextHash, p.TextLength, pageType,
			ocr, layout, conf, p.WarningFlags,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// GetPDFSourceByURI loads a PDF source row.
func (s *Store) GetPDFSourceByURI(ctx context.Context, uri string) (*PDFSource, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(document_id, 0), root_name, document_uri, source_path, file_name, file_hash,
		       file_size, page_count, title, product, version, authority, language, pdf_version,
		       encrypted, ocr_mode, extraction_status, created_at, updated_at
		FROM pdf_sources WHERE document_uri = ?`, uri)
	return scanPDFSource(row)
}

// DeletePDFSourceByURI removes pdf_sources (cascades pdf_pages). Document is left to caller.
func (s *Store) DeletePDFSourceByURI(ctx context.Context, uri string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pdf_sources WHERE document_uri = ?`, uri)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func scanPDFSource(row *sql.Row) (*PDFSource, error) {
	var p PDFSource
	var enc int
	if err := row.Scan(
		&p.ID, &p.DocumentID, &p.RootName, &p.DocumentURI, &p.SourcePath, &p.FileName, &p.FileHash,
		&p.FileSize, &p.PageCount, &p.Title, &p.Product, &p.Version, &p.Authority, &p.Language, &p.PDFVersion,
		&enc, &p.OCRMode, &p.ExtractionStatus, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pdf source not found")
		}
		return nil, err
	}
	p.Encrypted = enc != 0
	return &p, nil
}

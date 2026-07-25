// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"implcache-mcp/ingest"
	"implcache-mcp/store"
)

// IngestURLOptions configures single-page URL ingestion.
type IngestURLOptions struct {
	URL               string
	RootName          string
	Authority         string
	Product           string
	Version           string
	Target            string
	Language          string
	Profile           string
	AllowInsecureHTTP bool
	MaxBytes          int64
	ExtraAllowedHosts map[string]struct{}
}

// URLIngestResult is the report for a single URL ingest.
type URLIngestResult struct {
	URL          string `json:"url"`
	CanonicalURL string `json:"canonicalUrl"`
	DocumentURI  string `json:"documentUri"`
	RootName     string `json:"rootName"`
	Title        string `json:"title"`
	ContentHash  string `json:"contentHash"`
	Skipped      bool   `json:"skipped"`
	Chunks       int    `json:"chunks"`
	Status       string `json:"status"`
	DurationMS   int64  `json:"durationMs"`
}

// IngestURL fetches one approved URL and indexes it.
func IngestURL(ctx context.Context, st *store.Store, opt IngestURLOptions) (*URLIngestResult, error) {
	start := time.Now()
	root := strings.TrimSpace(opt.RootName)
	if root == "" {
		root = "web-docs"
	}
	profile := opt.Profile
	if profile == "" {
		profile = ProfileGeneric
	}
	authority := opt.Authority
	if authority == "" {
		authority = store.AuthorityOfficialDocs
	}

	page, err := FetchURL(ctx, opt.URL, FetchOptions{
		AllowInsecureHTTP: opt.AllowInsecureHTTP,
		MaxBytes:          opt.MaxBytes,
		ExtraAllowedHosts: opt.ExtraAllowedHosts,
	})
	if err != nil {
		return nil, err
	}
	rel := RelativePathFromURL(page.CanonicalURL)
	if profile == ProfileSphinx && SkipSphinxPath(rel) {
		return &URLIngestResult{
			URL: opt.URL, CanonicalURL: page.CanonicalURL, RootName: root,
			Skipped: true, Status: "skipped_profile", DurationMS: time.Since(start).Milliseconds(),
		}, nil
	}
	if profile == ProfileDoxygen && SkipDoxygenPath(rel) {
		return &URLIngestResult{
			URL: opt.URL, CanonicalURL: page.CanonicalURL, RootName: root,
			Skipped: true, Status: "skipped_profile", DurationMS: time.Since(start).Milliseconds(),
		}, nil
	}

	title, md, err := CleanHTML(page.ContentType, page.Body, profile)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(md) == "" {
		return nil, fmt.Errorf("no extractable content from %s", page.CanonicalURL)
	}
	if title == "" {
		title = path.Base(rel)
	}
	hash := sha256Hex(page.Body)
	uri := ingest.ProjectURI(root, rel)
	existing, err := st.GetHashByURI(ctx, uri)
	if err != nil {
		return nil, err
	}
	res := &URLIngestResult{
		URL:          opt.URL,
		CanonicalURL: page.CanonicalURL,
		DocumentURI:  uri,
		RootName:     root,
		Title:        title,
		ContentHash:  hash,
		DurationMS:   time.Since(start).Milliseconds(),
	}
	if existing == hash {
		res.Skipped = true
		res.Status = "unchanged"
		return res, nil
	}
	chunks := ingest.ChunkMarkdown(md)
	written, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI:            uri,
		Title:          title,
		SourceType:     store.SourceWeb,
		Path:           page.CanonicalURL,
		RootName:       root,
		Authority:      authority,
		Technology:     opt.Product,
		Language:       opt.Language,
		ProductVersion: firstNonEmpty(opt.Version, opt.Target),
		Hash:           hash,
		Mtime:          time.Now().Unix(),
		Chunks:         chunks,
	})
	if err != nil {
		return nil, err
	}
	res.Chunks = len(chunks)
	if written {
		res.Status = "ingested"
	} else {
		res.Skipped = true
		res.Status = "unchanged"
	}
	res.DurationMS = time.Since(start).Milliseconds()
	return res, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

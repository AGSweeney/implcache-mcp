// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"implcache-mcp/store"
)

// ProjectResult summarizes an ingest_project run.
type ProjectResult struct {
	RootName string   `json:"rootName"`
	Ingested int      `json:"ingested"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
	URIs     []string `json:"uris,omitempty"`
}

// ProjectOptions configures source-tree ingest limits.
type ProjectOptions struct {
	Path             string
	RootName         string
	MaxFiles         int
	MaxDocumentBytes int64
}

// IngestProject walks a source tree and ingests text-like files.
func IngestProject(ctx context.Context, st *store.Store, rootPath, rootName string) (*ProjectResult, error) {
	return IngestProjectOpts(ctx, st, ProjectOptions{Path: rootPath, RootName: rootName})
}

// IngestProjectOpts is the configurable entry point for project ingest.
func IngestProjectOpts(ctx context.Context, st *store.Store, opt ProjectOptions) (*ProjectResult, error) {
	maxBytes := opt.MaxDocumentBytes
	if maxBytes <= 0 {
		maxBytes = maxFileBytes
	}
	maxFiles := opt.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 50000
	}

	absRoot, err := filepath.Abs(opt.Path)
	if err != nil {
		return nil, err
	}
	if eval, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = eval
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absRoot)
	}
	rootName := strings.Trim(opt.RootName, "/")
	if rootName == "" {
		rootName = filepath.Base(absRoot)
	}

	res := &ProjectResult{RootName: rootName}
	filesSeen := 0

	err = filepath.WalkDir(absRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if p != absRoot && ShouldIgnoreDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if ShouldIgnoreFile(d.Name()) || !IsTextExt(d.Name()) {
			return nil
		}
		if ShouldSkipHelpPath(p) {
			return nil
		}
		filesSeen++
		if filesSeen > maxFiles {
			return fmt.Errorf("ingest file limit exceeded (%d)", maxFiles)
		}
		if err := ingestProjectFile(ctx, st, absRoot, rootName, p, maxBytes, res); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
		}
		return nil
	})
	return res, err
}

func ingestProjectFile(ctx context.Context, st *store.Store, absRoot, rootName, path string, maxBytes int64, res *ProjectResult) error {
	data, info, err := readTextFileLimited(path, maxBytes)
	if err != nil {
		return err
	}

	rel, err := relativePathWithin(absRoot, path)
	if err != nil {
		return err
	}
	uri := ProjectURI(rootName, rel)
	hash := sha256Hex(data)

	existing, err := st.GetHashByURI(ctx, uri)
	if err != nil {
		return err
	}
	if existing == hash {
		res.Skipped++
		return nil
	}

	sourceType := store.SourceSource
	var chunks []store.Chunk
	content := string(data)

	title := TitleFromPath(rel)
	switch {
	case IsHTMLExt(path):
		doc, err := ContentForDocIngest(path, data)
		if err != nil {
			return err
		}
		if strings.TrimSpace(doc.Markdown) == "" {
			res.Skipped++
			return nil
		}
		sourceType = store.SourceMarkdown
		content = doc.Markdown
		if doc.Title != "" {
			title = doc.Title
		}
		chunks = ChunkMarkdown(content)
	case IsMarkdownExt(path):
		sourceType = store.SourceMarkdown
		chunks = ChunkMarkdown(content)
	default:
		chunks = ChunkSource(content)
	}

	syms := ExtractSymbols(path, content)
	written, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI:            uri,
		Title:          title,
		SourceType:     sourceType,
		Path:           rel,
		RootName:       rootName,
		Authority:      InferAuthority(rootName, rel),
		Language:       languageFromPath(path),
		ProductVersion: InferProductVersion(rootName, rel, content),
		Mtime:          info.ModTime().Unix(),
		Hash:           hash,
		Chunks:         chunks,
		Symbols:        syms,
	})
	if err != nil {
		return err
	}
	if written {
		res.Ingested++
		res.URIs = append(res.URIs, uri)
	} else {
		res.Skipped++
	}
	return nil
}

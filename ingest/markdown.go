// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"implcache-mcp/store"
)

const maxFileBytes = 8 << 20 // 8 MiB (Creo/help HTML can exceed 1 MiB)

// MarkdownResult summarizes an ingest_markdown run.
type MarkdownResult struct {
	RootName string   `json:"rootName"`
	Ingested int      `json:"ingested"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
	URIs     []string `json:"uris,omitempty"`
}

// MarkdownOptions configures markdown/HTML ingest limits.
type MarkdownOptions struct {
	Path             string
	Recursive        bool
	RootName         string
	MaxFiles         int
	MaxDocumentBytes int64
}

// IngestMarkdown ingests markdown or HTML into the store using portable
// project://{rootName}/{rel} URIs (never absolute filesystem paths).
func IngestMarkdown(ctx context.Context, st *store.Store, path string, recursive bool, rootName string) (*MarkdownResult, error) {
	return IngestMarkdownOpts(ctx, st, MarkdownOptions{Path: path, Recursive: recursive, RootName: rootName})
}

// IngestMarkdownOpts is the configurable entry point for markdown/HTML ingest.
func IngestMarkdownOpts(ctx context.Context, st *store.Store, opt MarkdownOptions) (*MarkdownResult, error) {
	maxBytes := opt.MaxDocumentBytes
	if maxBytes <= 0 {
		maxBytes = maxFileBytes
	}
	maxFiles := opt.MaxFiles
	if maxFiles <= 0 {
		maxFiles = 50000
	}

	abs, err := filepath.Abs(opt.Path)
	if err != nil {
		return nil, err
	}
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		abs = eval
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}

	rootName := strings.Trim(opt.RootName, "/")
	var absRoot string
	if info.IsDir() {
		absRoot = abs
		if rootName == "" {
			rootName = filepath.Base(absRoot)
		}
	} else {
		absRoot = filepath.Dir(abs)
		if rootName == "" {
			rootName = filepath.Base(absRoot)
		}
	}

	res := &MarkdownResult{RootName: rootName}
	filesSeen := 0

	if !info.IsDir() {
		if err := ingestMarkdownFile(ctx, st, absRoot, rootName, abs, maxBytes, res); err != nil {
			res.Errors = append(res.Errors, err.Error())
		}
		return res, nil
	}

	walkFn := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // do not follow file/dir symlinks during ingest
		}
		if d.IsDir() {
			if p != absRoot && !opt.Recursive {
				return fs.SkipDir
			}
			if ShouldIgnoreDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !IsDocExt(d.Name()) {
			return nil
		}
		if ShouldSkipHelpPath(p) {
			return nil
		}
		filesSeen++
		if filesSeen > maxFiles {
			return fmt.Errorf("ingest file limit exceeded (%d)", maxFiles)
		}
		if err := ingestMarkdownFile(ctx, st, absRoot, rootName, p, maxBytes, res); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
		}
		return nil
	}

	if err := filepath.WalkDir(absRoot, walkFn); err != nil {
		return res, err
	}
	return res, nil
}

func ingestMarkdownFile(ctx context.Context, st *store.Store, absRoot, rootName, path string, maxBytes int64, res *MarkdownResult) error {
	if ShouldSkipHelpPath(path) {
		res.Skipped++
		return nil
	}
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

	doc, err := ContentForDocIngest(path, data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(doc.Markdown) == "" {
		res.Skipped++
		return nil
	}
	title := doc.Title
	if title == "" {
		title = TitleFromPath(rel)
	}
	chunks := ChunkMarkdown(doc.Markdown)
	written, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI:        uri,
		Title:      title,
		SourceType: store.SourceMarkdown,
		Path:       rel,
		RootName:   rootName,
		Authority:  InferAuthority(rootName, rel),
		Language:   languageFromPath(path),
		Mtime:      info.ModTime().Unix(),
		Hash:       hash,
		Chunks:     chunks,
		Symbols:    ExtractSymbols(path, doc.Markdown),
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

func readTextFile(path string) ([]byte, os.FileInfo, error) {
	return readTextFileLimited(path, maxFileBytes)
}

func readTextFileLimited(path string, maxBytes int64) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("refusing to ingest symlink")
	}
	if info.Size() > maxBytes {
		return nil, nil, fmt.Errorf("file too large (%d bytes; max %d)", info.Size(), maxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return DecodeTextToUTF8(data), info, nil
}

func relativePathWithin(absRoot, path string) (string, error) {
	rel, err := filepath.Rel(absRoot, path)
	if err != nil {
		return "", err
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes ingest root")
	}
	return filepath.ToSlash(rel), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

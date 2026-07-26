// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"implcache-mcp/librarydocs"
	"implcache-mcp/manifest"
	"implcache-mcp/store"
)

// ProjectResult summarizes an ingest_project run.
type ProjectResult struct {
	RootName       string                    `json:"rootName"`
	Ingested       int                       `json:"ingested"`
	Skipped        int                       `json:"skipped"`
	Errors         []string                  `json:"errors,omitempty"`
	URIs           []string                  `json:"uris,omitempty"`
	Files          []IngestedFile            `json:"files,omitempty"`
	BytesProcessed int64                     `json:"bytesProcessed,omitempty"`
	LibraryDocs    *librarydocs.PackageSummary `json:"libraryDocs,omitempty"`
}

// IngestedFile is one file written or skipped during tree ingest.
type IngestedFile struct {
	URI          string `json:"uri"`
	RelativePath string `json:"relativePath"`
	ContentHash  string `json:"contentHash"`
	Language     string `json:"language,omitempty"`
	SourceType   string `json:"sourceType"`
	FileSize     int64  `json:"fileSize"`
	Written      bool   `json:"written"`
}

// PathFilter decides whether a relative path should be ingested.
type PathFilter func(rel string) bool

// ProjectProgressFunc reports per-file ingest progress (optional).
// done is files considered so far; total is 0 when unknown.
type ProjectProgressFunc func(done, total int, bytes int64, currentPath, message string)

// ProjectOptions configures source-tree ingest limits.
type ProjectOptions struct {
	Path              string
	RootName          string
	MaxFiles          int
	MaxDocumentBytes  int64
	MaxTotalBytes     int64
	IncludePatterns   []string
	ExcludePatterns   []string
	PathFilter        PathFilter // if set, overrides include/exclude helpers
	URIScheme           string // "project" (default) or "git"
	SourceType          string // override source type; empty = infer markdown/source
	Authority           string
	OnlyRelativePaths   []string // if non-empty, only these relative paths (refresh)
	SkipDirNames        map[string]struct{}
	Progress            ProjectProgressFunc
	LibraryDocsHandling string // auto|normal|exclude; empty resolves via manifest/auto
	SkipLibraryDocs     bool   // when true, caller (e.g. gitrepo) handles LibraryDocs itself
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
	only := map[string]struct{}{}
	for _, p := range opt.OnlyRelativePaths {
		only[filepath.ToSlash(p)] = struct{}{}
	}

	handling := librarydocs.NormalizeHandling(opt.LibraryDocsHandling)
	if strings.TrimSpace(opt.LibraryDocsHandling) == "" && !opt.SkipLibraryDocs {
		if m, err := manifest.LoadFromDir(absRoot); err == nil && m != nil && strings.TrimSpace(m.LibraryDocsHandling) != "" {
			handling = librarydocs.NormalizeHandling(m.LibraryDocsHandling)
		} else {
			handling = librarydocs.HandlingAuto
		}
	}
	excPatterns := append([]string{}, opt.ExcludePatterns...)
	if !opt.SkipLibraryDocs && handling == librarydocs.HandlingExclude {
		excPatterns = append(excPatterns, "LibraryDocs", "LibraryDocs/**")
	}

	allow := opt.PathFilter
	if allow == nil && (len(opt.IncludePatterns) > 0 || len(excPatterns) > 0) {
		inc, exc := opt.IncludePatterns, excPatterns
		allow = func(rel string) bool {
			return pathAllowed(rel, inc, exc)
		}
	}

	err = filepath.WalkDir(absRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // symlink policy: ignore
		}
		if d.IsDir() {
			if p != absRoot {
				if opt.SkipDirNames != nil {
					if _, ok := opt.SkipDirNames[d.Name()]; ok {
						return fs.SkipDir
					}
				}
				if ShouldIgnoreDir(d.Name()) {
					// Still skip .git always; vendor/node_modules skip unless PathFilter allows later files
					if d.Name() == ".git" {
						return fs.SkipDir
					}
					if allow == nil {
						return fs.SkipDir
					}
				}
			}
			return nil
		}
		rel, err := relativePathWithin(absRoot, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if len(only) > 0 {
			if _, ok := only[rel]; !ok {
				return nil
			}
		}
		if allow != nil {
			if !allow(rel) {
				return nil
			}
		} else if ShouldIgnoreFile(d.Name()) || !IsTextExt(d.Name()) {
			return nil
		}
		if ShouldSkipHelpPath(p) {
			return nil
		}
		filesSeen++
		if filesSeen > maxFiles {
			return fmt.Errorf("ingest file limit exceeded (%d)", maxFiles)
		}
		if err := ingestProjectFile(ctx, st, absRoot, rootName, p, rel, maxBytes, opt, res); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
		}
		if opt.Progress != nil {
			opt.Progress(filesSeen, 0, res.BytesProcessed, rel, "index")
		}
		if opt.MaxTotalBytes > 0 && res.BytesProcessed > opt.MaxTotalBytes {
			return fmt.Errorf("ingest total bytes limit exceeded (%d)", opt.MaxTotalBytes)
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	if !opt.SkipLibraryDocs {
		applyProjectLibraryDocs(ctx, st, absRoot, rootName, opt, res)
	}
	return res, nil
}

func applyProjectLibraryDocs(ctx context.Context, st *store.Store, absRoot, rootName string, opt ProjectOptions, res *ProjectResult) {
	handling := librarydocs.NormalizeHandling(opt.LibraryDocsHandling)
	if strings.TrimSpace(opt.LibraryDocsHandling) == "" {
		if m, err := manifest.LoadFromDir(absRoot); err == nil && m != nil && strings.TrimSpace(m.LibraryDocsHandling) != "" {
			handling = librarydocs.NormalizeHandling(m.LibraryDocsHandling)
		} else {
			handling = librarydocs.HandlingAuto
		}
	}
	scheme := "project"
	if strings.EqualFold(opt.URIScheme, "git") {
		scheme = "git"
	}
	meta := librarydocs.AnalyzeCheckout(absRoot, rootName, handling, "")
	if handling == librarydocs.HandlingExclude || meta.PackageState == librarydocs.StateNotPresent {
		_ = librarydocs.DeleteMeta(ctx, st, scheme, rootName)
	} else {
		if _, err := librarydocs.PersistMeta(ctx, st, scheme, rootName, meta); err != nil {
			res.Errors = append(res.Errors, "librarydocs meta: "+err.Error())
		}
		if handling == librarydocs.HandlingAuto {
			_ = librarydocs.ApplyTrustUpdates(ctx, st, scheme, rootName, meta)
		}
	}
	res.Errors = append(res.Errors, meta.Warnings...)
	sum := meta.Summary
	res.LibraryDocs = &sum
}

func ingestProjectFile(ctx context.Context, st *store.Store, absRoot, rootName, path, rel string, maxBytes int64, opt ProjectOptions, res *ProjectResult) error {
	data, info, err := readTextFileLimited(path, maxBytes)
	if err != nil {
		return err
	}
	res.BytesProcessed += int64(len(data))

	var uri string
	switch strings.ToLower(opt.URIScheme) {
	case "git":
		uri = GitURI(rootName, rel)
	default:
		uri = ProjectURI(rootName, rel)
	}
	hash := sha256Hex(data)

	existing, err := st.GetHashByURI(ctx, uri)
	if err != nil {
		return err
	}
	if existing == hash {
		res.Skipped++
		res.Files = append(res.Files, IngestedFile{
			URI: uri, RelativePath: rel, ContentHash: hash, Written: false, FileSize: int64(len(data)),
		})
		return nil
	}

	sourceType := store.SourceSource
	if opt.SourceType != "" {
		sourceType = opt.SourceType
	}
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
		if opt.SourceType == "" {
			sourceType = store.SourceMarkdown
		}
		content = doc.Markdown
		if doc.Title != "" {
			title = doc.Title
		}
		chunks = ChunkMarkdown(content)
	case IsMarkdownExt(path):
		if opt.SourceType == "" {
			sourceType = store.SourceMarkdown
		}
		chunks = ChunkMarkdown(content)
	default:
		chunks = ChunkSource(content)
	}

	authority := opt.Authority
	if authority == "" {
		authority = InferAuthority(rootName, rel)
	}

	syms := ExtractSymbols(path, content)
	written, err := st.UpsertDocument(ctx, store.UpsertInput{
		URI:            uri,
		Title:          title,
		SourceType:     sourceType,
		Path:           rel,
		RootName:       rootName,
		Authority:      authority,
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
	f := IngestedFile{
		URI: uri, RelativePath: rel, ContentHash: hash,
		Language: languageFromPath(path), SourceType: sourceType,
		FileSize: int64(len(data)), Written: written,
	}
	res.Files = append(res.Files, f)
	if written {
		res.Ingested++
		res.URIs = append(res.URIs, uri)
	} else {
		res.Skipped++
	}
	return nil
}

func pathAllowed(rel string, include, exclude []string) bool {
	rel = filepath.ToSlash(rel)
	if len(include) > 0 {
		ok := false
		for _, p := range include {
			if matchGlob(p, rel) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, p := range exclude {
		if matchGlob(p, rel) {
			return false
		}
	}
	return true
}

func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	if pattern == "**" || pattern == "**/*" {
		return true
	}
	if strings.HasPrefix(pattern, "**/") {
		suf := pattern[3:]
		if matchGlob(suf, name) {
			return true
		}
		for i := 0; i < len(name); i++ {
			if name[i] == '/' && matchGlob(suf, name[i+1:]) {
				return true
			}
		}
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		pre := strings.TrimSuffix(pattern, "/**")
		return name == pre || strings.HasPrefix(name, pre+"/")
	}
	ok, err := path.Match(pattern, name)
	if err == nil && ok {
		return true
	}
	ok, err = path.Match(pattern, path.Base(name))
	return err == nil && ok
}

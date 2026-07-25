// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"path/filepath"
	"strings"
)

var ignoredDirNames = map[string]struct{}{
	".git":         {},
	".hg":          {},
	".svn":         {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"target":       {},
	"__pycache__":  {},
	".venv":        {},
	".idea":        {},
	".vscode":      {},
	"mft":          {}, // mobile help-framework assets
}

var ignoredExts = map[string]struct{}{
	".exe":   {},
	".dll":   {},
	".so":    {},
	".dylib": {},
	".o":     {},
	".a":     {},
	".png":   {},
	".jpg":   {},
	".jpeg":  {},
	".gif":   {},
	".webp":  {},
	".pdf":   {},
	".zip":   {},
	".tar":   {},
	".gz":    {},
	".7z":    {},
	".rar":   {},
	".bin":   {},
	".wasm":  {},
	".class": {},
	".jar":   {},
	".pyc":   {},
}

// ShouldIgnoreDir reports whether a directory name should be skipped.
func ShouldIgnoreDir(name string) bool {
	_, ok := ignoredDirNames[name]
	return ok
}

// ShouldIgnoreFile reports whether a file should be skipped by extension.
func ShouldIgnoreFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := ignoredExts[ext]
	return ok
}

// IsTextExt reports whether the extension is a candidate for project ingest.
func IsTextExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".py", ".rs", ".c", ".h", ".cc", ".cpp", ".hpp", ".cs",
		".java", ".kt", ".swift", ".rb", ".php", ".scala",
		".md", ".mdx", ".txt", ".rst", ".toml", ".yaml", ".yml",
		".json", ".jsonc", ".xml", ".html", ".css", ".scss",
		".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd",
		".sql", ".proto", ".graphql", ".cmake", ".makefile",
		".gradle", ".sbt", ".lua", ".r", ".jl", ".zig",
		".mod", ".sum", ".env", ".gitignore", ".dockerignore":
		return true
	case "":
		// Extensionless: allow common names
		base := strings.ToLower(filepath.Base(name))
		switch base {
		case "makefile", "dockerfile", "gemfile", "rakefile", "procfile", "license", "readme":
			return true
		}
		return false
	default:
		return false
	}
}

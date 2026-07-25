// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"path/filepath"
	"strings"
)

// ClassifyPath assigns a content class for ranking metadata.
func ClassifyPath(rel string) string {
	p := filepath.ToSlash(strings.ToLower(rel))
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	switch {
	case strings.Contains(p, "/vendor/") || strings.HasPrefix(p, "vendor/") ||
		strings.Contains(p, "/third_party/") || strings.HasPrefix(p, "third_party/"):
		return "third_party"
	case strings.Contains(p, "/testdata/") || strings.HasPrefix(p, "testdata/") ||
		strings.Contains(p, "/fixtures/") || strings.HasPrefix(p, "fixtures/") ||
		strings.Contains(p, "/test/") || strings.HasPrefix(p, "test/") ||
		strings.Contains(p, "/tests/") || strings.HasPrefix(p, "tests/") ||
		strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		strings.HasPrefix(base, "test_"):
		return "test"
	case strings.Contains(p, "/example/") || strings.Contains(p, "/examples/") ||
		strings.HasPrefix(p, "example/") || strings.HasPrefix(p, "examples/") ||
		strings.Contains(p, "/sample/") || strings.Contains(p, "/samples/") ||
		strings.HasPrefix(p, "sample/") || strings.HasPrefix(p, "samples/"):
		return "example"
	case base == "license" || base == "licence" || strings.HasPrefix(base, "license.") ||
		strings.HasPrefix(base, "licence.") || base == "notice" || strings.HasPrefix(base, "notice.") ||
		base == "copying" || base == ".gitignore" || base == ".gitattributes" ||
		base == "authors" || base == "contributors":
		return "project_meta"
	case strings.Contains(p, "/docs/") || strings.Contains(p, "/doc/") ||
		ext == ".md" || ext == ".rst" || base == "readme" || strings.HasPrefix(base, "readme."):
		return "documentation"
	case base == "cmakelists.txt" || base == "makefile" || strings.HasPrefix(base, "dockerfile") ||
		ext == ".cmake" || ext == ".gradle" || base == "go.mod" || base == "go.sum" ||
		base == "package.json" || base == "cargo.toml" || base == "meson.build":
		return "build_file"
	case ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml" ||
		ext == ".ini" || ext == ".cfg" || ext == ".conf":
		return "configuration"
	case strings.Contains(p, "/generated/") || strings.Contains(p, ".pb.go") ||
		strings.HasSuffix(base, ".generated.go"):
		return "generated"
	case ext == ".h" || ext == ".hpp" || ext == ".hh" ||
		(strings.Contains(p, "/include/") && (ext == ".h" || ext == ".hpp")):
		return "public_header"
	case ext == ".c" || ext == ".cc" || ext == ".cpp" || ext == ".cxx" ||
		ext == ".go" || ext == ".rs" || ext == ".py" || ext == ".java" ||
		ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" ||
		ext == ".sql":
		return "source"
	default:
		return "unknown"
	}
}

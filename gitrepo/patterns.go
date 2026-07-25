// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package gitrepo

import (
	"path"
	"path/filepath"
	"strings"
)

// DefaultExcludePatterns from the Git ingestion PRD.
var DefaultExcludePatterns = []string{
	".git/**",
	"build/**",
	"dist/**",
	"out/**",
	"node_modules/**",
	"vendor/**",
	"third_party/**",
	"**/*.exe",
	"**/*.dll",
	"**/*.so",
	"**/*.a",
	"**/*.lib",
	"**/*.bin",
	"**/*.zip",
	"**/*.tar",
	"**/*.gz",
	"**/*.png",
	"**/*.jpg",
	"**/*.jpeg",
	"**/*.gif",
	"**/*.mp4",
}

// MatchGlob supports * and ** against slash-separated paths.
func MatchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "./")
	if pattern == "**" || pattern == "**/*" {
		return true
	}
	if strings.HasPrefix(pattern, "**/") {
		suf := pattern[3:]
		if MatchGlob(suf, name) {
			return true
		}
		for i := 0; i < len(name); i++ {
			if name[i] == '/' && MatchGlob(suf, name[i+1:]) {
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
	return err == nil && ok
}

// PathAllowed applies include then exclude. Empty include = all text candidates.
// Explicit include matches win over default exclude patterns (e.g. vendor/**).
func PathAllowed(rel string, include, exclude []string) bool {
	rel = filepath.ToSlash(rel)
	matchedInclude := false
	if len(include) > 0 {
		for _, p := range include {
			if MatchGlob(p, rel) {
				matchedInclude = true
				break
			}
		}
		if !matchedInclude {
			return false
		}
	}
	ex := exclude
	if len(ex) == 0 {
		if matchedInclude {
			ex = []string{".git/**"}
		} else {
			ex = DefaultExcludePatterns
		}
	}
	for _, p := range ex {
		if MatchGlob(p, rel) {
			return false
		}
	}
	return true
}

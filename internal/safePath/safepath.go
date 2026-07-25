package safePath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveUnderRoot resolves relPath under rootAbs and ensures the result stays
// inside rootAbs. Caller paths must be relative (no drive letters, UNC, or abs).
func ResolveUnderRoot(rootAbs, relPath string) (string, error) {
	rootAbs = filepath.Clean(rootAbs)
	if !filepath.IsAbs(rootAbs) {
		var err error
		rootAbs, err = filepath.Abs(rootAbs)
		if err != nil {
			return "", err
		}
		rootAbs = filepath.Clean(rootAbs)
	}

	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, `\`) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	// Windows drive / UNC
	if len(relPath) >= 2 && relPath[1] == ':' {
		return "", fmt.Errorf("drive-letter paths are not allowed")
	}
	if strings.HasPrefix(relPath, `\\`) || strings.HasPrefix(relPath, "//") {
		return "", fmt.Errorf("UNC paths are not allowed")
	}

	cleaned := filepath.Clean(relPath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes output root")
	}

	full := filepath.Clean(filepath.Join(rootAbs, cleaned))
	rel, err := filepath.Rel(rootAbs, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes output root")
	}
	return full, nil
}

// ContainPath reports whether absPath is inside rootAbs (after Clean).
func ContainPath(rootAbs, absPath string) bool {
	rootAbs = filepath.Clean(rootAbs)
	absPath = filepath.Clean(absPath)
	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// EvalAndContain resolves symlinks for path and root when possible and checks
// the final target remains under rootAbs.
func EvalAndContain(rootAbs, absPath string) (string, error) {
	rootEval, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		// root may not exist yet
		rootEval = filepath.Clean(rootAbs)
	}
	// If the file does not exist yet, evaluate the parent.
	target := absPath
	if _, err := os.Lstat(absPath); err != nil {
		parent := filepath.Dir(absPath)
		if parentEval, err2 := filepath.EvalSymlinks(parent); err2 == nil {
			target = filepath.Join(parentEval, filepath.Base(absPath))
		}
	} else if eval, err := filepath.EvalSymlinks(absPath); err == nil {
		target = eval
	}
	if !ContainPath(rootEval, target) {
		return "", fmt.Errorf("path escapes output root after symlink resolution")
	}
	return filepath.Clean(target), nil
}

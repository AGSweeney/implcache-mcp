// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package librarydocs

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationRelativePath is the conventional validation report.
const ValidationRelativePath = "LibraryDocs/VALIDATION.md"

// ParseValidationFile reads VALIDATION.md.
func ParseValidationFile(checkout string) (*ValidationInfo, []string, error) {
	p := filepath.Join(checkout, filepath.FromSlash(ValidationRelativePath))
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return ParseValidationMarkdown(string(data))
}

// ParseValidationMarkdown extracts validation result from frontmatter or body.
func ParseValidationMarkdown(md string) (*ValidationInfo, []string, error) {
	var warns []string
	fm, body, warn := SplitFrontmatter(md)
	if warn != "" {
		warns = append(warns, ValidationRelativePath+": "+warn)
	}
	info := &ValidationInfo{}
	if fm.RawPresent {
		// reuse unknown map via re-parse for validation-specific keys
		info = parseValidationYAMLBlock(md)
	}
	if info.Result == "" {
		info.Result = scanResultInText(body)
		if info.Result == "" {
			info.Result = scanResultInText(md)
		}
	}
	if info.Result != "" && info.Result != "pass" && info.Result != "fail" {
		warns = append(warns, ValidationRelativePath+": unknown validation result "+info.Result)
	}
	if info.Summary == "" {
		info.Summary = firstParagraph(body)
	}
	return info, warns, nil
}

func parseValidationYAMLBlock(md string) *ValidationInfo {
	fm, _, _ := SplitFrontmatter(md)
	info := &ValidationInfo{}
	if !fm.RawPresent {
		// try whole-file yaml-ish keys
		return scanValidationKeys(md)
	}
	// Re-unmarshal known validation fields from the frontmatter segment
	s := strings.TrimLeft(md, "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return info
	}
	rest := s[3:]
	rest = strings.TrimPrefix(rest, "\r")
	rest = strings.TrimPrefix(rest, "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return info
	}
	var m map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &m); err != nil {
		return info
	}
	info.Result = strings.ToLower(asString(m["result"]))
	info.Date = asString(m["date"])
	info.Validator = asString(m["validator"])
	info.StandardVersion = firstNonEmpty(asString(m["standard_version"]), asString(m["standardVersion"]))
	info.Summary = asString(m["summary"])
	return info
}

func scanValidationKeys(md string) *ValidationInfo {
	info := &ValidationInfo{}
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "result:"):
			info.Result = strings.ToLower(strings.TrimSpace(line[len("result:"):]))
		case strings.HasPrefix(lower, "date:"):
			info.Date = strings.TrimSpace(line[len("date:"):])
		case strings.HasPrefix(lower, "validator:"):
			info.Validator = strings.TrimSpace(line[len("validator:"):])
		case strings.HasPrefix(lower, "standard_version:") || strings.HasPrefix(lower, "standard version:"):
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.StandardVersion = strings.TrimSpace(parts[1])
			}
		}
	}
	return info
}

func scanResultInText(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "result: pass") || strings.Contains(lower, "result:pass"):
		return "pass"
	case strings.Contains(lower, "result: fail") || strings.Contains(lower, "result:fail"):
		return "fail"
	case strings.Contains(lower, "**pass**") && strings.Contains(lower, "result"):
		return "pass"
	case strings.Contains(lower, "**fail**") && strings.Contains(lower, "result"):
		return "fail"
	default:
		return ""
	}
}

func firstParagraph(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "\n\n")
	p := strings.TrimSpace(parts[0])
	if len(p) > 400 {
		return p[:400] + "…"
	}
	return p
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case nil:
		return ""
	default:
		return strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.TrimSpace(toStringish(t)), "\n", " "), `"`))
	}
}

func toStringish(v any) string {
	b, _ := yaml.Marshal(v)
	return string(b)
}

// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"regexp"
	"strings"
)

// Conservative regex extractors for Python, JavaScript/TypeScript, and Java.
// Intentionally shallow — no parser dependencies.

var (
	rePyDef   = regexp.MustCompile(`(?m)^[ \t]*(?:async\s+)?def\s+([A-Za-z_][\w]*)\s*\(`)
	rePyClass = regexp.MustCompile(`(?m)^[ \t]*class\s+([A-Za-z_][\w]*)\s*[:(]`)

	reJSFunc     = regexp.MustCompile(`(?m)^[ \t]*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)\s*\(`)
	reJSClass    = regexp.MustCompile(`(?m)^[ \t]*(?:export\s+)?class\s+([A-Za-z_$][\w$]*)\b`)
	reJSMethod   = regexp.MustCompile(`(?m)^[ \t]*(?:async\s+)?([A-Za-z_$][\w$]*)\s*\([^;]*\)\s*\{`)
	reJSArrow    = regexp.MustCompile(`(?m)^[ \t]*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>`)
	reJSAssignFn = regexp.MustCompile(`(?m)^[ \t]*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s+)?function\b`)

	reJavaType   = regexp.MustCompile(`(?m)^[ \t]*(?:public|protected|private|abstract|final|static|\s)*(?:class|interface|enum|record)\s+([A-Za-z_][\w]*)\b`)
	reJavaMethod = regexp.MustCompile(`(?m)^[ \t]*(?:public|protected|private|static|final|synchronized|native|abstract|default|\s)+[\w.<>,\[\]\s]+\s+([A-Za-z_][\w]*)\s*\([^;]*\)\s*(?:throws\s+[\w.,\s]+)?\s*\{`)
	reJavaDecl   = regexp.MustCompile(`(?m)^[ \t]*(?:public|protected|private|static|final|default|\s)+[\w.<>,\[\]\s]+\s+([A-Za-z_][\w]*)\s*\([^;]*\)\s*(?:throws\s+[\w.,\s]+)?;`)
)

func extractPython(body string, lines []string, add func(name, kind, sig string, line int)) {
	for _, m := range rePyClass.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindType, strings.TrimSpace(lineAt(lines, line)), line)
	}
	for _, m := range rePyDef.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		if isNoiseIdent(name) || isPyNoise(name) {
			continue
		}
		line := 1 + strings.Count(body[:m[2]], "\n")
		sig := strings.TrimSpace(lineAt(lines, line))
		kind := KindFunction
		// Indented defs are treated as methods.
		if strings.HasPrefix(lineAt(lines, line), " ") || strings.HasPrefix(lineAt(lines, line), "\t") {
			kind = KindMethod
		}
		add(name, kind, sig, line)
	}
}

func extractJavaScript(body string, lines []string, add func(name, kind, sig string, line int)) {
	seen := map[string]struct{}{}
	mark := func(name string) { seen[strings.ToLower(name)] = struct{}{} }
	has := func(name string) bool { _, ok := seen[strings.ToLower(name)]; return ok }

	for _, m := range reJSClass.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindType, strings.TrimSpace(lineAt(lines, line)), line)
		mark(name)
	}
	for _, re := range []*regexp.Regexp{reJSFunc, reJSArrow, reJSAssignFn} {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			if isNoiseIdent(name) || isJSNoise(name) || has(name) {
				continue
			}
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, KindFunction, strings.TrimSpace(lineAt(lines, line)), line)
			mark(name)
		}
	}
	for _, m := range reJSMethod.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		if isNoiseIdent(name) || isJSNoise(name) || has(name) {
			continue
		}
		line := 1 + strings.Count(body[:m[2]], "\n")
		// Skip likely control-flow lookalikes at column 0 without class context heuristics.
		if isJSControl(name) {
			continue
		}
		add(name, KindMethod, strings.TrimSpace(lineAt(lines, line)), line)
		mark(name)
	}
}

func extractJava(body string, lines []string, add func(name, kind, sig string, line int)) {
	seen := map[string]struct{}{}
	mark := func(name string) { seen[name] = struct{}{} }
	has := func(name string) bool { _, ok := seen[name]; return ok }

	for _, m := range reJavaType.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindType, strings.TrimSpace(lineAt(lines, line)), line)
		mark(name)
	}
	for _, m := range reJavaMethod.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		if isNoiseIdent(name) || isJavaNoise(name) || has(name) {
			continue
		}
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindMethod, strings.TrimSpace(lineAt(lines, line)), line)
		mark(name)
	}
	for _, m := range reJavaDecl.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		if isNoiseIdent(name) || isJavaNoise(name) || has(name) {
			continue
		}
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindDeclaration, strings.TrimSpace(lineAt(lines, line)), line)
		mark(name)
	}
}

func isPyNoise(name string) bool {
	switch name {
	case "__init__", "__str__", "__repr__", "__enter__", "__exit__":
		return false // keep dunders that are real APIs
	case "print", "len", "range", "open", "super":
		return true
	}
	return false
}

func isJSNoise(name string) bool {
	switch name {
	case "constructor", "require", "include", "define":
		return true
	}
	return isJSControl(name)
}

func isJSControl(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "function", "class", "return", "get", "set":
		return true
	}
	return false
}

func isJavaNoise(name string) bool {
	switch name {
	case "if", "for", "while", "switch", "catch", "return", "new", "class", "interface",
		"enum", "record", "void", "int", "long", "boolean", "String":
		return true
	}
	return false
}

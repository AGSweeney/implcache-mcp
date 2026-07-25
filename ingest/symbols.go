// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ingest

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"implcache-mcp/store"
)

// Symbol kinds — definitions/declarations outrank ordinary call-site references.
const (
	KindFunction    = "function"
	KindDeclaration = "declaration"
	KindMethod      = "method"
	KindType        = "type"
	KindMacro       = "macro"
	KindConstant    = "constant"
	KindCall        = "call"
)

var (
	reGoFunc = regexp.MustCompile(`(?m)^func\s+(?:\(([^)]*)\)\s+)?([A-Za-z_][\w]*)\s*\(`)

	// C/C++ definitions (body opens with {). Angle brackets normalized before match.
	reCDef = regexp.MustCompile(`(?m)^(?:static\s+|extern\s+|inline\s+|constexpr\s+|virtual\s+|typename\s+)*[\w:<>,\s\*\&]+\s+([A-Za-z_][\w:]*)\s*\([^;]*\)\s*(?:const\s*|noexcept\s*)?\{`)

	// C/C++ declarations / prototypes ending with ; (require a return-type token so
	// call statements like `ns::Helper();` are not treated as prototypes).
	reCDecl = regexp.MustCompile(`(?m)^(?:static\s+|extern\s+|inline\s+|constexpr\s+|virtual\s+)*(?:[A-Za-z_][\w:<>,\*\&]*\s+)+([A-Za-z_][\w:]*)\s*\([^;]*\)\s*(?:const\s*|noexcept\s*)?;`)

	// C++ scoped method definitions: Type::Method(...) {
	reCppMethodDef = regexp.MustCompile(`(?m)^(?:[\w:<>,\s\*\&]+\s+)?([A-Za-z_][\w]*::[A-Za-z_][\w]*)\s*\([^;{]*\)\s*(?:const\s*|noexcept\s*)?\{`)

	// template<...> class/struct/using and template functions (after angle normalization).
	reTemplateType = regexp.MustCompile(`(?m)^template\s*<>\s*(?:class|struct|enum(?:\s+class)?|union)\s+([A-Za-z_][\w]*)`)
	reType         = regexp.MustCompile(`(?m)^(?:(?:template\s*<>\s*)?(?:class|struct|enum(?:\s+class)?|union|interface))\s+([A-Za-z_][\w]*)`)
	reUsingAlias   = regexp.MustCompile(`(?m)^(?:using|typedef)\s+(?:[\w:<>,\s\*\&]+\s+)?([A-Za-z_][\w]*)\s*(?:=|;)`)

	// Object-like and function-like macros.
	reMacro     = regexp.MustCompile(`(?m)^#\s*define\s+([A-Za-z_][\w]*)\b`)
	reMacroFunc = regexp.MustCompile(`(?m)^#\s*define\s+([A-Za-z_][\w]*)\s*\(`)

	reConst = regexp.MustCompile(`(?m)^(?:static\s+|constexpr\s+|const\s+)*const(?:expr)?\s+[\w:<>,\s\*\&]+\s+([A-Z][A-Z0-9_]{2,})\s*=`)

	// Qualified / member call sites (lower priority).
	reQualifiedCall = regexp.MustCompile(`\b([A-Za-z_][\w]*::[A-Za-z_][\w]*)\s*\(`)
	reMemberCall    = regexp.MustCompile(`\b([A-Za-z_][\w]*\.[A-Za-z_][\w]*)\s*\(`)
	rePascalCall    = regexp.MustCompile(`\b([A-Z][A-Za-z0-9]{2,}[A-Z][A-Za-z0-9]*)\s*\(`)

	reInclude = regexp.MustCompile(`(?m)^#\s*include\s*[<"]([^>"]+)[>"]`)
)

var kindRank = map[string]int{
	KindFunction:    6,
	KindMethod:      5,
	KindDeclaration: 4,
	KindType:        3,
	KindMacro:       3,
	KindConstant:    2,
	KindCall:        1,
}

// ExtractSymbols performs pragmatic symbol extraction for common languages.
// Definitions and declarations are preferred over call-site references when deduping.
func ExtractSymbols(path string, body string) []store.SymbolInput {
	lang := languageFromPath(path)
	var out []store.SymbolInput
	origLines := strings.Split(body, "\n")
	// Normalize template angle brackets so nested templates match reliably.
	normBody := normalizeTemplateAngles(body)
	normLines := strings.Split(normBody, "\n")

	add := func(name, kind, sig string, line int) {
		name = strings.TrimSpace(name)
		if name == "" || len(name) < 2 || isNoiseIdent(name) {
			return
		}
		out = append(out, store.SymbolInput{
			Name: name, Kind: kind, Language: lang, Signature: sig, StartLine: line, EndLine: line,
		})
	}

	switch lang {
	case "go":
		for _, m := range reGoFunc.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[4]:m[5]]
			line := 1 + strings.Count(body[:m[4]], "\n")
			kind := KindFunction
			if m[2] != -1 && strings.TrimSpace(body[m[2]:m[3]]) != "" {
				kind = KindMethod
			}
			add(name, kind, strings.TrimSpace(lineAt(origLines, line)), line)
		}
	case "c", "cpp", "csharp":
		extractCFamily(normBody, origLines, normLines, lang, add)
	case "python":
		extractPython(body, origLines, add)
	case "javascript", "typescript":
		extractJavaScript(body, origLines, add)
	case "java":
		extractJava(body, origLines, add)
	default:
		// Unknown / unsupported languages: do not run C-family regex (avoids
		// false symbols from Markdown, config, scripts, etc.).
		return nil
	}

	if len(out) > 80 {
		out = preferHigherKinds(out, 80)
	}
	return dedupeSymbols(out)
}

// normalizeTemplateAngles replaces balanced <...> template argument lists with <>
// so regexes can match template functions/types without nested-angle complexity.
func normalizeTemplateAngles(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	depth := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if depth == 0 {
			if ch == '<' && i > 0 {
				prev := s[i-1]
				if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
					prev == '_' || prev == '>' || (prev >= '0' && prev <= '9') {
					depth = 1
					b.WriteString("<>")
					continue
				}
			}
			b.WriteByte(ch)
			continue
		}
		switch ch {
		case '<':
			depth++
		case '>':
			depth--
		}
	}
	return b.String()
}

func extractCFamily(body string, origLines, normLines []string, lang string, add func(name, kind, sig string, line int)) {
	seenDef := map[string]struct{}{}
	mark := func(name string) { seenDef[store.NormalizeSymbol(name)] = struct{}{} }
	hasDef := func(name string) bool {
		_, ok := seenDef[store.NormalizeSymbol(name)]
		return ok
	}
	sigAt := func(line int) string {
		s := strings.TrimSpace(lineAt(origLines, line))
		if s == "" {
			s = strings.TrimSpace(lineAt(normLines, line))
		}
		return s
	}

	if lang == "cpp" || lang == "c" || lang == "csharp" || lang == "" {
		for _, m := range reCppMethodDef.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, KindMethod, sigAt(line), line)
			mark(name)
			mark(store.UnqualifiedSymbol(name))
		}
		for _, m := range reCDef.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			kind := KindFunction
			if strings.Contains(name, "::") {
				kind = KindMethod
			}
			add(name, kind, sigAt(line), line)
			mark(name)
			mark(store.UnqualifiedSymbol(name))
		}
		for _, m := range reCDecl.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			if hasDef(name) || hasDef(store.UnqualifiedSymbol(name)) {
				continue
			}
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, KindDeclaration, sigAt(line), line)
			mark(name)
		}
	}

	for _, m := range reTemplateType.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindType, sigAt(line), line)
	}
	for _, m := range reType.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindType, sigAt(line), line)
	}
	for _, m := range reUsingAlias.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindType, sigAt(line), line)
	}
	for _, m := range reMacroFunc.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindMacro, sigAt(line), line)
	}
	for _, m := range reMacro.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindMacro, sigAt(line), line)
	}
	for _, m := range reConst.FindAllStringSubmatchIndex(body, -1) {
		name := body[m[2]:m[3]]
		line := 1 + strings.Count(body[:m[2]], "\n")
		add(name, KindConstant, sigAt(line), line)
	}

	addCalls := func(re *regexp.Regexp, forceKind string) {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			if hasDef(name) || hasDef(store.UnqualifiedSymbol(name)) {
				continue
			}
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, forceKind, name+"()", line)
		}
	}
	addCalls(reQualifiedCall, KindCall)
	addCalls(reMemberCall, KindCall)
	addCalls(rePascalCall, KindCall)
}

func isNoiseIdent(name string) bool {
	switch strings.ToLower(store.UnqualifiedSymbol(name)) {
	case "if", "for", "while", "switch", "return", "sizeof", "typeof", "new", "delete",
		"main", "printf", "sprintf", "malloc", "free", "memcpy", "memset", "assert",
		"operator", "template", "typename", "constexpr", "noexcept":
		return true
	}
	return false
}

func preferHigherKinds(in []store.SymbolInput, limit int) []store.SymbolInput {
	// Stable: keep earlier higher-kind items by sorting copy then truncating by rank+order.
	type item struct {
		s store.SymbolInput
		i int
	}
	tmp := make([]item, len(in))
	for i, s := range in {
		tmp[i] = item{s, i}
	}
	for i := 0; i < len(tmp); i++ {
		for j := i + 1; j < len(tmp); j++ {
			ri, rj := kindRank[tmp[i].s.Kind], kindRank[tmp[j].s.Kind]
			if rj > ri || (rj == ri && tmp[j].i < tmp[i].i) {
				tmp[i], tmp[j] = tmp[j], tmp[i]
			}
		}
	}
	out := make([]store.SymbolInput, 0, limit)
	for _, it := range tmp {
		out = append(out, it.s)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func languageFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".c", ".h":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".java":
		return "java"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	default:
		return ""
	}
}

func lineAt(lines []string, line int) string {
	if line <= 0 || line > len(lines) {
		return ""
	}
	return lines[line-1]
}

func dedupeSymbols(in []store.SymbolInput) []store.SymbolInput {
	best := map[string]store.SymbolInput{}
	order := []string{}
	for _, s := range in {
		key := store.NormalizeSymbol(s.Name)
		if key == "" {
			continue
		}
		prev, ok := best[key]
		if !ok {
			best[key] = s
			order = append(order, key)
			continue
		}
		if kindRank[s.Kind] > kindRank[prev.Kind] {
			best[key] = s
		}
	}
	out := make([]store.SymbolInput, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

// ExtractIncludes returns #include lines from a body (shared with implctx/vomit callers via ingest).
func ExtractIncludes(body string) []string {
	var out []string
	for _, m := range reInclude.FindAllStringSubmatch(body, -1) {
		out = append(out, "#include \""+m[1]+"\"")
	}
	return out
}

// LooksLikeAPIToken reports whether a token is a plausible API/symbol name.
func LooksLikeAPIToken(s string) bool {
	s = strings.Trim(s, "`,.;()<>\"'")
	if len(s) < 3 {
		return false
	}
	if strings.Contains(s, "::") || strings.Contains(s, ".") {
		return true
	}
	hasU, hasL := false, false
	for _, r := range s {
		if unicode.IsUpper(r) {
			hasU = true
		}
		if unicode.IsLower(r) {
			hasL = true
		}
	}
	return (hasU && hasL) || strings.Contains(s, "_")
}

// InferAuthority picks a default authority from root/path heuristics.
func InferAuthority(rootName, relPath string) string {
	r := strings.ToLower(rootName + " " + relPath)
	switch {
	case strings.Contains(r, "recipe") || strings.Contains(r, "curated"):
		return store.AuthorityCuratedRecipe
	case strings.Contains(r, "sample") || strings.Contains(r, "example"):
		return store.AuthorityOfficialExample
	case strings.Contains(r, "help") || strings.Contains(r, "doc") || strings.Contains(r, "api/dita"):
		return store.AuthorityOfficialDocs
	case strings.Contains(r, "testdata") || strings.HasSuffix(relPath, ".go") || strings.Contains(r, "/src/"):
		return store.AuthorityRelatedProject
	default:
		return store.AuthorityUnknown
	}
}

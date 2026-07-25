package ingest

import (
	"path/filepath"
	"regexp"
	"strings"

	"implcache-mcp/store"
)

var (
	reGoFunc    = regexp.MustCompile(`(?m)^func\s+(?:\([^)]+\)\s+)?([A-Z][A-Za-z0-9_]*)\s*\(`)
	reCFunc     = regexp.MustCompile(`(?m)^(?:static\s+|extern\s+|inline\s+)*[A-Za-z_][\w\s\*]+\s+([A-Za-z_][\w]*)\s*\([^;]*\)\s*\{`)
	reProAPI    = regexp.MustCompile(`\b(Pro[A-Z][A-Za-z0-9_]{3,})\s*\(`)
	reInclude   = regexp.MustCompile(`(?m)^#\s*include\s*[<"]([^>"]+)[>"]`)
	reClassLike = regexp.MustCompile(`(?m)^(?:class|struct|interface)\s+([A-Za-z_][\w]*)`)
)

// ExtractSymbols performs pragmatic symbol extraction for common languages.
func ExtractSymbols(path string, body string) []store.SymbolInput {
	lang := languageFromPath(path)
	var out []store.SymbolInput
	lines := strings.Split(body, "\n")

	add := func(name, kind, sig string, line int) {
		name = strings.TrimSpace(name)
		if name == "" || len(name) < 2 {
			return
		}
		out = append(out, store.SymbolInput{
			Name: name, Kind: kind, Language: lang, Signature: sig, StartLine: line, EndLine: line,
		})
	}

	switch lang {
	case "go":
		for _, m := range reGoFunc.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, "function", strings.TrimSpace(lineAt(lines, line)), line)
		}
	case "c", "cpp":
		for _, m := range reCFunc.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, "function", strings.TrimSpace(lineAt(lines, line)), line)
		}
		for _, m := range reProAPI.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, "api", name+"()", line)
		}
		for _, m := range reClassLike.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, "type", strings.TrimSpace(lineAt(lines, line)), line)
		}
	default:
		for _, m := range reProAPI.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			line := 1 + strings.Count(body[:m[2]], "\n")
			add(name, "api", name+"()", line)
		}
	}

	// Cap per file to keep ingest cheap.
	if len(out) > 80 {
		out = out[:80]
	}
	return dedupeSymbols(out)
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
	seen := map[string]struct{}{}
	var out []store.SymbolInput
	for _, s := range in {
		key := store.NormalizeSymbol(s.Name) + "|" + s.Kind
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
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

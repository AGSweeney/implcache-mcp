import { useMemo } from "react";
import hljs from "highlight.js/lib/core";
import cpp from "highlight.js/lib/languages/cpp";
import c from "highlight.js/lib/languages/c";
import javascript from "highlight.js/lib/languages/javascript";
import typescript from "highlight.js/lib/languages/typescript";
import python from "highlight.js/lib/languages/python";
import xml from "highlight.js/lib/languages/xml";
import markdown from "highlight.js/lib/languages/markdown";
import json from "highlight.js/lib/languages/json";
import bash from "highlight.js/lib/languages/bash";
import css from "highlight.js/lib/languages/css";
import go from "highlight.js/lib/languages/go";
import rust from "highlight.js/lib/languages/rust";
import java from "highlight.js/lib/languages/java";
import plaintext from "highlight.js/lib/languages/plaintext";

let registered = false;
function ensureLangs() {
  if (registered) return;
  hljs.registerLanguage("cpp", cpp);
  hljs.registerLanguage("c", c);
  hljs.registerLanguage("javascript", javascript);
  hljs.registerLanguage("typescript", typescript);
  hljs.registerLanguage("python", python);
  hljs.registerLanguage("xml", xml);
  hljs.registerLanguage("html", xml);
  hljs.registerLanguage("markdown", markdown);
  hljs.registerLanguage("json", json);
  hljs.registerLanguage("bash", bash);
  hljs.registerLanguage("shell", bash);
  hljs.registerLanguage("css", css);
  hljs.registerLanguage("go", go);
  hljs.registerLanguage("rust", rust);
  hljs.registerLanguage("java", java);
  hljs.registerLanguage("plaintext", plaintext);
  registered = true;
}

const EXT_LANG: Record<string, string> = {
  ".cpp": "cpp",
  ".cc": "cpp",
  ".cxx": "cpp",
  ".c": "c",
  ".h": "cpp",
  ".hpp": "cpp",
  ".hh": "cpp",
  ".js": "javascript",
  ".mjs": "javascript",
  ".cjs": "javascript",
  ".ts": "typescript",
  ".tsx": "typescript",
  ".jsx": "javascript",
  ".py": "python",
  ".md": "markdown",
  ".markdown": "markdown",
  ".json": "json",
  ".html": "html",
  ".htm": "html",
  ".xml": "xml",
  ".css": "css",
  ".go": "go",
  ".rs": "rust",
  ".java": "java",
  ".sh": "bash",
  ".bash": "bash",
};

export function detectLanguage(uri?: string, sourceType?: string, path?: string): string {
  const candidates = [uri, path].filter(Boolean) as string[];
  for (const s of candidates) {
    const base = s.split(/[?#]/)[0].toLowerCase();
    const m = base.match(/(\.[a-z0-9]+)$/);
    if (m && EXT_LANG[m[1]]) return EXT_LANG[m[1]];
  }
  switch ((sourceType || "").toLowerCase()) {
    case "markdown":
      return "markdown";
    case "web":
      return "html";
    case "source":
    case "git":
      return "cpp";
    case "pdf":
      return "plaintext";
    default:
      return "plaintext";
  }
}

export default function CodeBlock({
  code,
  language,
  className = "",
}: {
  code: string;
  language?: string;
  className?: string;
}) {
  const html = useMemo(() => {
    ensureLangs();
    const lang = language && hljs.getLanguage(language) ? language : "plaintext";
    try {
      return hljs.highlight(code, { language: lang, ignoreIllegals: true }).value;
    } catch {
      return hljs.highlight(code, { language: "plaintext" }).value;
    }
  }, [code, language]);

  return (
    <pre className={`doc-json doc-source hljs ${className}`.trim()}>
      <code className={`language-${language || "plaintext"}`} dangerouslySetInnerHTML={{ __html: html }} />
    </pre>
  );
}

import { useEffect, useMemo, useRef, useState } from "react";
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
import Button from "./Button";

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

function escapeHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

/** Highlight one logical source line so spans never cross row boundaries. */
function highlightLine(line: string, language?: string): string {
  ensureLangs();
  const lang = language && hljs.getLanguage(language) ? language : "plaintext";
  try {
    return hljs.highlight(line, { language: lang, ignoreIllegals: true }).value;
  } catch {
    return escapeHtml(line);
  }
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Wrap query hits in text nodes only (never inside HTML tags). */
function markQueryInHtml(html: string, query: string, current: boolean): string {
  const q = query.trim();
  if (!q) return html;
  const re = new RegExp(escapeRegExp(q), "gi");
  const markClass = current ? "code-mark is-current" : "code-mark";
  return html.replace(/(<[^>]+>)|([^<]+)/g, (_full, tag: string | undefined, text: string | undefined) => {
    if (tag) return tag;
    if (!text) return "";
    return text.replace(re, (hit) => `<mark class="${markClass}">${hit}</mark>`);
  });
}

async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    try {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.left = "-9999px";
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand("copy");
      document.body.removeChild(ta);
      return ok;
    } catch {
      return false;
    }
  }
}

/** Resolve selected line range inside a code block (1-based, inclusive). */
export function getSelectedLineRange(root: ParentNode | null): { start: number; end: number } | null {
  if (!root) return null;
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return null;
  const anchor = sel.anchorNode;
  const focus = sel.focusNode;
  if (!anchor || !focus || !root.contains(anchor) || !root.contains(focus)) return null;
  const a = (anchor instanceof Element ? anchor : anchor.parentElement)?.closest(".code-line") as HTMLElement | null;
  const b = (focus instanceof Element ? focus : focus.parentElement)?.closest(".code-line") as HTMLElement | null;
  if (!a || !b) return null;
  const n1 = Number(a.dataset.line);
  const n2 = Number(b.dataset.line);
  if (!n1 || !n2) return null;
  return { start: Math.min(n1, n2), end: Math.max(n1, n2) };
}

export default function CodeBlock({
  code,
  language,
  className = "",
  showSearch = true,
  showCopy = true,
  scrollToLine,
  onScrollToLineHandled,
}: {
  code: string;
  language?: string;
  className?: string;
  showSearch?: boolean;
  showCopy?: boolean;
  scrollToLine?: number | null;
  onScrollToLineHandled?: () => void;
}) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const [query, setQuery] = useState("");
  const [activeMatch, setActiveMatch] = useState(0);
  const [copyFlash, setCopyFlash] = useState<string | null>(null);
  const [wrap, setWrap] = useState(false);

  const plainLines = useMemo(() => code.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n"), [code]);

  const highlightedLines = useMemo(
    () => plainLines.map((line) => highlightLine(line, language)),
    [plainLines, language],
  );

  const matchIndexes = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return [] as number[];
    const hits: number[] = [];
    plainLines.forEach((line, i) => {
      if (line.toLowerCase().includes(q)) hits.push(i);
    });
    return hits;
  }, [plainLines, query]);

  useEffect(() => {
    setActiveMatch(0);
  }, [query, code]);

  useEffect(() => {
    if (!matchIndexes.length) return;
    const line = matchIndexes[Math.min(activeMatch, matchIndexes.length - 1)] + 1;
    const el = scrollerRef.current?.querySelector(`#L${line}`);
    el?.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [activeMatch, matchIndexes]);

  useEffect(() => {
    if (scrollToLine == null || scrollToLine < 1) return;
    const el = scrollerRef.current?.querySelector(`#L${scrollToLine}`);
    if (el) {
      el.scrollIntoView({ block: "center", behavior: "smooth" });
      el.classList.add("is-jump");
      window.setTimeout(() => el.classList.remove("is-jump"), 1600);
    }
    onScrollToLineHandled?.();
  }, [scrollToLine, code, onScrollToLineHandled]);

  const flash = (msg: string) => {
    setCopyFlash(msg);
    window.setTimeout(() => setCopyFlash(null), 1500);
  };

  const copyAll = async () => {
    flash((await copyText(code)) ? "Copied all" : "Copy failed");
  };

  const copySelection = async () => {
    const sel = window.getSelection()?.toString() || "";
    if (!sel.trim()) {
      flash("No selection");
      return;
    }
    flash((await copyText(sel)) ? "Copied selection" : "Copy failed");
  };

  const goMatch = (dir: 1 | -1) => {
    if (!matchIndexes.length) return;
    setActiveMatch((i) => (i + dir + matchIndexes.length) % matchIndexes.length);
  };

  const matchLabel =
    query.trim() === ""
      ? ""
      : matchIndexes.length
        ? `${Math.min(activeMatch + 1, matchIndexes.length)} of ${matchIndexes.length}`
        : "0 of 0";

  const currentLineIndex = matchIndexes.length
    ? matchIndexes[Math.min(activeMatch, matchIndexes.length - 1)]
    : -1;
  const q = query.trim();

  return (
    <div className={`code-block ${className}`.trim()}>
      {(showSearch || showCopy) && (
        <div className="code-block-toolbar">
          {showSearch && (
            <>
              <div className="code-search">
                <input
                  type="search"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Find in file…"
                  aria-label="Find in file"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      goMatch(e.shiftKey ? -1 : 1);
                    }
                  }}
                />
              </div>
              <div className="code-search-nav-group" role="group" aria-label="Search matches">
                <span className="code-search-count" aria-live="polite">
                  {matchLabel || "—"}
                </span>
                <Button
                  variant="secondary"
                  className="code-search-nav"
                  disabled={!matchIndexes.length}
                  onClick={() => goMatch(-1)}
                  aria-label="Previous match (Shift+Enter)"
                  title="Previous match (Shift+Enter)"
                >
                  ▲
                </Button>
                <Button
                  variant="secondary"
                  className="code-search-nav"
                  disabled={!matchIndexes.length}
                  onClick={() => goMatch(1)}
                  aria-label="Next match (Enter)"
                  title="Next match (Enter)"
                >
                  ▼
                </Button>
              </div>
            </>
          )}
          <div className="code-toolbar-trailing">
            <label className="code-wrap-toggle" title="Wrap long lines">
              <input type="checkbox" checked={wrap} onChange={(e) => setWrap(e.target.checked)} />
              Wrap
            </label>
            {showCopy && (
              <div className="code-copy-actions">
                {copyFlash && <span className="muted">{copyFlash}</span>}
                <Button variant="secondary" onClick={copySelection}>
                  Copy selection
                </Button>
                <Button variant="secondary" onClick={copyAll}>
                  Copy all
                </Button>
              </div>
            )}
            {language && <span className="doc-lang-badge">{language}</span>}
          </div>
        </div>
      )}
      <div className={`code-block-scroll ${wrap ? "is-wrap" : "is-scroll-x"}`} ref={scrollerRef}>
        <div className={`code-lines hljs ${wrap ? "is-wrap" : ""}`} role="table" aria-label="Source">
          {highlightedLines.map((html, i) => {
            const n = i + 1;
            const isMatch = matchIndexes.includes(i);
            const isCurrent = i === currentLineIndex;
            const marked =
              isMatch && q ? markQueryInHtml(html, q, isCurrent) : html;
            return (
              <div
                key={n}
                className={[
                  "code-line",
                  isMatch ? "is-match" : "",
                  isCurrent ? "is-current-match" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                data-line={n}
                id={`L${n}`}
                role="row"
              >
                <span className="line-number" aria-hidden="true">
                  {n}
                </span>
                <code
                  className={`line-code language-${language || "plaintext"}`}
                  dangerouslySetInnerHTML={{ __html: marked || " " }}
                />
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export { copyText };

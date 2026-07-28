import { useMemo, useState, type ReactNode } from "react";
import { api } from "../api";
import Button from "../components/Button";
import EmptyState from "../components/EmptyState";
import ErrorState from "../components/ErrorState";
import RootSelect from "../components/RootSelect";
import { useToast } from "../components/Toast";
import PageHead from "../PageHead";

type Mode = "knowledge" | "symbol" | "context";

type Citation = {
  uri?: string;
  title?: string;
  section?: string;
  rootName?: string;
  authority?: string;
};

type ContextPackage = {
  summary?: string;
  requiredApis?: string[];
  relevantSymbols?: string[];
  sequence?: string[];
  examples?: unknown[];
  constraints?: string[];
  pitfalls?: string[];
  citations?: Citation[];
  coverage?: string;
  estimatedTokens?: number;
  needsChoice?: boolean;
  availableRoots?: string[];
};

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

function asStringList(v: unknown): string[] {
  if (!Array.isArray(v)) return [];
  return v.map((x) => (typeof x === "string" ? x : JSON.stringify(x))).filter(Boolean);
}

async function copyText(text: string) {
  await navigator.clipboard.writeText(text);
}

function ResultSection({
  title,
  children,
  copyValue,
}: {
  title: string;
  children: ReactNode;
  copyValue?: string;
}) {
  const toast = useToast();
  if (!children) return null;
  return (
    <section className="search-section">
      <div className="search-section-head">
        <h3>{title}</h3>
        {copyValue && (
          <Button
            variant="ghost"
            className="btn-icon"
            aria-label={`Copy ${title}`}
            title={`Copy ${title}`}
            onClick={async () => {
              try {
                await copyText(copyValue);
                toast.push({ variant: "success", message: `Copied ${title}` });
              } catch {
                toast.push({ variant: "danger", message: "Copy failed" });
              }
            }}
          >
            Copy
          </Button>
        )}
      </div>
      <div className="search-section-body">{children}</div>
    </section>
  );
}

function ContextResults({ data }: { data: ContextPackage }) {
  const toast = useToast();
  const apis = asStringList(data.requiredApis);
  const symbols = asStringList(data.relevantSymbols);
  const sequence = asStringList(data.sequence);
  const constraints = asStringList(data.constraints);
  const pitfalls = asStringList(data.pitfalls);
  const examples = Array.isArray(data.examples) ? data.examples : [];
  const citations = Array.isArray(data.citations) ? data.citations : [];

  return (
    <div className="search-results stack">
      {(data.summary || data.coverage || data.estimatedTokens != null) && (
        <div className="search-meta panel">
          {data.summary && <p>{data.summary}</p>}
          <div className="row muted">
            {data.coverage && <span>Coverage: {data.coverage}</span>}
            {data.estimatedTokens != null && <span className="mono">~{data.estimatedTokens} tokens</span>}
            {data.needsChoice && <span>Root choice required</span>}
          </div>
          {data.availableRoots && data.availableRoots.length > 0 && (
            <p className="muted">Available roots: {data.availableRoots.join(", ")}</p>
          )}
        </div>
      )}

      {apis.length > 0 && (
        <ResultSection title="Required APIs" copyValue={apis.join("\n")}>
          <ul className="search-list mono">
            {apis.map((a) => (
              <li key={a}>{a}</li>
            ))}
          </ul>
        </ResultSection>
      )}
      {symbols.length > 0 && (
        <ResultSection title="Symbols" copyValue={symbols.join("\n")}>
          <ul className="search-list mono">
            {symbols.map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ul>
        </ResultSection>
      )}
      {sequence.length > 0 && (
        <ResultSection title="Sequence" copyValue={sequence.map((s, i) => `${i + 1}. ${s}`).join("\n")}>
          <ol className="search-list">
            {sequence.map((s) => (
              <li key={s}>{s}</li>
            ))}
          </ol>
        </ResultSection>
      )}
      {examples.length > 0 && (
        <ResultSection title="Examples" copyValue={JSON.stringify(examples, null, 2)}>
          <pre className="search-pre">{JSON.stringify(examples, null, 2)}</pre>
        </ResultSection>
      )}
      {constraints.length > 0 && (
        <ResultSection title="Constraints" copyValue={constraints.join("\n")}>
          <ul className="search-list">
            {constraints.map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
        </ResultSection>
      )}
      {pitfalls.length > 0 && (
        <ResultSection title="Pitfalls" copyValue={pitfalls.join("\n")}>
          <ul className="search-list">
            {pitfalls.map((p) => (
              <li key={p}>{p}</li>
            ))}
          </ul>
        </ResultSection>
      )}
      {citations.length > 0 && (
        <ResultSection title="Citations" copyValue={citations.map((c) => c.uri || c.title || "").join("\n")}>
          <ul className="search-citations">
            {citations.map((c, i) => (
              <li key={`${c.uri || c.title || "c"}-${i}`}>
                <div className="search-cite-main">
                  <strong>{c.title || c.uri || "Citation"}</strong>
                  {c.authority && <span className="badge">{c.authority}</span>}
                </div>
                {c.uri && (
                  <div className="mono muted search-cite-uri">
                    {c.uri}
                    <Button
                      variant="ghost"
                      className="btn-icon"
                      aria-label="Copy URI"
                      title="Copy URI"
                      onClick={async () => {
                        try {
                          await copyText(c.uri || "");
                          toast.push({ variant: "success", message: "Copied URI" });
                        } catch {
                          toast.push({ variant: "danger", message: "Copy failed" });
                        }
                      }}
                    >
                      Copy
                    </Button>
                  </div>
                )}
                {(c.section || c.rootName) && (
                  <div className="muted">
                    {[c.rootName, c.section].filter(Boolean).join(" · ")}
                  </div>
                )}
              </li>
            ))}
          </ul>
        </ResultSection>
      )}
    </div>
  );
}

function GenericResults({ data }: { data: unknown }) {
  const rec = asRecord(data);
  const hits =
    (rec && (rec.results || rec.hits || rec.symbols || rec.documents)) ||
    (Array.isArray(data) ? data : null);

  if (Array.isArray(hits) && hits.length > 0) {
    return (
      <div className="search-results">
        <ResultSection title="Hits" copyValue={JSON.stringify(hits, null, 2)}>
          <ul className="search-list">
            {hits.slice(0, 40).map((h, i) => {
              const r = asRecord(h) || {};
              const title = String(r.title || r.name || r.symbol || r.uri || `Hit ${i + 1}`);
              const uri = r.uri ? String(r.uri) : "";
              const root = r.rootName ? String(r.rootName) : "";
              return (
                <li key={`${title}-${i}`}>
                  <strong>{title}</strong>
                  {root && <span className="muted"> · {root}</span>}
                  {uri && <div className="mono muted">{uri}</div>}
                </li>
              );
            })}
          </ul>
        </ResultSection>
      </div>
    );
  }

  return null;
}

export default function SearchLab() {
  const toast = useToast();
  const [query, setQuery] = useState("");
  const [rootName, setRootName] = useState("");
  const [semantic, setSemantic] = useState(false);
  const [explain, setExplain] = useState(true);
  const [mode, setMode] = useState<Mode>("context");
  const [tokenBudget, setTokenBudget] = useState(2500);
  const [result, setResult] = useState<unknown>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [showRaw, setShowRaw] = useState(false);

  const modeHelp = useMemo(() => {
    if (mode === "context") return "Budgeted implementation package: APIs, sequence, examples, pitfalls, citations.";
    if (mode === "symbol") return "Find symbols by name across indexed extractors.";
    return "Search knowledge chunks and documents with optional explain plan.";
  }, [mode]);

  async function run() {
    setBusy(true);
    setError("");
    setShowRaw(false);
    try {
      if (mode === "symbol") {
        setResult(await api.searchSymbols({ name: query, roots: rootName ? [rootName] : [], limit: 20 }));
      } else if (mode === "context") {
        setResult(
          await api.searchContext({
            task: query,
            projectRoot: rootName || undefined,
            preferredRoots: rootName ? [rootName] : undefined,
            semantic,
            maxContextTokens: tokenBudget,
          }),
        );
      } else {
        setResult(
          await api.search({
            query,
            rootName: rootName || undefined,
            limit: 20,
            semantic,
            explain,
          }),
        );
      }
    } catch (e) {
      setResult(null);
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function reset() {
    setQuery("");
    setResult(null);
    setError("");
    setShowRaw(false);
  }

  const contextPkg = mode === "context" && result ? (asRecord(result) as ContextPackage | null) : null;
  const hasStructured =
    contextPkg &&
    (contextPkg.summary ||
      (contextPkg.requiredApis && contextPkg.requiredApis.length) ||
      (contextPkg.citations && contextPkg.citations.length) ||
      (contextPkg.sequence && contextPkg.sequence.length));

  return (
    <div className="search-lab">
      <PageHead title="Search Lab" blurb="Exercise retrieval against the live index." />
      <div className="panel stack search-lab-form">
        <label className="search-query-field">
          Query / task
          <textarea
            className="search-query-input"
            rows={3}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={
              mode === "symbol"
                ? "Symbol name, e.g. ListenForConnection…"
                : "Describe the implementation task, API, symbol, or problem…"
            }
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey && query.trim() && !busy) {
                e.preventDefault();
                void run();
              }
            }}
          />
        </label>

        <div className="search-config-row">
          <label>
            Mode
            <select value={mode} onChange={(e) => setMode(e.target.value as Mode)}>
              <option value="context">Implementation Context</option>
              <option value="symbol">Find Symbol</option>
              <option value="knowledge">Search Knowledge</option>
            </select>
          </label>
          <RootSelect
            value={rootName}
            onChange={setRootName}
            allowAll
            allRootsHint="Searches across all enabled roots and ranks the combined results."
          />
          {mode === "context" ? (
            <label>
              Token budget
              <input
                type="number"
                min={500}
                max={20000}
                step={100}
                value={tokenBudget}
                onChange={(e) => setTokenBudget(Number(e.target.value) || 2500)}
              />
            </label>
          ) : (
            <div className="search-config-spacer" aria-hidden="true" />
          )}
        </div>

        {(mode === "knowledge" || mode === "context") && (
          <div className="search-options-row">
            <label
              className="check-inline"
              title="Uses semantic similarity in addition to exact text and symbol matching."
            >
              <input type="checkbox" checked={semantic} onChange={(e) => setSemantic(e.target.checked)} />
              <span>Semantic matching</span>
            </label>
            {mode === "knowledge" && (
              <label className="check-inline" title="Include the retrieval plan explanation in the response.">
                <input type="checkbox" checked={explain} onChange={(e) => setExplain(e.target.checked)} />
                <span>Explain plan</span>
              </label>
            )}
          </div>
        )}

        <p className="search-mode-help">{modeHelp}</p>

        <div className="search-actions row">
          <Button
            variant="primary"
            className="btn-run-retrieval"
            disabled={!query.trim() || busy}
            onClick={() => void run()}
          >
            {busy ? "Running…" : "Run Retrieval"}
          </Button>
          <Button variant="ghost" onClick={reset} disabled={busy}>
            Reset
          </Button>
          {result != null && (
            <Button
              variant="ghost"
              onClick={async () => {
                try {
                  await copyText(JSON.stringify(result, null, 2));
                  toast.push({ variant: "success", message: "Copied full result" });
                } catch {
                  toast.push({ variant: "danger", message: "Copy failed" });
                }
              }}
            >
              Copy JSON
            </Button>
          )}
        </div>
      </div>

      <div className={`search-result-area ${result != null ? "has-results" : ""}`}>
        {error && (
          <ErrorState
            message={error}
            onRetry={() => {
              if (query.trim()) void run();
            }}
          />
        )}

        {!error && result == null && !busy && (
          <EmptyState
            title="Run a retrieval"
            body="Choose a mode and root, enter a task or symbol, then Run Retrieval. Results stay on this page for inspection."
            icon={
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
                <circle cx="11" cy="11" r="6.5" />
                <path d="M16 16l4 4" strokeLinecap="round" />
              </svg>
            }
          />
        )}

        {result != null && (
          <div className="stack search-result-stack">
            {hasStructured && contextPkg ? (
              <ContextResults data={contextPkg} />
            ) : (
              <GenericResults data={result} />
            )}
            <div className="panel search-raw-panel">
              <button type="button" className="btn btn-ghost" onClick={() => setShowRaw((v) => !v)}>
                {showRaw ? "Hide raw JSON" : "Show raw JSON"}
              </button>
              {showRaw && <pre className="search-pre">{JSON.stringify(result, null, 2)}</pre>}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

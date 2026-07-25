import { useState } from "react";
import { api } from "../api";
import PageHead from "../PageHead";

type Mode = "knowledge" | "symbol" | "context";

export default function SearchLab() {
  const [query, setQuery] = useState("");
  const [rootName, setRootName] = useState("");
  const [semantic, setSemantic] = useState(false);
  const [explain, setExplain] = useState(true);
  const [mode, setMode] = useState<Mode>("knowledge");
  const [result, setResult] = useState<unknown>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function run() {
    setBusy(true);
    setError("");
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
            maxContextTokens: 2500,
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
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <PageHead title="Search Lab" blurb="Exercise retrieval against the live index." />
      <div className="panel stack">
        <div className="form-grid">
          <label>
            Mode
            <select value={mode} onChange={(e) => setMode(e.target.value as Mode)}>
              <option value="knowledge">Search Knowledge</option>
              <option value="symbol">Find Symbol</option>
              <option value="context">Implementation Context</option>
            </select>
          </label>
          <label>
            Query / name / task
            <input value={query} onChange={(e) => setQuery(e.target.value)} />
          </label>
          <label>
            Root
            <input value={rootName} onChange={(e) => setRootName(e.target.value)} placeholder="optional" />
          </label>
        </div>
        {(mode === "knowledge" || mode === "context") && (
          <div className="row">
            <label>
              <input type="checkbox" checked={semantic} onChange={(e) => setSemantic(e.target.checked)} /> Semantic
            </label>
            {mode === "knowledge" && (
              <label>
                <input type="checkbox" checked={explain} onChange={(e) => setExplain(e.target.checked)} /> Explain plan
              </label>
            )}
          </div>
        )}
        <button className="primary" type="button" disabled={!query.trim() || busy} onClick={run}>
          {busy ? "Running…" : "Run"}
        </button>
      </div>
      {error && <div className="error-box">{error}</div>}
      {result != null && (
        <div className="panel">
          <h2>Result</h2>
          <pre>{JSON.stringify(result, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}

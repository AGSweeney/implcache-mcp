import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api, getToken, setToken } from "../api";
import ConfirmDialog from "../components/ConfirmDialog";
import PageHead from "../PageHead";
import { formatBytes } from "../format";

export default function Settings() {
  const qc = useQueryClient();
  const server = useQuery({ queryKey: ["server"], queryFn: api.server });
  const analytics = useQuery({ queryKey: ["analytics-status"], queryFn: api.analyticsStatus });
  const [token, setTok] = useState(getToken());
  const [enabled, setEnabled] = useState(true);
  const [retention, setRetention] = useState(90);
  const [storeTaskText, setStoreTaskText] = useState(false);
  const [storeEvidenceText, setStoreEvidenceText] = useState(false);
  const [clearOpen, setClearOpen] = useState(false);

  useEffect(() => {
    if (analytics.data) {
      setEnabled(analytics.data.enabled);
      setRetention(analytics.data.retentionDays);
      setStoreTaskText(!!analytics.data.storeTaskText);
      setStoreEvidenceText(!!analytics.data.storeEvidenceText);
    }
  }, [analytics.data]);

  const saveAnalytics = useMutation({
    mutationFn: () =>
      api.putAnalyticsSettings({
        enabled,
        retentionDays: retention,
        storeTaskText,
        storeEvidenceText,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["analytics-status"] });
      qc.invalidateQueries({ queryKey: ["server"] });
    },
  });

  const clearAnalytics = useMutation({
    mutationFn: () => api.clearAnalyticsData(true, true),
    onSuccess: () => {
      setClearOpen(false);
      qc.invalidateQueries({ queryKey: ["analytics-status"] });
      qc.invalidateQueries({ queryKey: ["analytics"] });
    },
  });

  const st = analytics.data;

  const exportAggregate = async (format: "json" | "csv") => {
    const q = new URLSearchParams();
    q.set("days", "90");
    const url = api.analyticsExportUrl(q, format);
    const headers = new Headers();
    const tok = getToken();
    if (tok) headers.set("Authorization", `Bearer ${tok}`);
    const res = await fetch(url, { method: "POST", headers });
    if (!res.ok) {
      alert(`Export failed: ${res.status}`);
      return;
    }
    const blob = await res.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = `implcache-analytics.${format}`;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  return (
    <div>
      <PageHead title="Settings" blurb="Server capabilities, API token, and local usage analytics." />
      <div className="panel stack">
        <h2 className="section-title">Connection</h2>
        <pre>{JSON.stringify(server.data, null, 2)}</pre>
      </div>
      <div className="panel stack">
        <h2 className="section-title">API token</h2>
        <p className="muted">
          When the server is started with <span className="mono">-librarian-token</span>, paste the bearer token here.
          Stored only in this browser&apos;s localStorage.
        </p>
        <label>
          Bearer token
          <input type="password" value={token} onChange={(e) => setTok(e.target.value)} autoComplete="off" />
        </label>
        <div className="row">
          <button
            className="primary"
            type="button"
            onClick={() => {
              setToken(token.trim());
              server.refetch();
            }}
          >
            Save token
          </button>
          <button
            type="button"
            onClick={() => {
              setTok("");
              setToken("");
              server.refetch();
            }}
          >
            Clear
          </button>
        </div>
      </div>

      <div className="panel stack">
        <h2 className="section-title">Usage analytics</h2>
        {analytics.isError && <p className="error">Could not load analytics status.</p>}
        {st && (
          <>
            <p className="muted">{st.message || "Local analytics status."}</p>
            <p className="muted">
              No data leaves this machine. External transmission is unsupported. Metadata only by default (no
              prompts, excerpts, or answers).
            </p>
            <label className="row" style={{ gap: "0.6rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={enabled}
                disabled={!st.available}
                onChange={(e) => setEnabled(e.target.checked)}
              />
              Enable local usage analytics
            </label>
            <label className="row" style={{ gap: "0.6rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={storeTaskText}
                disabled={!st.available}
                onChange={(e) => setStoreTaskText(e.target.checked)}
              />
              Store request text for diagnostics
            </label>
            <label className="row" style={{ gap: "0.6rem", alignItems: "center" }}>
              <input
                type="checkbox"
                checked={storeEvidenceText}
                disabled={!st.available}
                onChange={(e) => setStoreEvidenceText(e.target.checked)}
              />
              Store returned evidence excerpts
            </label>
            {(storeTaskText || storeEvidenceText) && (
              <p className="analytics-banner warn-banner">
                Diagnostic capture is enabled. Stored text may include sensitive content. Keep it local and clear
                when finished.
              </p>
            )}
            <label>
              Retention
              <select
                value={retention}
                disabled={!st.available}
                onChange={(e) => setRetention(Number(e.target.value))}
              >
                <option value={30}>30 days</option>
                <option value={90}>90 days</option>
                <option value={180}>180 days</option>
                <option value={365}>365 days</option>
                <option value={0}>Unlimited</option>
              </select>
            </label>
            <div className="metric-strip">
              <div className="metric-strip-item">
                <span className="metric-strip-label">Database</span>
                <span className="metric-strip-value mono">{st.dbPath || "—"}</span>
              </div>
              <div className="metric-strip-item">
                <span className="metric-strip-label">Size</span>
                <span className="metric-strip-value">{formatBytes(st.databaseBytes)}</span>
              </div>
              <div className="metric-strip-item">
                <span className="metric-strip-label">Requests</span>
                <span className="metric-strip-value">{st.requestCount}</span>
              </div>
              <div className="metric-strip-item">
                <span className="metric-strip-label">Dropped</span>
                <span className="metric-strip-value">{st.droppedEvents}</span>
              </div>
              <div className="metric-strip-item">
                <span className="metric-strip-label">Estimator</span>
                <span className="metric-strip-value mono">{st.tokenEstimatorVersion || "chars_div_4_v1"}</span>
              </div>
            </div>
            <div className="row wrap">
              <button
                className="primary"
                type="button"
                disabled={!st.available || saveAnalytics.isPending}
                onClick={() => saveAnalytics.mutate()}
              >
                {saveAnalytics.isPending ? "Saving…" : "Save analytics settings"}
              </button>
              <button type="button" disabled={!st.available} onClick={() => exportAggregate("json")}>
                Export Aggregate Metrics (JSON)
              </button>
              <button type="button" disabled={!st.available} onClick={() => exportAggregate("csv")}>
                Export CSV
              </button>
              <button type="button" className="danger" disabled={!st.available} onClick={() => setClearOpen(true)}>
                Clear Analytics Data
              </button>
            </div>
            {saveAnalytics.isError && <p className="error">{(saveAnalytics.error as Error).message}</p>}
          </>
        )}
      </div>

      <ConfirmDialog
        open={clearOpen}
        title="Clear analytics data?"
        body="Deletes all local usage events. The knowledge database is not affected."
        confirmLabel="Clear data"
        busy={clearAnalytics.isPending}
        onCancel={() => setClearOpen(false)}
        onConfirm={() => clearAnalytics.mutate()}
      />
    </div>
  );
}

import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  api,
  normalizeList,
  type AnalyticsCoverage,
  type AnalyticsEfficiency,
  type AnalyticsGrounding,
  type AnalyticsKnowledge,
  type AnalyticsRequestDetail,
  type AnalyticsRequestRow,
  type AnalyticsSummary,
  type AnalyticsTimePoint,
} from "../api";
import PageHead from "../PageHead";

type Tab = "overview" | "usage" | "quality" | "outcomes" | "efficiency";

function pct(n: number | undefined) {
  if (n == null || Number.isNaN(n)) return "—";
  return `${(n * 100).toFixed(1)}%`;
}

function compactTokens(n: number | null | undefined) {
  if (n == null || Number.isNaN(n)) return null;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 10_000) return `${(n / 1000).toFixed(1)}K`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(Math.round(n));
}

function na(v: number | null | undefined, fmt?: (n: number) => string) {
  if (v == null) return "Not available";
  return fmt ? fmt(v) : String(v);
}

function rangeFromParam(range: string): { days?: number } {
  switch (range) {
    case "24h":
      return { days: 1 };
    case "7d":
      return { days: 7 };
    case "30d":
      return { days: 30 };
    case "90d":
      return { days: 90 };
    default:
      return {};
  }
}

function buildQuery(sp: URLSearchParams) {
  const q = new URLSearchParams();
  const range = sp.get("range") || "30d";
  const { days } = rangeFromParam(range);
  if (days) q.set("days", String(days));
  for (const key of ["root", "tool", "coverage", "status"] as const) {
    const v = sp.get(key);
    if (v) q.set(key, v);
  }
  const bucketOverride = sp.get("bucket");
  q.set("bucket", bucketOverride || (range === "24h" ? "hour" : "day"));
  return q;
}

function allowedBuckets(range: string): string[] {
  switch (range) {
    case "24h":
      return ["hour", "day"];
    case "7d":
      return ["hour", "day", "week"];
    case "30d":
    case "90d":
      return ["day", "week", "month"];
    default:
      return ["day", "week", "month"];
  }
}

function formatBucketLabel(bucket: string) {
  if (/^\d{4}-\d{2}-\d{2}T\d{2}/.test(bucket)) {
    const d = new Date(bucket);
    if (!Number.isNaN(d.getTime())) {
      return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit" });
    }
  }
  if (/^\d{4}-\d{2}-\d{2}$/.test(bucket)) {
    const d = new Date(bucket + "T00:00:00Z");
    if (!Number.isNaN(d.getTime())) {
      return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
    }
  }
  return bucket;
}

function formatTime(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function statusLabel(s: string) {
  switch (s) {
    case "grounded_local":
      return "Grounded local";
    case "grounded_curated":
      return "Grounded curated";
    case "grounded_mixed":
      return "Grounded mixed";
    case "root_selection_required":
      return "Root choice";
    case "no_local_match":
      return "No match";
    case "local_insufficient":
      return "Insufficient";
    case "request_error":
      return "Error";
    default:
      return s || "—";
  }
}

type BarRow = { label: string; value: number; color: string; title?: string };

function HBarChart({ rows, empty }: { rows: BarRow[]; empty?: string }) {
  const max = Math.max(1, ...rows.map((r) => r.value));
  const any = rows.some((r) => r.value > 0);
  if (!any) return <p className="muted">{empty || "No data in this range."}</p>;
  return (
    <div className="grounding-bars">
      {rows.map((r) => (
        <div className="grounding-row" key={r.label} title={r.title || `${r.label}: ${r.value}`}>
          <span className="grounding-label">{r.label}</span>
          <div className="grounding-track">
            <div
              className="grounding-fill"
              style={{ width: `${(r.value / max) * 100}%`, background: r.color }}
            />
          </div>
          <span className="mono grounding-n">{r.value}</span>
        </div>
      ))}
    </div>
  );
}

type SeriesKey = "total" | "grounded" | "rootChoice" | "noMatch" | "errors";

function RequestsOverTime({ points }: { points: AnalyticsTimePoint[] }) {
  const [series, setSeries] = useState<Record<SeriesKey, boolean>>({
    total: true,
    grounded: true,
    rootChoice: false,
    noMatch: false,
    errors: false,
  });
  if (!points.length) {
    return <p className="muted chart-empty">No request data in this range.</p>;
  }
  if (points.length === 1) {
    const p = points[0];
    return (
      <div className="chart-empty-state chart-compact">
        <p className="chart-empty-title">
          {p.total} request{p.total === 1 ? "" : "s"} in one time bucket
        </p>
        <p className="muted">
          {formatBucketLabel(p.bucket)} · Change range or granularity to view a trend
        </p>
        <div className="compact-bar-row" title={`${p.total} requests`}>
          <div className="compact-bar-fill" style={{ width: "100%" }} />
          <span className="mono">{p.total}</span>
        </div>
      </div>
    );
  }
  const colors: Record<SeriesKey, string> = {
    total: "var(--accent)",
    grounded: "var(--success)",
    rootChoice: "var(--warning)",
    noMatch: "var(--copper)",
    errors: "var(--danger)",
  };
  const valueOf = (p: AnalyticsTimePoint, k: SeriesKey) => {
    switch (k) {
      case "total":
        return p.total;
      case "grounded":
        return p.grounded;
      case "rootChoice":
        return p.rootChoice || 0;
      case "noMatch":
        return p.noMatch || 0;
      case "errors":
        return p.errors;
    }
  };
  const active = (Object.keys(series) as SeriesKey[]).filter((k) => series[k]);
  const w = 640;
  const h = 180;
  const padL = 36;
  const padR = 16;
  const padT = 20;
  const padB = 36;
  const max = Math.max(1, ...points.flatMap((p) => active.map((k) => valueOf(p, k))));
  const xs = points.map((_, i) => padL + (i * (w - padL - padR)) / Math.max(1, points.length - 1));
  return (
    <div>
      <div className="chart-legend series-toggles">
        {(
          [
            ["total", "Total"],
            ["grounded", "Grounded"],
            ["rootChoice", "Root choice"],
            ["noMatch", "No match"],
            ["errors", "Errors"],
          ] as const
        ).map(([k, label]) => (
          <label key={k} className="series-toggle">
            <input
              type="checkbox"
              checked={series[k]}
              onChange={(e) => setSeries((s) => ({ ...s, [k]: e.target.checked }))}
            />
            <i style={{ background: colors[k] }} /> {label}
          </label>
        ))}
      </div>
      <svg className="chart-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Requests over time">
        <line x1={padL} y1={h - padB} x2={w - padR} y2={h - padB} stroke="var(--line)" />
        <line x1={padL} y1={padT} x2={padL} y2={h - padB} stroke="var(--line)" />
        <text x={padL - 6} y={padT + 4} textAnchor="end" fill="var(--muted)" fontSize="10">
          {max}
        </text>
        <text x={padL - 6} y={h - padB} textAnchor="end" fill="var(--muted)" fontSize="10">
          0
        </text>
        {active.map((k) => {
          const ys = points.map((p) => h - padB - (valueOf(p, k) / max) * (h - padT - padB));
          const d = points.map((_, i) => `${i === 0 ? "M" : "L"}${xs[i]},${ys[i]}`).join(" ");
          return <path key={k} d={d} fill="none" stroke={colors[k]} strokeWidth="2.2" />;
        })}
        {points.map((p, i) => (
          <g key={p.bucket}>
            <circle cx={xs[i]} cy={h - padB - (p.total / max) * (h - padT - padB)} r="3" fill="var(--copper)">
              <title>{`${formatBucketLabel(p.bucket)}: total ${p.total}, grounded ${p.grounded}, root ${p.rootChoice || 0}, no-match ${p.noMatch || 0}, errors ${p.errors}`}</title>
            </circle>
            <text x={xs[i]} y={h - 12} textAnchor="middle" fill="var(--muted)" fontSize="9">
              {formatBucketLabel(p.bucket)}
            </text>
          </g>
        ))}
      </svg>
    </div>
  );
}

function TokenTimeseries({ points, mode }: { points: AnalyticsTimePoint[]; mode: "tokens" | "package" }) {
  if (!points.length) return <p className="muted">No token timeseries yet.</p>;
  if (points.length === 1) {
    const p = points[0];
    return (
      <p className="muted">
        {mode === "tokens"
          ? `${compactTokens(p.tokensServed || 0) || 0} tokens served in one bucket (${formatBucketLabel(p.bucket)})`
          : `Avg package ${p.avgPackage != null ? Math.round(p.avgPackage) : "—"} · reduction ${p.avgReduction != null ? p.avgReduction.toFixed(1) + "%" : "—"}`}
      </p>
    );
  }
  const w = 640;
  const h = 150;
  const padL = 40;
  const padR = 12;
  const padT = 16;
  const padB = 32;
  const series =
    mode === "tokens"
      ? [
          { key: "served", color: "var(--accent)", get: (p: AnalyticsTimePoint) => p.tokensServed || 0 },
          { key: "source", color: "var(--info)", get: (p: AnalyticsTimePoint) => p.sourceTokens || 0 },
          { key: "avoided", color: "var(--success)", get: (p: AnalyticsTimePoint) => p.tokensAvoided || 0 },
        ]
      : [
          { key: "pkg", color: "var(--accent)", get: (p: AnalyticsTimePoint) => p.avgPackage || 0 },
          { key: "red", color: "var(--warning)", get: (p: AnalyticsTimePoint) => p.avgReduction || 0 },
        ];
  const max = Math.max(1, ...points.flatMap((p) => series.map((s) => s.get(p))));
  const xs = points.map((_, i) => padL + (i * (w - padL - padR)) / Math.max(1, points.length - 1));
  return (
    <svg className="chart-svg chart-svg-sm" viewBox={`0 0 ${w} ${h}`} role="img">
      <line x1={padL} y1={h - padB} x2={w - padR} y2={h - padB} stroke="var(--line)" />
      {series.map((s) => {
        const ys = points.map((p) => h - padB - (s.get(p) / max) * (h - padT - padB));
        const d = points.map((_, i) => `${i === 0 ? "M" : "L"}${xs[i]},${ys[i]}`).join(" ");
        return <path key={s.key} d={d} fill="none" stroke={s.color} strokeWidth="2" />;
      })}
    </svg>
  );
}

function CoverageBars({
  c,
  onFilterUnclassified,
}: {
  c: AnalyticsCoverage;
  onFilterUnclassified: () => void;
}) {
  const rows: BarRow[] = [
    { label: "High", value: c.high, color: "var(--success)", title: `High coverage: ${c.high} of ${c.grounded} grounded` },
    { label: "Medium", value: c.medium, color: "var(--warning)", title: `Medium coverage: ${c.medium} of ${c.grounded} grounded` },
    { label: "Low", value: c.low, color: "var(--danger)", title: `Low coverage: ${c.low} of ${c.grounded} grounded` },
    {
      label: "Unclassified",
      value: c.unclassified,
      color: "var(--info)",
      title: `Should have coverage but did not: ${c.unclassified} of ${c.grounded} grounded`,
    },
    {
      label: "Not applicable",
      value: c.notApplicable || 0,
      color: "var(--muted)",
      title: `Coverage does not apply: ${c.notApplicable || 0}`,
    },
    {
      label: "Insufficient",
      value: c.localInsufficient,
      color: "var(--copper)",
      title: `Local insufficient: ${c.localInsufficient}`,
    },
    {
      label: "No match",
      value: c.noLocalMatch,
      color: "var(--danger)",
      title: `No local match: ${c.noLocalMatch}`,
    },
    {
      label: "Root choice",
      value: c.rootSelectionRequired,
      color: "var(--warning)",
      title: `Root selection required: ${c.rootSelectionRequired}`,
    },
  ];
  return (
    <>
      <p className="muted coverage-note">
        High / medium / low / unclassified are among grounded requests ({c.grounded}). Not applicable is
        separate. Insufficient, no match, and root choice are request outcomes.
      </p>
      {(c.unclassifiedWarning || (c.grounded > 0 && c.unclassified / c.grounded > 0.2)) && (
        <div className="analytics-banner warn-banner">
          <strong>Coverage classification incomplete</strong>
          <p>
            {c.unclassified} of {c.grounded} grounded requests do not have a coverage classification. This
            may distort retrieval-quality trends.{" "}
            <button type="button" className="linkish" onClick={onFilterUnclassified}>
              View unclassified requests
            </button>
          </p>
        </div>
      )}
      <HBarChart rows={rows} />
    </>
  );
}

function OutcomeAndEvidence({ g }: { g: AnalyticsGrounding }) {
  const o = g.outcomes;
  const e = g.evidence;
  const outcomeRows: BarRow[] = [
    { label: "Grounded curated", value: o.curated, color: "var(--accent)" },
    { label: "Grounded local", value: o.local || 0, color: "var(--success)" },
    { label: "Grounded mixed", value: o.mixed, color: "var(--warning)" },
    { label: "No match", value: o.noMatch, color: "var(--danger)" },
    { label: "Insufficient", value: o.insufficient, color: "var(--copper)" },
    { label: "Root choice", value: o.rootSelectionRequired, color: "var(--warning)" },
    { label: "Errors", value: o.errors, color: "var(--danger)" },
    { label: "Other", value: o.other, color: "var(--muted)" },
  ];
  const evidenceRows: BarRow[] = [
    { label: "Raw documents", value: e.rawDocuments, color: "var(--text-secondary)", title: "Requests with citation evidence" },
    { label: "Symbols", value: e.symbol, color: "var(--info)", title: "Requests with symbol evidence" },
    { label: "Documents", value: e.document, color: "var(--accent)", title: "Requests that fetched a document" },
    { label: "Recipes", value: e.recipe, color: "var(--copper)" },
    { label: "Curated knowledge", value: e.curated, color: "var(--accent)" },
  ];
  const outcomeSum =
    o.curated +
    (o.local || 0) +
    o.mixed +
    o.noMatch +
    o.insufficient +
    o.rootSelectionRequired +
    o.errors +
    o.other;
  return (
    <div className="analytics-split">
      <div>
        <h3 className="subsection-title">Request Outcomes</h3>
        <p className="muted">Mutually exclusive — each request counted once ({outcomeSum}/{o.total}).</p>
        <HBarChart rows={outcomeRows} />
      </div>
      <div>
        <h3 className="subsection-title">Evidence Usage</h3>
        <p className="muted">Overlapping — a request may use more than one type.</p>
        <HBarChart rows={evidenceRows} />
      </div>
    </div>
  );
}

function SummaryCards({ s }: { s: AnalyticsSummary }) {
  const cards: {
    label: string;
    value: string;
    detail?: string;
    title?: string;
    hide?: boolean;
  }[] = [
    {
      label: "Requests",
      value: String(s.totalRequests),
      detail: "in selected range",
      title: "Eligible analytics-tracked requests in the selected range.",
    },
    {
      label: "Local evidence rate",
      value: pct(s.localEvidenceRate),
      detail: `${s.groundedRequests} of ${s.totalRequests} requests`,
      title: "Requests that returned curated, recipe, symbol, raw-document, or mixed local evidence.",
    },
    {
      label: "Curated usage rate",
      value: pct(s.curatedUsageRate),
      detail: `${s.curatedRequests} of ${s.groundedRequests} grounded`,
      title: "Grounded requests containing at least one reviewed curated knowledge entry.",
    },
    {
      label: "High coverage",
      value: pct(s.highCoverageRate),
      detail: `${s.highCoverage} of ${s.groundedRequests} grounded`,
      title: "Grounded requests classified as having sufficient implementation evidence across the required package dimensions.",
    },
    {
      label: "Coverage unclassified",
      value: pct(s.unclassifiedCoverageRate),
      detail: `${s.unclassifiedCoverage} of ${s.groundedRequests} grounded`,
      title: "Grounded requests that should have received a coverage classification but did not.",
      hide: s.unclassifiedCoverage === 0 && s.groundedRequests === 0,
    },
    {
      label: "Root selection rate",
      value: pct(s.rootSelectionRate),
      detail: `${s.rootSelectionRequired} of ${s.totalRequests} requests`,
      title: "Requests that required an explicit root choice before retrieval could proceed.",
    },
    {
      label: "Local context tokens served",
      value: compactTokens(s.localContextTokensServed) || "0",
      detail: "estimated from returned packages",
      title: "Estimated tokens returned from ImplCache knowledge after package trimming.",
    },
    {
      label: "Avg package tokens",
      value: s.avgPackageTokens != null ? Math.round(s.avgPackageTokens).toLocaleString() : "—",
      detail: "estimated per grounded package",
      title: "Average estimated tokens returned per grounded context package.",
      hide: s.avgPackageTokens == null,
    },
  ].filter((c) => !c.hide);
  return (
    <>
      <div className="analytics-cards">
        {cards.map((c) => (
          <div className="metric analytics-card" key={c.label} title={c.title || c.label}>
            <div className="n">{c.value}</div>
            <div className="l">{c.label}</div>
            {c.detail && <div className="metric-detail">{c.detail}</div>}
          </div>
        ))}
      </div>
      <p className={`reconcile-line ${s.reconcileOk ? "" : "warn"}`}>
        {s.totalRequests} requests total · {s.groundedRequests} grounded · {s.rootSelectionRequired}{" "}
        required root selection · {s.noLocalMatch} no match · {s.localInsufficient} insufficient ·{" "}
        {s.errors} errors
        {!s.reconcileOk && (
          <span className="error"> · Telemetry warning: outcomes sum to {s.reconcileSum}, not {s.totalRequests}</span>
        )}
      </p>
    </>
  );
}

function RecentRequestsTable({
  rows,
  total,
  offset,
  limit,
  sort,
  order,
  onOpen,
  onSort,
  onPage,
}: {
  rows: AnalyticsRequestRow[];
  total: number;
  offset: number;
  limit: number;
  sort: string;
  order: string;
  onOpen: (id: string) => void;
  onSort: (col: string) => void;
  onPage: (nextOffset: number) => void;
}) {
  if (!rows.length) return <p className="muted">No recent requests in this range.</p>;
  const sortMark = (col: string) => (sort === col ? (order === "asc" ? " ↑" : " ↓") : "");
  return (
    <div>
      <div className="data-table-wrap">
        <table className="data-table analytics-table">
          <thead>
            <tr>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("time")}>
                  Time{sortMark("time")}
                </button>
              </th>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("tool")}>
                  Tool{sortMark("tool")}
                </button>
              </th>
              <th>Root</th>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("status")}>
                  Status{sortMark("status")}
                </button>
              </th>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("coverage")}>
                  Coverage{sortMark("coverage")}
                </button>
              </th>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("tokens")}>
                  Tokens{sortMark("tokens")}
                </button>
              </th>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("sources")}>
                  Sources{sortMark("sources")}
                </button>
              </th>
              <th>
                <button type="button" className="th-btn" onClick={() => onSort("latency")}>
                  Latency{sortMark("latency")}
                </button>
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.requestId} className="clickable-row" onClick={() => onOpen(r.requestId)}>
                <td>{formatTime(r.occurredAt)}</td>
                <td className="mono">{r.toolName}</td>
                <td>{r.roots?.length ? r.roots.join(", ") : "—"}</td>
                <td>
                  {statusLabel(r.resultStatus)}
                  {r.curated ? " · curated" : ""}
                </td>
                <td>{r.coverage || "—"}</td>
                <td className="mono">{r.returnedTokens || r.estimatedTokens || "—"}</td>
                <td className="mono">{r.sourceCount ?? "—"}</td>
                <td className="mono">{r.latencyMs != null ? `${r.latencyMs} ms` : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="row wrap" style={{ justifyContent: "space-between", marginTop: "0.75rem" }}>
        <span className="muted">
          Showing {offset + 1}–{Math.min(offset + rows.length, total)} of {total}
        </span>
        <div className="row">
          <button type="button" disabled={offset <= 0} onClick={() => onPage(Math.max(0, offset - limit))}>
            Previous
          </button>
          <button
            type="button"
            disabled={offset + limit >= total}
            onClick={() => onPage(offset + limit)}
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}

function RequestDrilldown({
  detail,
  loading,
  onClose,
}: {
  detail?: AnalyticsRequestDetail;
  loading: boolean;
  onClose: () => void;
}) {
  return (
    <div className="drilldown-backdrop" onClick={onClose} role="presentation">
      <aside
        className="drilldown-panel"
        role="dialog"
        aria-label="Request detail"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="row" style={{ justifyContent: "space-between", alignItems: "center" }}>
          <h2 className="section-title" style={{ margin: 0 }}>
            Request detail
          </h2>
          <button type="button" onClick={onClose}>
            Close
          </button>
        </div>
        {loading && <p className="muted">Loading…</p>}
        {detail && (
          <div className="stack">
            <dl className="detail-grid">
              <div>
                <dt>Time</dt>
                <dd>{formatTime(detail.occurredAt)}</dd>
              </div>
              <div>
                <dt>Request ID</dt>
                <dd className="mono wrap">{detail.requestId}</dd>
              </div>
              <div>
                <dt>Tool</dt>
                <dd className="mono">{detail.toolName}</dd>
              </div>
              <div>
                <dt>Request class</dt>
                <dd>{detail.requestClass || "—"}</dd>
              </div>
              <div>
                <dt>Client</dt>
                <dd>{detail.clientName || "—"}</dd>
              </div>
              <div>
                <dt>Model</dt>
                <dd>{detail.modelName || "—"}</dd>
              </div>
              <div>
                <dt>Session hash</dt>
                <dd className="mono wrap">{detail.sessionHash || "—"}</dd>
              </div>
              <div>
                <dt>Status</dt>
                <dd>{statusLabel(detail.resultStatus)}</dd>
              </div>
              <div>
                <dt>Coverage</dt>
                <dd>
                  {detail.coverage || "—"}
                  {detail.coverageApplicable === false ? " (not applicable)" : ""}
                </dd>
              </div>
              <div>
                <dt>Freshness</dt>
                <dd>{detail.freshness || "—"}</dd>
              </div>
              <div>
                <dt>Roots</dt>
                <dd>{detail.roots?.length ? detail.roots.join(", ") : "—"}</dd>
              </div>
              <div>
                <dt>Returned tokens (est.)</dt>
                <dd className="mono">{detail.returnedTokens || detail.estimatedTokens || "—"}</dd>
              </div>
              <div>
                <dt>Structured / raw</dt>
                <dd className="mono">
                  {detail.structuredTokens ?? "—"} / {detail.rawDocumentTokens ?? "—"}
                </dd>
              </div>
              <div>
                <dt>Source tokens (est.)</dt>
                <dd className="mono">{detail.estimatedSourceTokens || "—"}</dd>
              </div>
              <div>
                <dt>Tokens avoided (est.)</dt>
                <dd className="mono">{detail.estimatedTokensAvoided || "—"}</dd>
              </div>
              <div>
                <dt>Context reduction</dt>
                <dd className="mono">
                  {detail.contextReductionPercent != null
                    ? `${detail.contextReductionPercent.toFixed(1)}%`
                    : "—"}
                </dd>
              </div>
              <div>
                <dt>Estimator</dt>
                <dd className="mono">{detail.tokenEstimatorVersion || "—"}</dd>
              </div>
              <div>
                <dt>Latency</dt>
                <dd className="mono">{detail.latencyMs} ms</dd>
              </div>
              <div>
                <dt>Fingerprint</dt>
                <dd className="mono wrap">{detail.contextFingerprint || "—"}</dd>
              </div>
              <div>
                <dt>Task hash</dt>
                <dd className="mono wrap">{detail.taskHash || "—"}</dd>
              </div>
              <div>
                <dt>Follow-up retrieval</dt>
                <dd>{detail.additionalRetrievalRecommended ? "Recommended" : "No"}</dd>
              </div>
              <div>
                <dt>Counts</dt>
                <dd>
                  sources {detail.sourceCount ?? "—"} · citations {detail.citationCount} · symbols{" "}
                  {detail.symbolCount} · recipes {detail.recipeCount} · curated {detail.curatedCount}
                </dd>
              </div>
            </dl>
            {detail.errorMessage && <p className="error">{detail.errorMessage}</p>}
            <h3 className="subsection-title">Evidence (metadata)</h3>
            {!detail.evidence?.length && <p className="muted">No evidence rows.</p>}
            {!!detail.evidence?.length && (
              <ul className="evidence-list">
                {detail.evidence.map((e, i) => (
                  <li key={`${e.evidenceKey}-${i}`}>
                    <span className="mono">{e.evidenceType}</span>
                    {e.rootKey ? ` · ${e.rootKey}` : ""}
                    {e.estimatedTokens ? ` · ~${e.estimatedTokens} tok` : ""}
                    <div className="mono muted wrap">{e.evidenceKey || e.sourceUri || "—"}</div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </aside>
    </div>
  );
}

function EfficiencyPanel({ e }: { e: AnalyticsEfficiency }) {
  const cards: { label: string; value: string; title?: string; hide?: boolean }[] = [
    {
      label: "Local context tokens served",
      value: na(e.localContextTokensServed, (n) => compactTokens(n) || String(n)),
      title: "Sum of final package token estimates (Estimated)",
    },
    {
      label: "Structured-package tokens",
      value: na(e.structuredTokensServed, (n) => compactTokens(n) || String(n)),
      hide: e.structuredTokensServed == null,
    },
    {
      label: "Raw-document tokens",
      value: na(e.rawDocumentTokensServed, (n) => compactTokens(n) || String(n)),
      hide: e.rawDocumentTokensServed == null,
    },
    {
      label: "Avg package tokens",
      value: na(e.avgPackageTokens, (n) => Math.round(n).toLocaleString()),
      hide: e.avgPackageTokens == null,
    },
    {
      label: "Estimated source tokens",
      value: na(e.estimatedSourceTokens, (n) => compactTokens(n) || String(n)),
      hide: e.estimatedSourceTokens == null,
    },
    {
      label: "Estimated tokens avoided",
      value: na(e.estimatedTokensAvoided, (n) => compactTokens(n) || String(n)),
      hide: e.estimatedTokensAvoided == null,
    },
    {
      label: "Avg context reduction",
      value: na(e.avgContextReductionPercent, (n) => `${n.toFixed(1)}%`),
      hide: e.avgContextReductionPercent == null,
    },
    {
      label: "Raw-document share",
      value: na(e.rawDocumentShare, (n) => pct(n)),
      hide: e.rawDocumentShare == null,
    },
    {
      label: "Tokens / grounded request",
      value: na(e.tokensPerGroundedRequest, (n) => Math.round(n).toLocaleString()),
      hide: e.tokensPerGroundedRequest == null,
    },
    {
      label: "Tokens / successful outcome",
      value: na(e.tokensPerSuccessfulOutcome, (n) => Math.round(n).toLocaleString()),
      // Hidden until implementation outcome reports exist (outcome_events).
      hide: e.tokensPerSuccessfulOutcome == null || !e.successfulOutcomes,
    },
  ].filter((c) => !c.hide);
  return (
    <div className="stack">
      <p className="muted">
        All token values are estimates ({e.tokenEstimatorVersion || "chars_div_4_v1"}).
      </p>
      <div className="analytics-cards">
        {cards.map((c) => (
          <div className="metric analytics-card" key={c.label} title={c.title || c.label}>
            <div className="n">{c.value}</div>
            <div className="l">{c.label}</div>
          </div>
        ))}
      </div>
      <h3 className="subsection-title">Tokens over time (Estimated)</h3>
      <TokenTimeseries points={e.tokenTimeseries || []} mode="tokens" />
      <h3 className="subsection-title">Average package size &amp; reduction</h3>
      <TokenTimeseries points={e.packageTimeseries || []} mode="package" />
      <h3 className="subsection-title">Source-type breakdown</h3>
      <HBarChart
        rows={(e.sourceTypeBreakdown || []).map((b) => ({
          label: b.label || b.type,
          value: b.tokens,
          color: b.type === "overhead" ? "var(--muted)" : "var(--accent)",
        }))}
        empty="No evidence token attribution yet."
      />
    </div>
  );
}

function KnowledgePanel({ k }: { k: AnalyticsKnowledge }) {
  return (
    <div className="analytics-split">
      <div>
        <h3 className="subsection-title">Top roots</h3>
        <HBarChart
          rows={(k.roots || []).slice(0, 12).map((r) => ({
            label: r.label || r.key,
            value: r.timesSelected,
            color: "var(--accent)",
          }))}
          empty="No root usage yet."
        />
      </div>
      <div>
        <h3 className="subsection-title">Top evidence keys</h3>
        <HBarChart
          rows={(k.evidence || []).slice(0, 12).map((r) => ({
            label: (r.key || "").slice(0, 40),
            value: r.timesSelected,
            color: "var(--copper)",
            title: r.key,
          }))}
          empty="No evidence usage yet."
        />
      </div>
    </div>
  );
}

export default function Analytics() {
  const [sp, setSp] = useSearchParams();
  const tab = (sp.get("tab") as Tab) || "overview";
  const range = sp.get("range") || "30d";
  const q = useMemo(() => buildQuery(sp), [sp]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const reqOffset = Number(sp.get("roffset") || "0") || 0;
  const reqSort = sp.get("rsort") || "time";
  const reqOrder = sp.get("rorder") || "desc";

  const status = useQuery({ queryKey: ["analytics-status"], queryFn: api.analyticsStatus, refetchInterval: 15000 });
  const summary = useQuery({
    queryKey: ["analytics", "summary", q.toString()],
    queryFn: () => api.analyticsSummary(q),
    enabled: !!status.data?.available,
    refetchInterval: 15000,
  });
  const series = useQuery({
    queryKey: ["analytics", "timeseries", q.toString()],
    queryFn: () => api.analyticsTimeseries(q),
    enabled: !!status.data?.available,
  });
  const coverage = useQuery({
    queryKey: ["analytics", "coverage", q.toString()],
    queryFn: () => api.analyticsCoverage(q),
    enabled: !!status.data?.available,
  });
  const grounding = useQuery({
    queryKey: ["analytics", "grounding", q.toString()],
    queryFn: () => api.analyticsGrounding(q),
    enabled: !!status.data?.available,
  });
  const efficiency = useQuery({
    queryKey: ["analytics", "efficiency", q.toString()],
    queryFn: () => api.analyticsEfficiency(q),
    enabled: !!status.data?.available && (tab === "efficiency" || tab === "overview"),
  });
  const knowledge = useQuery({
    queryKey: ["analytics", "knowledge", q.toString()],
    queryFn: () => api.analyticsKnowledge(q),
    enabled: !!status.data?.available && (tab === "outcomes" || tab === "quality"),
  });
  const recentQ = useMemo(() => {
    const n = new URLSearchParams(q);
    n.set("limit", "25");
    n.set("offset", String(reqOffset));
    n.set("sort", reqSort);
    n.set("order", reqOrder);
    return n;
  }, [q, reqOffset, reqSort, reqOrder]);
  const recent = useQuery({
    queryKey: ["analytics", "requests", recentQ.toString()],
    queryFn: () => api.analyticsRequests(recentQ),
    enabled: !!status.data?.available,
  });
  const detail = useQuery({
    queryKey: ["analytics", "request", selectedId],
    queryFn: () => api.analyticsRequest(selectedId!),
    enabled: !!selectedId,
  });

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(sp);
    if (!value) next.delete(key);
    else next.set(key, value);
    setSp(next, { replace: true });
  };

  const dataAge = summary.dataUpdatedAt
    ? Math.max(0, Math.round((Date.now() - summary.dataUpdatedAt) / 1000))
    : null;

  const banner = (() => {
    if (status.isError)
      return {
        cls: "analytics-banner err-banner",
        text: "Analytics unavailable. ImplCache retrieval remains operational, but metrics could not be loaded.",
      };
    if (!status.data?.available)
      return {
        cls: "analytics-banner err-banner",
        text: "Analytics unavailable. Check the usage database path and permissions.",
      };
    if (!status.data.enabled)
      return {
        cls: "analytics-banner warn-banner",
        text: "Local analytics disabled. No new usage data is being recorded. Existing analytics remain available until cleared.",
      };
    const path = status.data.dbPath || "./implcache-usage.db";
    const ret = status.data.retentionDays > 0 ? `${status.data.retentionDays} days` : "unlimited";
    return {
      cls: "analytics-banner ok-banner",
      text: `Local analytics enabled · Metadata only · No data leaves this machine · Database: ${path} · Retention: ${ret}`,
    };
  })();

  const roots = useQuery({
    queryKey: ["roots"],
    queryFn: async () => normalizeList<string>(await api.roots(), "roots"),
  });
  const buckets = allowedBuckets(range);
  const bucketVal = sp.get("bucket") || (range === "24h" ? "hour" : "day");

  return (
    <div>
      <PageHead title="Analytics" blurb="Local usage, grounding, and coverage — stays on this machine." />
      <div className="analytics-meta-row">
        <div className={banner.cls}>{banner.text}</div>
        {dataAge != null && status.data?.available && (
          <span className="analytics-updated" title="Time since last successful summary refresh">
            Updated {dataAge}s ago
          </span>
        )}
      </div>

      <div className="tab-row analytics-tabs">
        {(
          [
            ["overview", "Overview"],
            ["usage", "Usage"],
            ["quality", "Retrieval Quality"],
            ["outcomes", "Outcomes"],
            ["efficiency", "Efficiency"],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            className={tab === id ? "tab active" : "tab"}
            onClick={() => setParam("tab", id)}
          >
            {label}
          </button>
        ))}
      </div>

      <div className="panel stack analytics-filters">
        <div className="row wrap">
          <label>
            Range
            <select
              value={range}
              onChange={(e) => {
                setParam("range", e.target.value);
                const nextAllowed = allowedBuckets(e.target.value);
                if (!nextAllowed.includes(bucketVal)) setParam("bucket", nextAllowed[0]);
              }}
            >
              <option value="24h">Last 24h</option>
              <option value="7d">Last 7 days</option>
              <option value="30d">Last 30 days</option>
              <option value="90d">Last 90 days</option>
              <option value="all">All</option>
            </select>
          </label>
          <label>
            Granularity
            <select value={bucketVal} onChange={(e) => setParam("bucket", e.target.value)}>
              <option value="hour" disabled={!buckets.includes("hour")}>
                Hour
              </option>
              <option value="day" disabled={!buckets.includes("day")}>
                Day
              </option>
              <option value="week" disabled={!buckets.includes("week")}>
                Week
              </option>
              <option value="month" disabled={!buckets.includes("month")}>
                Month
              </option>
            </select>
          </label>
          <label>
            Root
            <select value={sp.get("root") || ""} onChange={(e) => setParam("root", e.target.value)}>
              <option value="">All roots</option>
              {(roots.data || []).map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </label>
          <label>
            Tool
            <select value={sp.get("tool") || ""} onChange={(e) => setParam("tool", e.target.value)}>
              <option value="">All tools</option>
              <option value="get_implementation_context">get_implementation_context</option>
              <option value="search_knowledge">search_knowledge</option>
              <option value="find_symbol">find_symbol</option>
              <option value="get_document">get_document</option>
              <option value="search">search (HTTP)</option>
              <option value="search_context">search_context</option>
              <option value="search_symbols">search_symbols</option>
            </select>
          </label>
          <label>
            Coverage
            <select value={sp.get("coverage") || ""} onChange={(e) => setParam("coverage", e.target.value)}>
              <option value="">Any</option>
              <option value="high">high</option>
              <option value="medium">medium</option>
              <option value="low">low</option>
              <option value="unclassified">unclassified</option>
              <option value="not_applicable">not_applicable</option>
            </select>
          </label>
          <label>
            Status
            <select value={sp.get("status") || ""} onChange={(e) => setParam("status", e.target.value)}>
              <option value="">Any</option>
              <option value="grounded_curated">grounded_curated</option>
              <option value="grounded_local">grounded_local</option>
              <option value="grounded_mixed">grounded_mixed</option>
              <option value="root_selection_required">root_selection_required</option>
              <option value="local_insufficient">local_insufficient</option>
              <option value="no_local_match">no_local_match</option>
              <option value="request_error">request_error</option>
            </select>
          </label>
        </div>
      </div>

      {!status.data?.available && (
        <div className="panel stack analytics-content">
          <p className="muted">
            Analytics database is unavailable. Configure the usage DB path or check permissions.
          </p>
        </div>
      )}

      {status.data?.available && !status.data.enabled && (
        <div className="panel stack analytics-content">
          <p className="muted">
            Recording is off. Existing data below remains until cleared. Enable analytics in{" "}
            <Link to="/settings">Settings</Link>.
          </p>
        </div>
      )}

      {status.data?.available && summary.isError && (
        <div className="panel stack analytics-content">
          <p className="error">Failed to load summary.</p>
        </div>
      )}

      {status.data?.available && summary.data && summary.data.totalRequests === 0 && (
        <div className="panel stack analytics-content">
          <p className="muted">No usage events yet. Run retrieval tools or Search Lab to generate data.</p>
        </div>
      )}

      {summary.data && summary.data.totalRequests > 0 && (
        <>
          {(tab === "overview" || tab === "usage") && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Summary</h2>
              <SummaryCards s={summary.data} />
            </div>
          )}
          {(tab === "overview" || tab === "usage") && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Requests over time</h2>
              <RequestsOverTime points={series.data?.points || []} />
            </div>
          )}
          {(tab === "overview" || tab === "quality") && coverage.data && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Coverage</h2>
              <CoverageBars
                c={coverage.data}
                onFilterUnclassified={() => {
                  setParam("coverage", "unclassified");
                  setParam("tab", "usage");
                }}
              />
            </div>
          )}
          {(tab === "overview" || tab === "quality") && grounding.data && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Request Outcomes &amp; Evidence Usage</h2>
              <OutcomeAndEvidence g={grounding.data} />
            </div>
          )}
          {(tab === "overview" || tab === "usage") && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Recent requests</h2>
              <RecentRequestsTable
                rows={recent.data?.requests || []}
                total={recent.data?.total || 0}
                offset={recent.data?.offset || 0}
                limit={recent.data?.limit || 25}
                sort={reqSort}
                order={reqOrder}
                onOpen={(id) => setSelectedId(id)}
                onSort={(col) => {
                  if (reqSort === col) setParam("rorder", reqOrder === "asc" ? "desc" : "asc");
                  else {
                    setParam("rsort", col);
                    setParam("rorder", "desc");
                  }
                  setParam("roffset", "0");
                }}
                onPage={(off) => setParam("roffset", String(off))}
              />
            </div>
          )}
          {tab === "outcomes" && grounding.data && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Outcomes</h2>
              <OutcomeAndEvidence g={grounding.data} />
              {knowledge.data && (
                <>
                  <h2 className="section-title">Knowledge usage</h2>
                  <KnowledgePanel k={knowledge.data} />
                </>
              )}
            </div>
          )}
          {tab === "efficiency" && (
            <div className="panel stack analytics-content">
              <h2 className="section-title">Context efficiency</h2>
              {efficiency.isLoading && <p className="muted">Loading…</p>}
              {efficiency.data && <EfficiencyPanel e={efficiency.data} />}
            </div>
          )}
        </>
      )}

      {selectedId && (
        <RequestDrilldown
          detail={detail.data}
          loading={detail.isLoading}
          onClose={() => setSelectedId(null)}
        />
      )}
    </div>
  );
}

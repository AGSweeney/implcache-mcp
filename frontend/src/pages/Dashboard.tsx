import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, normalizeList, type HealthIssue, type Operation } from "../api";
import PageHead from "../PageHead";
import StatusBadge from "../components/StatusBadge";
import { formatBytes } from "../format";
import type { ReactNode } from "react";

type MetricDef = {
  label: string;
  value: string | number | undefined;
  accent: string;
  icon: ReactNode;
  group: "sources" | "corpus" | "jobs";
};

function IconSources() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M4 6.5h16M4 12h16M4 17.5h10" strokeLinecap="round" />
      <circle cx="18.5" cy="17.5" r="1.5" fill="currentColor" stroke="none" />
    </svg>
  );
}
function IconOk() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M20 7L10 17l-5-5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconFail() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 8v5M12 16.5h.01" strokeLinecap="round" />
      <path d="M10.3 4.8 2.9 18a2 2 0 0 0 1.7 3h14.8a2 2 0 0 0 1.7-3L13.7 4.8a2 2 0 0 0-3.4 0Z" strokeLinejoin="round" />
    </svg>
  );
}
function IconDocs() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M7 3.5h7l4 4V20a1.5 1.5 0 0 1-1.5 1.5H7A1.5 1.5 0 0 1 5.5 20V5A1.5 1.5 0 0 1 7 3.5Z" />
      <path d="M14 3.5V8h4.5M8.5 12h7M8.5 15.5h5" strokeLinecap="round" />
    </svg>
  );
}
function IconChunks() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <rect x="3.5" y="3.5" width="7" height="7" rx="1.2" />
      <rect x="13.5" y="3.5" width="7" height="7" rx="1.2" />
      <rect x="3.5" y="13.5" width="7" height="7" rx="1.2" />
      <rect x="13.5" y="13.5" width="7" height="7" rx="1.2" />
    </svg>
  );
}
function IconSymbols() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M8 5.5 4.5 12 8 18.5M16 5.5 19.5 12 16 18.5M13.2 5.5l-2.4 13" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconRecipes() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6 4.5h9.5L19 8v11.5A1.5 1.5 0 0 1 17.5 21H6A1.5 1.5 0 0 1 4.5 19.5v-13A1.5 1.5 0 0 1 6 4.5Z" />
      <path d="M8.5 11h7M8.5 14.5h5" strokeLinecap="round" />
    </svg>
  );
}
function IconDb() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <ellipse cx="12" cy="6" rx="7.5" ry="2.8" />
      <path d="M4.5 6v6c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8V6" />
      <path d="M4.5 12v6c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8v-6" />
    </svg>
  );
}
function IconJobs() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M5 7h14v11.5A1.5 1.5 0 0 1 17.5 20h-11A1.5 1.5 0 0 1 5 18.5V7Z" />
      <path d="M9 7V5.5A1.5 1.5 0 0 1 10.5 4h3A1.5 1.5 0 0 1 15 5.5V7M5 11h14" strokeLinecap="round" />
    </svg>
  );
}
function IconShield() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 3.5 5.5 6v5.5c0 4.2 2.8 7.6 6.5 9 3.7-1.4 6.5-4.8 6.5-9V6L12 3.5Z" strokeLinejoin="round" />
      <path d="M9.5 12.2 11.2 14l3.4-3.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconActivity() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M3.5 12h4l2-5.5L13.5 17l2.5-5H20.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function jobBadgeVariant(state: string): "success" | "danger" | "info" | "neutral" | "warning" {
  if (state === "ok" || state === "succeeded" || state === "completed") return "success";
  if (state === "failed" || state === "error") return "danger";
  if (state === "running" || state === "active") return "info";
  if (state === "queued" || state === "pending") return "warning";
  return "neutral";
}

function issueVariant(severity: string): "danger" | "warning" | "info" | "neutral" {
  if (severity === "error") return "danger";
  if (severity === "warning" || severity === "warn") return "warning";
  if (severity === "info") return "info";
  return "neutral";
}

export default function Dashboard() {
  const stats = useQuery({ queryKey: ["stats"], queryFn: api.stats, refetchInterval: 10000 });
  const health = useQuery({
    queryKey: ["health"],
    queryFn: async () => normalizeList<HealthIssue>(await api.health(), "issues"),
    refetchInterval: 15000,
  });
  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: async () => {
      const raw = await api.jobs();
      return normalizeList<Operation>(raw, "operations");
    },
    refetchInterval: 5000,
  });

  const s = stats.data;
  const issues = (health.data || []).filter((i) => i.severity !== "info").slice(0, 8);
  const recent = (jobs.data || []).slice(0, 8);

  const metrics: MetricDef[] = [
    { label: "Sources", value: s?.sourcesTotal, accent: "var(--accent)", icon: <IconSources />, group: "sources" },
    { label: "Healthy", value: s?.sourcesOk, accent: "var(--success)", icon: <IconOk />, group: "sources" },
    { label: "Failed", value: s?.sourcesFailed, accent: "var(--danger)", icon: <IconFail />, group: "sources" },
    { label: "Documents", value: s?.documents, accent: "var(--copper)", icon: <IconDocs />, group: "corpus" },
    { label: "Chunks", value: s?.chunks, accent: "var(--accent-hover)", icon: <IconChunks />, group: "corpus" },
    { label: "Symbols", value: s?.symbols, accent: "var(--info)", icon: <IconSymbols />, group: "corpus" },
    { label: "Recipes", value: s?.recipes, accent: "var(--copper-soft)", icon: <IconRecipes />, group: "corpus" },
    {
      label: "Database",
      value: s ? formatBytes(s.databaseBytes) : undefined,
      accent: "var(--muted)",
      icon: <IconDb />,
      group: "corpus",
    },
    { label: "Active jobs", value: s?.activeJobs, accent: "var(--warning)", icon: <IconJobs />, group: "jobs" },
  ];

  return (
    <div className="dashboard">
      <PageHead title="Dashboard" blurb="Library posture, health signals, and recent activity." />
      {stats.isError && <div className="error-box">{(stats.error as Error).message}</div>}

      <section className="dash-metrics-block" aria-label="Library metrics">
        <div className="dash-metric-groups" aria-hidden="true">
          <span>Sources</span>
          <span>Corpus</span>
          <span>Jobs</span>
        </div>
        <div className="grid metrics-grid">
          {metrics.map((m) => (
            <div
              className="metric"
              key={m.label}
              data-group={m.group}
              style={{ ["--metric-accent" as string]: m.accent }}
            >
              <div className="metric-icon">{m.icon}</div>
              <div className="n">{m.value ?? "—"}</div>
              <div className="l">{m.label}</div>
            </div>
          ))}
        </div>
      </section>

      <div className="dash-split">
        <section className="op-module" aria-labelledby="library-health-title">
          <div className="op-module-head">
            <h2 id="library-health-title">Library Health</h2>
            <Link to="/health" className="btn btn-ghost">
              Details
            </Link>
          </div>
          <div className="op-module-body">
            {health.isLoading && <p className="muted">Checking library…</p>}
            {!health.isLoading && issues.length === 0 && (
              <div className="op-empty">
                <div className="op-empty-icon">
                  <IconShield />
                </div>
                <strong>Library looks clear</strong>
                <p>No warnings or errors in the current health scan.</p>
              </div>
            )}
            {issues.length > 0 && (
              <ul className="attention-list">
                {issues.map((i, idx) => (
                  <li key={idx}>
                    <StatusBadge variant={issueVariant(i.severity)}>{i.severity}</StatusBadge>
                    <span>
                      {i.description}
                      {i.sourceId && (
                        <>
                          {" "}
                          <span className="mono muted">
                            {i.sourceKind}/{i.sourceId}
                          </span>
                        </>
                      )}
                      {i.recommendedAction && <div className="muted">{i.recommendedAction}</div>}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="op-module-foot">
            <Link to="/health" className="btn btn-primary">
              Open Health
            </Link>
          </div>
        </section>

        <section className="op-module" aria-labelledby="recent-activity-title">
          <div className="op-module-head">
            <h2 id="recent-activity-title">Recent Activity</h2>
            <Link to="/jobs" className="btn btn-ghost">
              All jobs
            </Link>
          </div>
          <div className="op-module-body">
            {jobs.isLoading && <p className="muted">Loading jobs…</p>}
            {!jobs.isLoading && recent.length === 0 && (
              <div className="op-empty">
                <div className="op-empty-icon">
                  <IconActivity />
                </div>
                <strong>No jobs yet</strong>
                <p>Ingest and refresh operations will show up here.</p>
              </div>
            )}
            {recent.length > 0 && (
              <table className="data-table compact">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>Source</th>
                    <th>State</th>
                    <th>Phase</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.map((j) => (
                    <tr key={j.opId}>
                      <td className="mono">
                        <Link to="/jobs">{j.opId.slice(0, 8)}</Link>
                      </td>
                      <td>
                        {j.source.kind}/{j.source.id}
                      </td>
                      <td>
                        <StatusBadge variant={jobBadgeVariant(j.state)}>{j.state}</StatusBadge>
                      </td>
                      <td>{j.progress?.phase || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          {recent.length === 0 && !jobs.isLoading && (
            <div className="op-module-foot">
              <Link to="/jobs" className="btn btn-primary">
                Open Jobs
              </Link>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

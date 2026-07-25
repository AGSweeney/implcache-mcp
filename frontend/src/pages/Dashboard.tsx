import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, normalizeList, type HealthIssue } from "../api";
import PageHead from "../PageHead";
import { formatBytes } from "../format";

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
      return normalizeList(raw, "operations");
    },
    refetchInterval: 5000,
  });

  const s = stats.data;
  const issues = (health.data || []).filter((i) => i.severity !== "info").slice(0, 8);

  return (
    <div>
      <PageHead title="Dashboard" blurb="Library posture, attention items, and recent jobs." />
      {stats.isError && <div className="error-box">{(stats.error as Error).message}</div>}
      <div className="panel plain">
        <div className="grid">
          {[
            ["Sources", s?.sourcesTotal],
            ["Healthy", s?.sourcesOk],
            ["Failed", s?.sourcesFailed],
            ["Documents", s?.documents],
            ["Chunks", s?.chunks],
            ["Symbols", s?.symbols],
            ["Recipes", s?.recipes],
            ["Database", s ? formatBytes(s.databaseBytes) : undefined],
            ["Active jobs", s?.activeJobs],
          ].map(([label, n]) => (
            <div className="metric" key={String(label)}>
              <div className="n">{n ?? "—"}</div>
              <div className="l">{label}</div>
            </div>
          ))}
        </div>
      </div>

      <div className="dash-split">
        <div className="panel">
          <h2 className="section-title">Attention</h2>
          {issues.length === 0 && <p className="muted">Library looks clear — no warnings or errors.</p>}
          {issues.length > 0 && (
            <ul className="attention-list">
              {issues.map((i, idx) => (
                <li key={idx}>
                  <span className={`badge ${i.severity === "error" ? "err" : "warn"}`}>{i.severity}</span>
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
          <p style={{ marginTop: "0.75rem" }}>
            <Link to="/health">Open Health</Link>
          </p>
        </div>

        <div className="panel">
          <h2 className="section-title">Recent jobs</h2>
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Source</th>
                <th>State</th>
                <th>Phase</th>
              </tr>
            </thead>
            <tbody>
              {(jobs.data || []).slice(0, 8).map((j) => (
                <tr key={j.opId}>
                  <td className="mono">
                    <Link to="/jobs">{j.opId.slice(0, 8)}</Link>
                  </td>
                  <td>
                    {j.source.kind}/{j.source.id}
                  </td>
                  <td>
                    <span className={`badge ${j.state === "ok" ? "ok" : j.state === "failed" ? "err" : ""}`}>{j.state}</span>
                  </td>
                  <td>{j.progress?.phase || "—"}</td>
                </tr>
              ))}
              {(jobs.data || []).length === 0 && (
                <tr>
                  <td colSpan={4} className="muted">
                    No jobs yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

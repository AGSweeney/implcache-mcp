import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api, normalizeList, type HealthIssue } from "../api";

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
      <h1>Dashboard</h1>
      {stats.isError && <div className="error-box">{(stats.error as Error).message}</div>}
      <div className="grid-metrics panel">
        {[
          ["Sources", s?.sourcesTotal],
          ["Healthy", s?.sourcesOk],
          ["Failed", s?.sourcesFailed],
          ["Documents", s?.documents],
          ["Chunks", s?.chunks],
          ["Symbols", s?.symbols],
          ["Recipes", s?.recipes],
          ["DB bytes", s?.databaseBytes],
          ["Active jobs", s?.activeJobs],
        ].map(([label, n]) => (
          <div className="metric" key={String(label)}>
            <div className="n">{n ?? "—"}</div>
            <div className="l">{label}</div>
          </div>
        ))}
      </div>

      <div className="panel">
        <h2>Attention</h2>
        {issues.length === 0 && <p className="muted">No warnings or errors.</p>}
        <ul>
          {issues.map((i, idx) => (
            <li key={idx}>
              <span className={`badge ${i.severity === "error" ? "err" : "warn"}`}>{i.severity}</span>{" "}
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
            </li>
          ))}
        </ul>
        <Link to="/health">Open Health</Link>
      </div>

      <div className="panel">
        <h2>Recent jobs</h2>
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
          </tbody>
        </table>
      </div>
    </div>
  );
}

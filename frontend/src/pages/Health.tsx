import { useQuery } from "@tanstack/react-query";
import { api, normalizeList, type HealthIssue } from "../api";
import PageHead from "../PageHead";

export default function Health() {
  const health = useQuery({
    queryKey: ["health"],
    queryFn: async () => normalizeList<HealthIssue>(await api.health(), "issues"),
    refetchInterval: 15000,
  });

  return (
    <div>
      <PageHead title="Health" blurb="Library-wide issues and recommended actions." />
      {health.isError && <div className="error-box">{(health.error as Error).message}</div>}
      <div className="panel">
        <table>
          <thead>
            <tr>
              <th>Severity</th>
              <th>Code</th>
              <th>Source</th>
              <th>Description</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {(health.data || []).map((i, idx) => (
              <tr key={idx}>
                <td>
                  <span className={`badge ${i.severity === "error" ? "err" : i.severity === "warning" ? "warn" : ""}`}>
                    {i.severity}
                  </span>
                </td>
                <td className="mono">{i.code}</td>
                <td className="mono">
                  {i.sourceKind}/{i.sourceId}
                </td>
                <td>{i.description}</td>
                <td className="muted">{i.recommendedAction}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {(health.data || []).length === 0 && <p className="muted">No issues reported.</p>}
      </div>
    </div>
  );
}

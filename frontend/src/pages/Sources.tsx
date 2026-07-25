import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api, normalizeList, type SourceSummary } from "../api";
import { useMemo, useState } from "react";

export default function Sources() {
  const qc = useQueryClient();
  const nav = useNavigate();
  const [typeFilter, setTypeFilter] = useState("");
  const q = useQuery({
    queryKey: ["sources"],
    queryFn: async () => normalizeList<SourceSummary>(await api.sources(), "sources"),
    refetchInterval: 10000,
  });

  const rows = useMemo(() => {
    let list = q.data || [];
    if (typeFilter) list = list.filter((s) => s.kind === typeFilter);
    return list;
  }, [q.data, typeFilter]);

  const refresh = useMutation({
    mutationFn: async (s: SourceSummary) => {
      if (s.kind === "web") return api.refreshWeb(s.id);
      if (s.kind === "repo") return api.refreshGit(s.id);
      throw new Error("refresh not supported for " + s.kind);
    },
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ["jobs"] });
      const opId = (res as { opId?: string } | undefined)?.opId;
      nav(opId ? `/jobs?op=${encodeURIComponent(opId)}` : "/jobs");
    },
  });

  const remove = useMutation({
    mutationFn: async (s: SourceSummary) => {
      if (!confirm(`Remove ${s.kind}/${s.id}? Indexed content may be deleted.`)) return;
      if (s.kind === "web") return api.deleteWeb(s.id);
      if (s.kind === "repo") return api.deleteGit(s.id);
      if (s.kind === "pdf") return api.deletePdf(s.id);
      throw new Error("remove not supported for " + s.kind);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["sources"] }),
  });

  return (
    <div>
      <h1>Sources</h1>
      <div className="row panel">
        <label>
          Type
          <select value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
            <option value="">All</option>
            <option value="web">web</option>
            <option value="repo">repo</option>
            <option value="pdf">pdf</option>
            <option value="local">local</option>
          </select>
        </label>
        <div>
          <Link to="/sources/add">
            <button className="primary" type="button">
              Add Source
            </button>
          </Link>
        </div>
      </div>
      {q.isError && <div className="error-box">{(q.error as Error).message}</div>}
      <div className="panel">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Root</th>
              <th>Status</th>
              <th>Docs</th>
              <th>Chunks</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((s) => (
              <tr key={`${s.kind}:${s.id}`}>
                <td>
                  <div>{s.title || s.id}</div>
                  <div className="mono muted">{s.id}</div>
                </td>
                <td>
                  <span className="badge">{s.kind}</span>
                </td>
                <td className="mono">{s.rootName}</td>
                <td>
                  <span
                    className={`badge ${
                      (s.lastStatus || "").startsWith("fail")
                        ? "err"
                        : s.lastStatus === "ok" || s.lastStatus === "ingested"
                          ? "ok"
                          : ""
                    }`}
                  >
                    {s.lastStatus || "idle"}
                  </span>
                </td>
                <td>{s.documentCount}</td>
                <td>{s.chunkCount}</td>
                <td className="stack">
                  {(s.kind === "web" || s.kind === "repo") && (
                    <button type="button" onClick={() => refresh.mutate(s)}>
                      Refresh
                    </button>
                  )}
                  {s.kind !== "local" && (
                    <button type="button" className="danger" onClick={() => remove.mutate(s)}>
                      Remove
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && <p className="muted">No sources yet.</p>}
      </div>
    </div>
  );
}

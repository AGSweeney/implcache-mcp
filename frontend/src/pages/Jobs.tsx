import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, getToken, normalizeList, type Operation } from "../api";
import EmptyState from "../components/EmptyState";
import ErrorState from "../components/ErrorState";
import StatusBadge from "../components/StatusBadge";
import PageHead from "../PageHead";

function formatProgress(p?: Operation["progress"]) {
  if (!p) return "";
  const parts: string[] = [];
  if (p.phase) parts.push(p.phase);
  if (p.done != null) parts.push(p.total ? `${p.done}/${p.total}` : String(p.done));
  if (p.current) parts.push(p.current);
  if (p.message) parts.push(p.message);
  return parts.join(" · ");
}

export default function Jobs() {
  const qc = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [selected, setSelected] = useState(params.get("op") || "");
  const [live, setLive] = useState("");

  const jobs = useQuery({
    queryKey: ["jobs"],
    queryFn: async () => normalizeList<Operation>(await api.jobs(), "operations"),
    refetchInterval: (q) => {
      const list = q.state.data || [];
      return list.some((j) => j.state === "running" || j.state === "cancelling") ? 1000 : 4000;
    },
  });

  useEffect(() => {
    const fromUrl = params.get("op") || "";
    if (fromUrl) {
      setSelected(fromUrl);
      return;
    }
    const running = (jobs.data || []).find((j) => j.state === "running" || j.state === "cancelling");
    if (running && !selected) setSelected(running.opId);
  }, [params, jobs.data, selected]);

  useEffect(() => {
    if (!selected) return;
    let poll: ReturnType<typeof setInterval> | undefined;
    const token = getToken();
    const q = token ? `?access_token=${encodeURIComponent(token)}` : "";
    const es = new EventSource(`/api/v1/jobs/${selected}/events${q}`);

    const apply = (op: Partial<Operation> & { progress?: Operation["progress"] }) => {
      setLive(JSON.stringify(op.progress ?? op, null, 2));
      qc.invalidateQueries({ queryKey: ["jobs"] });
    };

    es.addEventListener("progress", (ev) => {
      try {
        apply({ progress: JSON.parse((ev as MessageEvent).data) });
      } catch {
        setLive((ev as MessageEvent).data);
      }
    });

    // Always poll; SSE accelerates updates when healthy.
    poll = setInterval(async () => {
      try {
        const op = await api.job(selected);
        apply(op);
        if (op.state !== "running" && op.state !== "queued" && op.state !== "cancelling") {
          if (poll) clearInterval(poll);
          poll = undefined;
        }
      } catch {
        /* ignore */
      }
    }, 1000);

    es.onerror = () => {
      es.close();
    };

    return () => {
      es.close();
      if (poll) clearInterval(poll);
    };
  }, [selected, qc]);

  function selectJob(id: string) {
    setSelected(id);
    setParams(id ? { op: id } : {});
  }

  function jobVariant(state: string): "success" | "danger" | "info" | "neutral" | "warning" {
    if (state === "ok" || state === "succeeded" || state === "completed") return "success";
    if (state === "failed" || state === "error") return "danger";
    if (state === "running" || state === "active" || state === "cancelling") return "info";
    if (state === "queued" || state === "pending") return "warning";
    return "neutral";
  }

  const list = jobs.data || [];

  return (
    <div>
      <PageHead title="Jobs" blurb="Live ingest and refresh operations." />
      {jobs.isError && (
        <ErrorState message={(jobs.error as Error).message} onRetry={() => void jobs.refetch()} />
      )}
      {!jobs.isLoading && !jobs.isError && list.length === 0 && (
        <EmptyState
          title="No jobs yet"
          body="Ingestion and refresh jobs will appear here after a source is added."
        />
      )}
      {list.length > 0 && (
        <div className="panel">
          <div className="data-table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Source</th>
                  <th>State</th>
                  <th>Progress</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {list.map((j) => (
                  <tr key={j.opId} className={j.opId === selected ? "selected" : undefined}>
                    <td className="mono">
                      <button type="button" className="linkish" onClick={() => selectJob(j.opId)}>
                        {j.opId.slice(0, 8)}
                      </button>
                    </td>
                    <td>
                      {j.source?.kind}/{j.source?.id}
                    </td>
                    <td>
                      <StatusBadge variant={jobVariant(j.state)}>{j.state}</StatusBadge>
                    </td>
                    <td>{formatProgress(j.progress)}</td>
                    <td>
                      {(j.state === "running" || j.state === "cancelling") && (
                        <button
                          type="button"
                          onClick={() => api.cancelJob(j.opId).then(() => qc.invalidateQueries({ queryKey: ["jobs"] }))}
                        >
                          Cancel
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      <div className="panel">
        <h2>Live</h2>
        <pre className="muted">{live || "Select a job…"}</pre>
      </div>
    </div>
  );
}

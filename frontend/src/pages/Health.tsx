import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { api, normalizeList, type HealthIssue } from "../api";
import Button from "../components/Button";
import ConfirmDialog from "../components/ConfirmDialog";
import PageHeader from "../components/PageHeader";
import StatusBadge from "../components/StatusBadge";
import { useToast } from "../components/Toast";

function sourceLabel(i: HealthIssue): string {
  if (!i.sourceKind && !i.sourceId) return "library";
  if (i.sourceKind === "library") {
    return i.sourceId && i.sourceId !== "all" ? `library / ${i.sourceId}` : "library";
  }
  if (i.sourceKind && i.sourceId) return `${i.sourceKind}/${i.sourceId}`;
  return i.sourceKind || i.sourceId || "library";
}

function severityVariant(severity: string): "danger" | "warning" | "info" | "neutral" {
  if (severity === "error") return "danger";
  if (severity === "warning" || severity === "warn") return "warning";
  if (severity === "info") return "info";
  return "neutral";
}

export default function Health() {
  const toast = useToast();
  const qc = useQueryClient();
  const [confirmPurge, setConfirmPurge] = useState(false);

  const health = useQuery({
    queryKey: ["health"],
    queryFn: async () => normalizeList<HealthIssue>(await api.health(), "issues"),
    refetchInterval: 15000,
  });

  const emptyChunkIssue = (health.data || []).find((i) => i.code === "documents_without_chunks");

  const purge = useMutation({
    mutationFn: () => api.purgeEmptyDocs(),
    onSuccess: (res) => {
      setConfirmPurge(false);
      toast.push({
        variant: "success",
        message: `Purged ${res.deleted} chunkless document${res.deleted === 1 ? "" : "s"}`,
      });
      void qc.invalidateQueries({ queryKey: ["health"] });
      void qc.invalidateQueries({ queryKey: ["stats"] });
      void qc.invalidateQueries({ queryKey: ["documents"] });
    },
    onError: (err) => {
      toast.push({ variant: "danger", message: (err as Error).message || "Purge failed" });
    },
  });

  return (
    <div>
      <PageHeader
        title="Health"
        subtitle="Library-wide issues and recommended actions."
        actions={
          <Button
            variant="danger"
            disabled={purge.isPending}
            onClick={() => setConfirmPurge(true)}
            title="Delete documents that have no searchable chunks"
          >
            Purge chunkless docs
          </Button>
        }
      />
      {health.isError && <div className="error-box">{(health.error as Error).message}</div>}

      {emptyChunkIssue && (
        <div className="panel health-callout">
          <div className="health-callout-text">
            <StatusBadge variant="warning">warning</StatusBadge>
            <p>
              Chunkless documents are ingest stubs with no searchable text. Purge them to clear this
              warning, or re-ingest/remove the listed roots.
            </p>
          </div>
          <Button variant="danger" disabled={purge.isPending} onClick={() => setConfirmPurge(true)}>
            Purge chunkless docs
          </Button>
        </div>
      )}

      <div className="panel">
        <table className="data-table">
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
                  <StatusBadge variant={severityVariant(i.severity)}>{i.severity}</StatusBadge>
                </td>
                <td className="mono">{i.code}</td>
                <td className="mono">{sourceLabel(i)}</td>
                <td className="health-desc">{i.description}</td>
                <td>
                  <div className="health-action muted">{i.recommendedAction}</div>
                  {i.code === "documents_without_chunks" && (
                    <div className="health-action-links">
                      <button
                        type="button"
                        className="linkish"
                        disabled={purge.isPending}
                        onClick={() => setConfirmPurge(true)}
                      >
                        Purge now
                      </button>
                      {" · "}
                      <Link to="/sources">Open Sources</Link>
                      {" · "}
                      <Link to="/library">Open Library</Link>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {(health.data || []).length === 0 && <p className="muted">No issues reported.</p>}
      </div>

      <ConfirmDialog
        open={confirmPurge}
        title="Purge chunkless documents?"
        confirmLabel="Purge"
        busy={purge.isPending}
        onCancel={() => !purge.isPending && setConfirmPurge(false)}
        onConfirm={() => purge.mutate()}
        body={
          <>
            <p>
              Delete every document that has <strong>no chunks</strong> (ingest stubs / non-text
              files that never produced searchable content).
            </p>
            <ul>
              <li>Documents with chunks are kept.</li>
              <li>This cannot be undone from the UI.</li>
              {emptyChunkIssue && <li>Current warning: {emptyChunkIssue.description}</li>}
            </ul>
          </>
        }
      />
    </div>
  );
}

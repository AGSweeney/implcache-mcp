import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, type Doc } from "../api";
import Button from "../components/Button";
import DocumentViewer from "../components/DocumentViewer";
import EmptyState from "../components/EmptyState";
import ErrorState from "../components/ErrorState";
import LoadingSkeleton from "../components/LoadingSkeleton";
import RootSelect from "../components/RootSelect";
import PageHead from "../PageHead";

export default function Library() {
  const [params] = useSearchParams();
  const rootFromQuery = params.get("root") || "";
  const [root, setRoot] = useState(rootFromQuery);
  const [sourceType, setSourceType] = useState("");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Doc | null>(null);
  const limit = 40;

  useEffect(() => {
    setRoot(rootFromQuery);
    setOffset(0);
  }, [rootFromQuery]);

  const docs = useQuery({
    queryKey: ["documents", root, sourceType, offset],
    queryFn: async () => {
      const q = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      if (root) q.set("root", root);
      if (sourceType) q.set("sourceType", sourceType);
      return api.documents(q);
    },
  });

  const list: Doc[] = docs.data?.documents || [];
  const total = docs.data?.total ?? 0;

  return (
    <div>
      <PageHead title="Library" blurb="Browse indexed documents and open bounded detail." />
      <div className="row panel toolbar-panel">
        <RootSelect
          value={root}
          onChange={(r) => {
            setRoot(r);
            setOffset(0);
          }}
          allowAll
        />
        <label>
          Source type
          <select
            value={sourceType}
            onChange={(e) => {
              setSourceType(e.target.value);
              setOffset(0);
            }}
          >
            <option value="">All</option>
            <option value="markdown">markdown</option>
            <option value="source">source</option>
            <option value="web">web</option>
            <option value="pdf">pdf</option>
            <option value="git">git</option>
          </select>
        </label>
      </div>

      {docs.isError && (
        <ErrorState message={(docs.error as Error).message} onRetry={() => void docs.refetch()} />
      )}
      {docs.isLoading && <LoadingSkeleton />}
      {!docs.isLoading && !docs.isError && list.length === 0 && (
        <EmptyState
          title="No documents"
          body="No indexed documents match this root and source type. Ingest a source or clear filters."
        />
      )}

      {!docs.isLoading && !docs.isError && list.length > 0 && (
        <div className="panel">
          <div className="data-table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Title</th>
                  <th>URI</th>
                  <th>Type</th>
                  <th>Root</th>
                  <th>Version</th>
                </tr>
              </thead>
              <tbody>
                {list.map((d) => (
                  <tr
                    key={d.id}
                    onClick={() => setSelected(d)}
                    className={`is-clickable ${selected?.id === d.id ? "selected" : ""}`}
                  >
                    <td>{d.title}</td>
                    <td className="mono">{d.uri}</td>
                    <td>{d.sourceType}</td>
                    <td>{d.rootName}</td>
                    <td>{d.productVersion || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="pager-row">
            <Button
              variant="secondary"
              className="pager-btn"
              disabled={offset <= 0}
              aria-label="Previous page"
              title="Previous page"
              onClick={() => setOffset(Math.max(0, offset - limit))}
            >
              ← Prev
            </Button>
            <span className="muted mono">
              {offset + 1}–{Math.min(offset + limit, total)} of {total}
            </span>
            <Button
              variant="secondary"
              className="pager-btn"
              disabled={offset + limit >= total}
              aria-label="Next page"
              title="Next page"
              onClick={() => setOffset(offset + limit)}
            >
              Next →
            </Button>
          </div>
        </div>
      )}

      <DocumentViewer doc={selected} onClose={() => setSelected(null)} />
    </div>
  );
}

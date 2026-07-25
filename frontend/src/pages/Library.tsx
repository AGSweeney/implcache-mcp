import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, type Doc } from "../api";
import Button from "../components/Button";
import CodeBlock, { detectLanguage } from "../components/CodeBlock";
import Modal from "../components/Modal";
import PageHead from "../PageHead";

type DocChunk = {
  id?: number;
  ordinal?: number;
  heading?: string;
  body?: string;
  startLine?: number;
  endLine?: number;
};

type DocDetail = {
  document?: Doc & Record<string, unknown>;
  chunks?: DocChunk[];
  totalChunks?: number;
  truncated?: boolean;
};

function formatSource(detail: DocDetail | undefined): string {
  if (!detail?.chunks?.length) return "";
  return detail.chunks
    .map((c) => {
      const body = c.body || "";
      if (c.heading) return `// ${c.heading}\n${body}`;
      return body;
    })
    .join("\n\n");
}

export default function Library() {
  const [params] = useSearchParams();
  const rootFromQuery = params.get("root") || "";
  const [root, setRoot] = useState(rootFromQuery);
  const [sourceType, setSourceType] = useState("");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Doc | null>(null);
  const [showRaw, setShowRaw] = useState(false);
  const limit = 40;

  useEffect(() => {
    setRoot(rootFromQuery);
    setOffset(0);
  }, [rootFromQuery]);

  useEffect(() => {
    setShowRaw(false);
  }, [selected?.id]);

  const docs = useQuery({
    queryKey: ["documents", root, sourceType, offset],
    queryFn: async () => {
      const q = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      if (root) q.set("root", root);
      if (sourceType) q.set("sourceType", sourceType);
      return api.documents(q);
    },
  });

  const detail = useQuery({
    queryKey: ["document", selected?.id],
    queryFn: () => api.document(selected!.id) as Promise<DocDetail>,
    enabled: selected != null,
  });

  const list: Doc[] = docs.data?.documents || [];
  const total = docs.data?.total ?? 0;
  const detailTitle = selected?.title || (selected ? `Document ${selected.id}` : "Document");
  const sourceText = useMemo(() => formatSource(detail.data), [detail.data]);
  const meta = detail.data?.document || selected;
  const language = useMemo(
    () => detectLanguage(meta?.uri || selected?.uri, meta?.sourceType || selected?.sourceType, meta?.path || selected?.path),
    [meta, selected],
  );

  return (
    <div>
      <PageHead title="Library" blurb="Browse indexed documents and open bounded detail." />
      <div className="row panel">
        <label>
          Root
          <input
            value={root}
            onChange={(e) => {
              setRoot(e.target.value);
              setOffset(0);
            }}
            placeholder="rootName"
          />
        </label>
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
      <div className="panel">
        <table>
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
                style={{ cursor: "pointer" }}
                className={selected?.id === d.id ? "selected" : undefined}
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
        <div className="row">
          <button type="button" disabled={offset <= 0} onClick={() => setOffset(Math.max(0, offset - limit))}>
            Prev
          </button>
          <span className="muted">
            {offset + 1}–{Math.min(offset + limit, total)} of {total}
          </span>
          <button type="button" disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>
            Next
          </button>
        </div>
      </div>

      <Modal open={selected != null} title={detailTitle} onClose={() => setSelected(null)} wide>
        {selected && (
          <>
            <dl className="drawer-meta">
              <div>
                <dt>ID</dt>
                <dd className="mono">{selected.id}</dd>
              </div>
              <div>
                <dt>URI</dt>
                <dd className="mono">{meta?.uri || selected.uri}</dd>
              </div>
              <div>
                <dt>Type</dt>
                <dd>{meta?.sourceType || selected.sourceType}</dd>
              </div>
              <div>
                <dt>Root</dt>
                <dd className="mono">{meta?.rootName || selected.rootName || "—"}</dd>
              </div>
              <div>
                <dt>Version</dt>
                <dd>{meta?.productVersion || selected.productVersion || "—"}</dd>
              </div>
              {detail.data?.totalChunks != null && (
                <div>
                  <dt>Chunks</dt>
                  <dd>
                    {detail.data.chunks?.length ?? 0}
                    {detail.data.truncated ? ` of ${detail.data.totalChunks} (preview truncated)` : ` / ${detail.data.totalChunks}`}
                  </dd>
                </div>
              )}
            </dl>

            <div className="doc-view-toolbar">
              <Button variant={showRaw ? "secondary" : "primary"} onClick={() => setShowRaw(false)}>
                Source
              </Button>
              <Button variant={showRaw ? "primary" : "secondary"} onClick={() => setShowRaw(true)}>
                Raw JSON
              </Button>
              {!showRaw && <span className="doc-lang-badge">{language}</span>}
            </div>

            {detail.isLoading && <p className="muted">Loading detail…</p>}
            {detail.isError && <p className="error-box">{(detail.error as Error).message}</p>}
            {!detail.isLoading && !detail.isError && !showRaw && (
              <CodeBlock code={sourceText || "(no chunk text)"} language={language} />
            )}
            {!detail.isLoading && !detail.isError && showRaw && (
              <CodeBlock code={JSON.stringify(detail.data, null, 2)} language="json" />
            )}
          </>
        )}
      </Modal>
    </div>
  );
}

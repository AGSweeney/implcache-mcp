import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, type Doc } from "../api";
import DocumentViewer from "../components/DocumentViewer";
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

      <DocumentViewer doc={selected} onClose={() => setSelected(null)} />
    </div>
  );
}

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api, type Doc } from "../api";

export default function Library() {
  const [root, setRoot] = useState("");
  const [sourceType, setSourceType] = useState("");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<number | null>(null);
  const limit = 40;

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
    queryKey: ["document", selected],
    queryFn: () => api.document(selected!),
    enabled: selected != null,
  });

  const list: Doc[] = docs.data?.documents || [];
  const total = docs.data?.total ?? 0;

  return (
    <div>
      <h1>Library</h1>
      <div className="row panel">
        <label>
          Root
          <input value={root} onChange={(e) => { setRoot(e.target.value); setOffset(0); }} placeholder="rootName" />
        </label>
        <label>
          Source type
          <select value={sourceType} onChange={(e) => { setSourceType(e.target.value); setOffset(0); }}>
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
              <tr key={d.id} onClick={() => setSelected(d.id)} style={{ cursor: "pointer" }}>
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
      {selected != null && (
        <div className="panel">
          <h2>Document {selected}</h2>
          <pre>{JSON.stringify(detail.data, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}

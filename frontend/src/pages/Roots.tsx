import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, normalizeList, type RootGroup } from "../api";
import PageHead from "../PageHead";

export default function Roots() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [members, setMembers] = useState("");
  const roots = useQuery({
    queryKey: ["roots"],
    queryFn: async () => normalizeList<string>(await api.roots(), "roots"),
  });
  const groups = useQuery({
    queryKey: ["root-groups"],
    queryFn: async () => normalizeList<RootGroup>(await api.rootGroups(), "groups"),
  });

  const save = useMutation({
    mutationFn: async () => {
      const list = members
        .split(/[\n,]+/)
        .map((s) => s.trim())
        .filter(Boolean)
        .map((rootName, i) => ({ rootName, priority: 100 - i }));
      return api.upsertRootGroup(name.trim(), { members: list });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["root-groups"] });
      setName("");
      setMembers("");
    },
  });

  return (
    <div>
      <PageHead title="Roots" blurb="Knowledge roots and prioritized root groups." />
      <div className="panel">
        <h2 className="section-title">Roots</h2>
        <ul>
          {(roots.data || []).map((r) => (
            <li key={r} className="mono">
              {r}
            </li>
          ))}
        </ul>
      </div>
      <div className="panel">
        <h2 className="section-title">Root groups</h2>
        {(groups.data || []).map((g) => (
          <div key={g.name} style={{ marginBottom: "1rem" }}>
            <strong>{g.name}</strong>
            <div className="muted">{g.description}</div>
            <ul>
              {(g.members || []).map((m) => (
                <li key={m.rootName} className="mono">
                  {m.rootName} (priority {m.priority})
                </li>
              ))}
            </ul>
            <button
              type="button"
              className="danger"
              onClick={() => api.deleteRootGroup(g.name).then(() => qc.invalidateQueries({ queryKey: ["root-groups"] }))}
            >
              Delete group
            </button>
          </div>
        ))}
      </div>
      <div className="panel stack">
        <h2 className="section-title">Create / update group</h2>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label>
          Members (comma or newline, order = priority)
          <textarea rows={4} value={members} onChange={(e) => setMembers(e.target.value)} />
        </label>
        <button className="primary" type="button" disabled={!name.trim()} onClick={() => save.mutate()}>
          Save group
        </button>
        {save.isError && <div className="error-box">{(save.error as Error).message}</div>}
      </div>
    </div>
  );
}

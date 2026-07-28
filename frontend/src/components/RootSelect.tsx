import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { api, normalizeList } from "../api";

const SESSION_KEY = "librarian.lastRoot";

export default function RootSelect({
  value,
  onChange,
  allowAll = true,
  id = "root-select",
  label = "Root",
  allRootsHint,
}: {
  value: string;
  onChange: (root: string) => void;
  allowAll?: boolean;
  id?: string;
  label?: string;
  allRootsHint?: string;
}) {
  const roots = useQuery({
    queryKey: ["roots"],
    queryFn: async () => normalizeList<string>(await api.roots(), "roots"),
  });
  const restored = useRef(false);

  const options = [...normalizeList<string>(roots.data as never, "roots")].sort((a, b) =>
    a.localeCompare(b),
  );

  useEffect(() => {
    if (restored.current || value || options.length === 0) return;
    try {
      const saved = sessionStorage.getItem(SESSION_KEY) || "";
      if (saved && options.includes(saved)) {
        restored.current = true;
        onChange(saved);
      }
    } catch {
      /* ignore */
    }
  }, [options, value, onChange]);

  function select(next: string) {
    onChange(next);
    try {
      if (next) sessionStorage.setItem(SESSION_KEY, next);
      else sessionStorage.removeItem(SESSION_KEY);
    } catch {
      /* ignore */
    }
  }

  return (
    <div className="root-select">
      <label htmlFor={id}>
        {label}
        <select
          id={id}
          value={value}
          onChange={(e) => select(e.target.value)}
          disabled={roots.isLoading}
        >
          {allowAll ? <option value="">All roots</option> : <option value="">Select a root</option>}
          {options.map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>
      </label>
      {allowAll && !value && allRootsHint && !roots.isError && options.length > 0 && (
        <p className="muted root-select-hint">{allRootsHint}</p>
      )}
      {roots.isError && (
        <p className="muted root-select-hint">Could not load roots — {(roots.error as Error).message}</p>
      )}
      {!roots.isLoading && !roots.isError && options.length === 0 && (
        <p className="muted root-select-hint">
          No roots available. <Link to="/sources">Add a source</Link> to create one.
        </p>
      )}
    </div>
  );
}

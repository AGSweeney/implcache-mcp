import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { useEffect, useMemo, useState } from "react";
import { api, normalizeList, type SourceSummary } from "../api";
import Button from "../components/Button";
import ConfirmDialog from "../components/ConfirmDialog";
import DetailsDrawer from "../components/DetailsDrawer";
import EmptyState from "../components/EmptyState";
import ErrorState from "../components/ErrorState";
import LoadingSkeleton from "../components/LoadingSkeleton";
import MetricStrip from "../components/MetricStrip";
import OverflowMenu from "../components/OverflowMenu";
import PageHeader from "../components/PageHeader";
import StatusBadge from "../components/StatusBadge";
import Toolbar from "../components/Toolbar";
import TypeBadge from "../components/TypeBadge";
import { useToast } from "../components/Toast";
import {
  formatLastIndexed,
  mapSourceStatus,
  sourceDisplayName,
  sourceSecondaryLine,
  sourceVersion,
  sourceWarningInfo,
} from "../sourceUi";

const FILTERS_KEY = "implcache.librarian.sources.filters";

type Filters = {
  search: string;
  type: string;
  status: string;
  root: string;
  warningsOnly: boolean;
};

const defaultFilters: Filters = {
  search: "",
  type: "",
  status: "",
  root: "",
  warningsOnly: false,
};

function loadFilters(): Filters {
  try {
    const raw = localStorage.getItem(FILTERS_KEY);
    if (!raw) return defaultFilters;
    return { ...defaultFilters, ...JSON.parse(raw) };
  } catch {
    return defaultFilters;
  }
}

function matchesStatus(s: SourceSummary, status: string): boolean {
  if (!status) return true;
  return mapSourceStatus(s).ui === status;
}

function matchesSearch(s: SourceSummary, q: string): boolean {
  if (!q) return true;
  const hay = [
    sourceDisplayName(s),
    sourceSecondaryLine(s),
    s.id,
    s.rootName,
    s.kind,
    s.title || "",
    s.lastStatus || "",
  ]
    .join(" ")
    .toLowerCase();
  return hay.includes(q.toLowerCase());
}

export default function Sources() {
  const qc = useQueryClient();
  const nav = useNavigate();
  const toast = useToast();
  const [filters, setFilters] = useState<Filters>(() => loadFilters());
  const [searchInput, setSearchInput] = useState(filters.search);
  const [selected, setSelected] = useState<SourceSummary | null>(null);
  const [pendingRemove, setPendingRemove] = useState<SourceSummary | null>(null);

  useEffect(() => {
    const t = window.setTimeout(() => {
      setFilters((f) => ({ ...f, search: searchInput }));
    }, 250);
    return () => window.clearTimeout(t);
  }, [searchInput]);

  useEffect(() => {
    localStorage.setItem(FILTERS_KEY, JSON.stringify(filters));
  }, [filters]);

  const q = useQuery({
    queryKey: ["sources"],
    queryFn: async () => normalizeList<SourceSummary>(await api.sources(), "sources"),
    refetchInterval: 10000,
  });

  const all = q.data || [];

  const roots = useMemo(() => {
    const set = new Set(all.map((s) => s.rootName).filter(Boolean));
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [all]);

  const rows = useMemo(() => {
    return all.filter((s) => {
      if (filters.type && s.kind !== filters.type) return false;
      if (filters.root && s.rootName !== filters.root) return false;
      if (!matchesStatus(s, filters.status)) return false;
      if (filters.warningsOnly && sourceWarningInfo(s).count === 0) return false;
      if (!matchesSearch(s, filters.search)) return false;
      return true;
    });
  }, [all, filters]);

  const metrics = useMemo(() => {
    const totalDocs = all.reduce((n, s) => n + (s.documentCount || 0), 0);
    const repo = all.filter((s) => s.kind === "repo").length;
    const local = all.filter((s) => s.kind === "local").length;
    const failed = all.filter((s) => mapSourceStatus(s).ui === "failed").length;
    return [
      { label: "sources", value: all.length },
      { label: "documents", value: totalDocs },
      { label: "repos", value: repo },
      { label: "local", value: local },
      { label: "failed", value: failed },
    ];
  }, [all]);

  const refresh = useMutation({
    mutationFn: async (s: SourceSummary) => {
      if (s.kind === "web") return api.refreshWeb(s.id);
      if (s.kind === "repo") return api.refreshGit(s.id);
      throw new Error("Refresh is not supported for " + s.kind);
    },
    onSuccess: (res, s) => {
      qc.invalidateQueries({ queryKey: ["jobs"] });
      qc.invalidateQueries({ queryKey: ["sources"] });
      const opId = (res as { opId?: string } | undefined)?.opId;
      toast.push({
        message: `Refresh started for ${sourceDisplayName(s)}`,
        variant: "success",
        actionLabel: opId ? "View job" : undefined,
        onAction: opId ? () => nav(`/jobs?op=${encodeURIComponent(opId)}`) : undefined,
      });
    },
    onError: (err: Error) => toast.push({ message: err.message, variant: "danger" }),
  });

  const remove = useMutation({
    mutationFn: async (s: SourceSummary) => {
      if (s.kind === "web") return api.deleteWeb(s.id);
      if (s.kind === "repo") return api.deleteGit(s.id);
      if (s.kind === "pdf") return api.deletePdf(s.id);
      if (s.kind === "local") return api.deleteLocal(s.id);
      throw new Error("Remove is not supported for " + s.kind);
    },
    onSuccess: (_res, s) => {
      qc.invalidateQueries({ queryKey: ["sources"] });
      toast.push({ message: `Removed ${sourceDisplayName(s)}`, variant: "success" });
      setPendingRemove(null);
      if (selected && selected.kind === s.kind && selected.id === s.id) setSelected(null);
    },
    onError: (err: Error) => toast.push({ message: err.message, variant: "danger" }),
  });

  return (
    <div>
      <PageHeader
        title="Sources"
        subtitle="Unified inventory of local, Git, web, and PDF corpora."
        actions={
          <Link to="/sources/add">
            <Button variant="primary">Add Source</Button>
          </Link>
        }
      />

      <MetricStrip items={metrics} />

      <Toolbar>
        <label className="grow">
          Search
          <input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Name, URL, path, root…"
          />
        </label>
        <label>
          Type
          <select value={filters.type} onChange={(e) => setFilters((f) => ({ ...f, type: e.target.value }))}>
            <option value="">All</option>
            <option value="web">Web</option>
            <option value="repo">Repo</option>
            <option value="pdf">PDF</option>
            <option value="local">Local</option>
          </select>
        </label>
        <label>
          Status
          <select value={filters.status} onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value }))}>
            <option value="">All</option>
            <option value="ready">Ready</option>
            <option value="never">Never indexed</option>
            <option value="refreshing">Refreshing</option>
            <option value="attention">Needs attention</option>
            <option value="failed">Failed</option>
            <option value="disabled">Disabled</option>
          </select>
        </label>
        <label>
          Root
          <select value={filters.root} onChange={(e) => setFilters((f) => ({ ...f, root: e.target.value }))}>
            <option value="">All</option>
            {roots.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={filters.warningsOnly}
            onChange={(e) => setFilters((f) => ({ ...f, warningsOnly: e.target.checked }))}
          />
          Warnings only
        </label>
      </Toolbar>

      {q.isLoading && (
        <div className="data-table-wrap">
          <LoadingSkeleton />
        </div>
      )}

      {q.isError && (
        <ErrorState message={(q.error as Error).message} onRetry={() => q.refetch()} />
      )}

      {!q.isLoading && !q.isError && all.length === 0 && (
        <EmptyState
          title="No sources yet"
          body="Add a local folder, Git repo, web crawl, or PDF to start indexing."
          action={
            <Link to="/sources/add">
              <Button variant="primary">Add Source</Button>
            </Link>
          }
        />
      )}

      {!q.isLoading && !q.isError && all.length > 0 && rows.length === 0 && (
        <EmptyState title="No matching sources" body="Try clearing filters or adjusting search." />
      )}

      {!q.isLoading && !q.isError && rows.length > 0 && (
        <div className="data-table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th className="col-hide-sm">Root</th>
                <th className="col-hide-md">Version / Commit</th>
                <th>Status</th>
                <th className="num">Documents</th>
                <th className="col-hide-md">Last Indexed</th>
                <th className="col-hide-sm">Warnings</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((s) => {
                const st = mapSourceStatus(s);
                const warn = sourceWarningInfo(s);
                const lastIndexed = formatLastIndexed(s);
                const selectedRow = selected?.kind === s.kind && selected?.id === s.id;
                return (
                  <tr key={`${s.kind}:${s.id}`} className={selectedRow ? "selected" : undefined}>
                    <td>
                      <div className="name-cell">
                        <button type="button" className="name-link" onClick={() => setSelected(s)}>
                          {sourceDisplayName(s)}
                        </button>
                        <span className="sub">{sourceSecondaryLine(s)}</span>
                      </div>
                    </td>
                    <td>
                      <TypeBadge kind={s.kind} />
                    </td>
                    <td className="mono col-hide-sm">{s.rootName}</td>
                    <td
                      className={`col-hide-md ${s.kind === "local" ? "muted" : "mono"}`}
                      title={s.kind === "local" ? "Local folder ingest — no version/commit tracking" : undefined}
                    >
                      {sourceVersion(s)}
                    </td>
                    <td>
                      <StatusBadge variant={st.ui} title={st.raw}>
                        {st.label}
                      </StatusBadge>
                    </td>
                    <td className="num">
                      <Link to={`/library?root=${encodeURIComponent(s.rootName)}`}>{s.documentCount}</Link>
                    </td>
                    <td className={`col-hide-md ${lastIndexed.title ? "muted" : ""}`} title={lastIndexed.title}>
                      {lastIndexed.text}
                    </td>
                    <td className="col-hide-sm">
                      {warn.count > 0 ? (
                        <StatusBadge variant="warning" title={warn.label}>
                          {warn.count}
                        </StatusBadge>
                      ) : (
                        <span className="muted warn-none">None</span>
                      )}
                    </td>
                    <td>
                      <div className="actions-cell">
                        {(s.kind === "web" || s.kind === "repo") && (
                          <Button
                            variant="secondary"
                            disabled={refresh.isPending}
                            onClick={() => refresh.mutate(s)}
                          >
                            Refresh
                          </Button>
                        )}
                        <OverflowMenu
                          items={[
                            {
                              label: "View documents",
                              onClick: () => nav(`/library?root=${encodeURIComponent(s.rootName)}`),
                            },
                            {
                              label: "View jobs",
                              onClick: () => nav("/jobs"),
                            },
                            {
                              label: "Remove…",
                              danger: true,
                              onClick: () => setPendingRemove(s),
                            },
                          ]}
                        />
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <DetailsDrawer
        open={!!selected}
        title={selected ? sourceDisplayName(selected) : ""}
        onClose={() => setSelected(null)}
        footer={
          selected && (
            <>
              <Button variant="secondary" onClick={() => nav(`/library?root=${encodeURIComponent(selected.rootName)}`)}>
                View documents
              </Button>
              {(selected.kind === "web" || selected.kind === "repo") && (
                <Button variant="primary" onClick={() => refresh.mutate(selected)} disabled={refresh.isPending}>
                  Refresh
                </Button>
              )}
              <Button variant="danger" onClick={() => setPendingRemove(selected)}>
                Remove…
              </Button>
            </>
          )
        }
      >
        {selected && (
          <dl className="drawer-meta">
            <div>
              <dt>Type</dt>
              <dd>
                <TypeBadge kind={selected.kind} />
              </dd>
            </div>
            <div>
              <dt>ID</dt>
              <dd className="mono">{selected.id}</dd>
            </div>
            <div>
              <dt>Root</dt>
              <dd className="mono">{selected.rootName}</dd>
            </div>
            <div>
              <dt>Status</dt>
              <dd>
                <StatusBadge variant={mapSourceStatus(selected).ui} title={mapSourceStatus(selected).raw}>
                  {mapSourceStatus(selected).label}
                </StatusBadge>
              </dd>
            </div>
            <div>
              <dt>Version / Commit</dt>
              <dd className={selected.kind === "local" ? "muted" : "mono"}>{sourceVersion(selected)}</dd>
            </div>
            <div>
              <dt>Documents</dt>
              <dd>{selected.documentCount}</dd>
            </div>
            <div>
              <dt>Chunks</dt>
              <dd>{selected.chunkCount}</dd>
            </div>
            <div>
              <dt>Last indexed</dt>
              <dd title={formatLastIndexed(selected).title}>{formatLastIndexed(selected).text}</dd>
            </div>
            {selected.kind === "local" && (
              <div>
                <dt>Note</dt>
                <dd className="muted">
                  Local folder root — content is indexed in the library. Refresh timestamps and version/commit apply to
                  web and git sources.
                </dd>
              </div>
            )}
            <div>
              <dt>Location</dt>
              <dd className="mono">{sourceSecondaryLine(selected)}</dd>
            </div>
          </dl>
        )}
      </DetailsDrawer>

      <ConfirmDialog
        open={!!pendingRemove}
        title="Remove source?"
        body={
          pendingRemove && (
            <>
              <p>
                Remove <strong>{sourceDisplayName(pendingRemove)}</strong> ({pendingRemove.kind}/
                {pendingRemove.id})?
              </p>
              <ul>
                <li>Indexed documents for this source will be deleted.</li>
                <li>This cannot be undone from the UI.</li>
              </ul>
            </>
          )
        }
        confirmLabel="Remove"
        busy={remove.isPending}
        onCancel={() => setPendingRemove(null)}
        onConfirm={() => pendingRemove && remove.mutate(pendingRemove)}
      />
    </div>
  );
}

import type { SourceSummary } from "./api";

export type UiStatus = "ready" | "never" | "refreshing" | "attention" | "failed" | "disabled" | "unknown";

export function sourceDisplayName(s: SourceSummary): string {
  const detail = s.detail || {};
  if (s.kind === "repo") {
    const remote = String(detail.remoteUrl || "");
    if (s.title && !/^https?:\/\//i.test(s.title) && s.title !== remote) return s.title;
    if (s.id) return s.id;
  }
  if (s.kind === "web") {
    if (s.id && s.id !== s.title) return s.id;
  }
  if (s.title && !/^https?:\/\//i.test(s.title) && !s.title.includes("://")) return s.title;
  if (s.id) return s.id;
  return s.rootName || "source";
}

export function sourceSecondaryLine(s: SourceSummary): string {
  const detail = s.detail || {};
  if (s.kind === "repo") {
    const remote = String(detail.remoteUrl || s.title || "");
    return shortenUrl(remote) || s.id;
  }
  if (s.kind === "web") {
    return String(detail.startUrl || s.title || s.id);
  }
  if (s.kind === "pdf") {
    return String(detail.sourcePath || s.id);
  }
  return s.id !== s.rootName ? s.id : s.rootName;
}

export function shortenUrl(url: string): string {
  try {
    const u = new URL(url);
    return `${u.host}${u.pathname}`.replace(/\/$/, "") || url;
  } catch {
    return url;
  }
}

export function sourceVersion(s: SourceSummary): string {
  const d = s.detail || {};
  if (s.kind === "repo") {
    const sha = String(d.resolvedCommit || "");
    if (sha) return sha.slice(0, 8);
    return String(d.requestedRef || "—");
  }
  if (s.kind === "web") {
    return String(d.detectedVersion || d.declaredVersion || "—");
  }
  if (s.kind === "pdf") {
    return String(d.version || "—");
  }
  if (s.kind === "local") {
    return "Local root";
  }
  return "—";
}

/** Last-indexed cell: timestamps for web/git/pdf; explanatory label for local roots. */
export function formatLastIndexed(s: SourceSummary): { text: string; title?: string } {
  const ts = s.lastSuccessAt || s.lastAttemptAt;
  if (ts) return { text: formatEpoch(ts) };
  if (s.kind === "local") {
    return {
      text: "On ingest",
      title:
        "Local folder roots are indexed into the library but do not track refresh timestamps (unlike web/git sources).",
    };
  }
  return { text: "—" };
}

export function mapSourceStatus(s: SourceSummary): { ui: UiStatus; label: string; raw: string } {
  const raw = (s.lastStatus || "").trim();
  const lower = raw.toLowerCase();
  if (s.enabled === false) return { ui: "disabled", label: "Disabled", raw: raw || "disabled" };
  // Local roots are synthesized from inventory and often have no lastStatus even when
  // documents exist. Treat non-empty corpora as ready rather than "never indexed".
  if (!raw || lower === "idle") {
    if ((s.documentCount || 0) > 0 || (s.chunkCount || 0) > 0 || (s.lastSuccessAt || 0) > 0) {
      return { ui: "ready", label: "Ready", raw: raw || "indexed" };
    }
    return { ui: "never", label: "Never indexed", raw: raw || "idle" };
  }
  if (lower === "running" || lower.includes("refresh") || lower === "queued") {
    return { ui: "refreshing", label: "Refreshing", raw };
  }
  if (lower === "ok" || lower === "ingested" || lower === "text" || lower === "unchanged") {
    return { ui: "ready", label: "Ready", raw };
  }
  if (lower.startsWith("partial")) return { ui: "attention", label: "Needs attention", raw };
  if (lower.startsWith("failed") || lower === "corrupt" || lower === "encrypted" || lower === "image-only") {
    return { ui: "failed", label: "Failed", raw };
  }
  return { ui: "unknown", label: raw || "Unknown", raw: raw || "unknown" };
}

export function formatEpoch(ts?: number): string {
  if (!ts) return "—";
  try {
    return new Date(ts * 1000).toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return "—";
  }
}

export function sourceWarning(s: SourceSummary): string {
  const st = mapSourceStatus(s);
  if (st.ui === "failed" || st.ui === "attention") return st.label;
  return "";
}

export function typeLabel(kind: string): string {
  switch (kind) {
    case "local":
      return "LOCAL";
    case "repo":
      return "REPO";
    case "web":
      return "WEB";
    case "pdf":
      return "PDF";
    default:
      return kind.toUpperCase();
  }
}

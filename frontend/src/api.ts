export type ServerInfo = {
  serverVersion: string;
  apiVersion: number;
  schemaVersion: number;
  readOnly: boolean;
  semanticEnabled: boolean;
  ocrSupported: boolean;
  supportedSourceTypes: string[];
  authMode: string;
  librarianEnabled: boolean;
  role?: string;
  analyticsEnabled?: boolean;
  analyticsAvailable?: boolean;
};

export type AnalyticsStatus = {
  enabled: boolean;
  available: boolean;
  dbPath?: string;
  retentionDays: number;
  storeTaskText: boolean;
  storeEvidenceText: boolean;
  databaseBytes?: number;
  oldestAt?: string;
  newestAt?: string;
  requestCount: number;
  droppedEvents: number;
  message?: string;
  localOnly: boolean;
  metadataOnly: boolean;
  tokenEstimatorVersion?: string;
  schemaVersion?: number;
};

export type AnalyticsSummary = {
  totalRequests: number;
  groundedRequests: number;
  localEvidenceRate: number;
  curatedRequests: number;
  curatedUsageRate: number;
  highCoverage: number;
  mediumCoverage: number;
  lowCoverage: number;
  unclassifiedCoverage: number;
  notApplicableCoverage: number;
  highCoverageRate: number;
  mediumCoverageRate: number;
  lowCoverageRate: number;
  unclassifiedCoverageRate: number;
  rootSelectionRequired: number;
  rootSelectionRate: number;
  localContextTokensServed: number;
  avgPackageTokens?: number | null;
  noLocalMatch: number;
  localInsufficient: number;
  errors: number;
  reconcileOk: boolean;
  reconcileSum: number;
  unclassifiedCoverageWarning: boolean;
  tokenEstimatorVersion?: string;
};

export type AnalyticsTimePoint = {
  bucket: string;
  total: number;
  grounded: number;
  rootChoice?: number;
  noMatch?: number;
  insufficient: number;
  errors: number;
  high?: number;
  medium?: number;
  low?: number;
  tokensServed?: number;
  sourceTokens?: number;
  tokensAvoided?: number;
  avgPackage?: number | null;
  avgReduction?: number | null;
};

export type AnalyticsCoverage = {
  high: number;
  medium: number;
  low: number;
  unclassified: number;
  notApplicable?: number;
  localInsufficient: number;
  noLocalMatch: number;
  rootSelectionRequired: number;
  errors: number;
  grounded: number;
  total: number;
  unclassifiedWarning?: boolean;
};

export type AnalyticsOutcomes = {
  curated: number;
  local?: number;
  recipe?: number;
  symbolLed?: number;
  rawDocLed?: number;
  mixed: number;
  noMatch: number;
  insufficient: number;
  rootSelectionRequired: number;
  errors: number;
  other: number;
  total: number;
};

export type AnalyticsEvidence = {
  curated: number;
  recipe: number;
  symbol: number;
  rawDocuments: number;
  document: number;
};

export type AnalyticsGrounding = {
  outcomes: AnalyticsOutcomes;
  evidence: AnalyticsEvidence;
};

export type AnalyticsEfficiency = {
  localContextTokensServed?: number | null;
  structuredTokensServed?: number | null;
  rawDocumentTokensServed?: number | null;
  avgPackageTokens?: number | null;
  estimatedSourceTokens?: number | null;
  estimatedTokensAvoided?: number | null;
  avgContextReductionPercent?: number | null;
  rawDocumentShare?: number | null;
  tokensPerGroundedRequest?: number | null;
  tokensPerSuccessfulOutcome?: number | null;
  tokenEstimatorVersion?: string;
  sourceTypeBreakdown?: { type: string; tokens: number; label: string }[];
  tokenTimeseries?: AnalyticsTimePoint[];
  packageTimeseries?: AnalyticsTimePoint[];
  groundedRequests?: number;
  successfulOutcomes?: number;
};

export type AnalyticsKnowledge = {
  roots: { key: string; label?: string; timesSelected: number; timesIncluded: number }[];
  evidence: { key: string; label?: string; timesSelected: number; timesIncluded: number }[];
  curated: { key: string; label?: string; timesSelected: number; timesIncluded: number }[];
};

export type AnalyticsRequestRow = {
  requestId: string;
  occurredAt: string;
  toolName: string;
  resultStatus: string;
  coverage?: string;
  estimatedTokens: number;
  returnedTokens?: number;
  sourceCount?: number;
  latencyMs: number;
  requestClass?: string;
  roots?: string[];
  curated?: boolean;
};

export type AnalyticsRequestList = {
  requests: AnalyticsRequestRow[];
  count: number;
  total: number;
  limit: number;
  offset: number;
};

export type AnalyticsRequestDetail = AnalyticsRequestRow & {
  clientName?: string;
  modelName?: string;
  sessionHash?: string;
  freshness?: string;
  contextFingerprint?: string;
  taskHash?: string;
  rootSelectionRequired: boolean;
  additionalRetrievalRecommended: boolean;
  citationCount: number;
  curatedCount: number;
  recipeCount: number;
  symbolCount: number;
  structuredTokens?: number;
  rawDocumentTokens?: number;
  estimatedSourceTokens?: number;
  estimatedTokensAvoided?: number;
  contextReductionPercent?: number | null;
  tokenEstimatorVersion?: string;
  coverageApplicable?: boolean | null;
  errorCategory?: string;
  errorMessage?: string;
  evidence?: {
    evidenceType: string;
    evidenceKey?: string;
    rootKey?: string;
    sourceUri?: string;
    authority?: string;
    rankPosition: number;
    selectedForPackage: boolean;
    includedAfterTrimming: boolean;
    estimatedTokens: number;
  }[];
};

export type SourceSummary = {
  kind: string;
  id: string;
  rootName: string;
  title?: string;
  enabled?: boolean;
  lastStatus?: string;
  lastAttemptAt?: number;
  lastSuccessAt?: number;
  documentCount: number;
  chunkCount: number;
  symbolCount?: number;
  detail?: Record<string, unknown>;
};

export type LibraryStats = {
  documents: number;
  chunks: number;
  symbols: number;
  recipes: number;
  databaseBytes: number;
  sourcesTotal: number;
  sourcesOk: number;
  sourcesFailed: number;
  activeJobs: number;
};

export type Operation = {
  opId: string;
  source: { kind: string; id: string; rootName?: string; title?: string };
  state: string;
  progress: {
    phase?: string;
    done?: number;
    total?: number;
    bytes?: number;
    current?: string;
    message?: string;
  };
  startedAt: number;
  finishedAt?: number;
  errors?: string[];
};

export type HealthIssue = {
  severity: string;
  code: string;
  sourceKind?: string;
  sourceId?: string;
  description: string;
  recommendedAction?: string;
};

export type APIError = {
  code: string;
  message: string;
  detail?: string;
};

const TOKEN_KEY = "implcache.librarian.token";

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || "";
}

export function setToken(token: string) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (!headers.has("Content-Type") && init?.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) {
    let err: APIError = { code: "http", message: res.statusText };
    try {
      err = await res.json();
    } catch {
      /* ignore */
    }
    throw Object.assign(new Error(err.message || res.statusText), { api: err, status: res.status });
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  server: () => request<ServerInfo>("/api/v1/server"),
  sources: () => request<{ sources: SourceSummary[]; count: number } | SourceSummary[]>("/api/v1/sources"),
  source: (kind: string, id: string) =>
    request<SourceSummary>(`/api/v1/sources/${kind}/${encodeURIComponent(id)}`),
  sourceHealth: (kind: string, id: string) =>
    request(`/api/v1/sources/${kind}/${encodeURIComponent(id)}/health`),
  sourceErrors: (kind: string, id: string) =>
    request<{ errors: string[] }>(`/api/v1/sources/${kind}/${encodeURIComponent(id)}/errors`),
  stats: () => request<LibraryStats>("/api/v1/library/stats"),
  jobs: () => request<{ operations: Operation[]; count: number } | Operation[]>("/api/v1/jobs"),
  job: (id: string) => request<Operation>(`/api/v1/jobs/${id}`),
  cancelJob: (id: string) => request(`/api/v1/jobs/${id}/cancel`, { method: "POST" }),
  health: () => request<{ issues: HealthIssue[] } | HealthIssue[]>("/api/v1/health"),
  purgeEmptyDocs: () =>
    request<{
      deleted: number;
      before: number;
      byRoot?: { rootName: string; count: number; sourceType?: string }[];
      sampleUris?: string[];
    }>("/api/v1/library/purge-empty-docs", { method: "POST", body: "{}" }),
  documents: (q: URLSearchParams) =>
    request<{ documents: Doc[]; total: number; count: number }>(`/api/v1/library/documents?${q}`),
  document: (id: number) => request(`/api/v1/library/documents/${id}`),
  documentSymbols: (id: number, limit = 200) =>
    request(`/api/v1/library/documents/${id}/symbols?limit=${limit}`),
  roots: () => request<{ roots: string[]; count: number } | string[]>("/api/v1/roots"),
  rootGroups: () => request<{ groups: RootGroup[] } | RootGroup[]>("/api/v1/root-groups"),
  upsertRootGroup: (name: string, body: { description?: string; members: { rootName: string; priority: number }[] }) =>
    request(`/api/v1/root-groups/${encodeURIComponent(name)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteRootGroup: (name: string) =>
    request(`/api/v1/root-groups/${encodeURIComponent(name)}`, { method: "DELETE" }),
  search: (body: Record<string, unknown>) =>
    request("/api/v1/search", { method: "POST", body: JSON.stringify(body) }),
  searchSymbols: (body: Record<string, unknown>) =>
    request("/api/v1/search/symbols", { method: "POST", body: JSON.stringify(body) }),
  upsertWeb: (body: Record<string, unknown>) =>
    request("/api/v1/sources/web", { method: "POST", body: JSON.stringify(body) }),
  ingestWeb: (name: string, body?: Record<string, unknown>) =>
    request<{ opId: string }>(`/api/v1/sources/web/${encodeURIComponent(name)}/ingest`, {
      method: "POST",
      body: JSON.stringify(body || {}),
    }),
  refreshWeb: (name: string) =>
    request<{ opId: string }>(`/api/v1/sources/web/${encodeURIComponent(name)}/refresh`, { method: "POST", body: "{}" }),
  deleteWeb: (name: string) =>
    request(`/api/v1/sources/web/${encodeURIComponent(name)}`, { method: "DELETE" }),
  ingestGit: (body: Record<string, unknown>) =>
    request<{ opId: string }>("/api/v1/sources/git", { method: "POST", body: JSON.stringify(body) }),
  refreshGit: (name: string) =>
    request<{ opId: string }>(`/api/v1/sources/git/${encodeURIComponent(name)}/refresh`, { method: "POST", body: "{}" }),
  deleteGit: (name: string) =>
    request(`/api/v1/sources/git/${encodeURIComponent(name)}`, { method: "DELETE" }),
  inspectPdf: (body: Record<string, unknown>) =>
    request("/api/v1/sources/pdf/inspect", { method: "POST", body: JSON.stringify(body) }),
  ingestPdf: (body: Record<string, unknown>) =>
    request("/api/v1/sources/pdf/ingest", { method: "POST", body: JSON.stringify(body) }),
  deletePdf: (uri: string) =>
    request(`/api/v1/sources/pdf?uri=${encodeURIComponent(uri)}`, { method: "DELETE" }),
  deleteLocal: (rootName: string) =>
    request(`/api/v1/sources/local/${encodeURIComponent(rootName)}`, { method: "DELETE" }),
  previewLocal: (body: Record<string, unknown>) =>
    request("/api/v1/sources/local/preview", { method: "POST", body: JSON.stringify(body) }),
  inspectGit: (body: Record<string, unknown>) =>
    request("/api/v1/sources/git/inspect", { method: "POST", body: JSON.stringify(body) }),
  previewWeb: (body: Record<string, unknown>) =>
    request("/api/v1/sources/web/preview", { method: "POST", body: JSON.stringify(body) }),
  searchContext: (body: Record<string, unknown>) =>
    request("/api/v1/search/context", { method: "POST", body: JSON.stringify(body) }),
  logs: (limit = 100) => request<{ lines: { at: number; level: string; message: string }[] }>(`/api/v1/logs?limit=${limit}`),
  ingestLocal: (body: Record<string, unknown>) =>
    request("/api/v1/sources/local/ingest", { method: "POST", body: JSON.stringify(body) }),
  analyticsStatus: () => request<AnalyticsStatus>("/api/v1/analytics/status"),
  analyticsSummary: (q: URLSearchParams) =>
    request<AnalyticsSummary>(`/api/v1/analytics/summary?${q}`),
  analyticsTimeseries: (q: URLSearchParams) =>
    request<{ points: AnalyticsTimePoint[]; count: number }>(`/api/v1/analytics/timeseries?${q}`),
  analyticsCoverage: (q: URLSearchParams) =>
    request<AnalyticsCoverage>(`/api/v1/analytics/coverage?${q}`),
  analyticsGrounding: (q: URLSearchParams) =>
    request<AnalyticsGrounding>(`/api/v1/analytics/grounding?${q}`),
  analyticsOutcomes: (q: URLSearchParams) =>
    request<AnalyticsOutcomes>(`/api/v1/analytics/outcomes?${q}`),
  analyticsEvidence: (q: URLSearchParams) =>
    request<AnalyticsEvidence>(`/api/v1/analytics/evidence?${q}`),
  analyticsEfficiency: (q: URLSearchParams) =>
    request<AnalyticsEfficiency>(`/api/v1/analytics/efficiency?${q}`),
  analyticsKnowledge: (q: URLSearchParams) =>
    request<AnalyticsKnowledge>(`/api/v1/analytics/knowledge?${q}`),
  analyticsRequests: (q: URLSearchParams) =>
    request<AnalyticsRequestList>(`/api/v1/analytics/requests?${q}`),
  analyticsRequest: (id: string) =>
    request<AnalyticsRequestDetail>(`/api/v1/analytics/requests/${encodeURIComponent(id)}`),
  analyticsExportUrl: (q: URLSearchParams, format: "json" | "csv" = "json") => {
    const n = new URLSearchParams(q);
    n.set("format", format);
    return `/api/v1/analytics/export?${n}`;
  },
  putAnalyticsSettings: (body: {
    enabled?: boolean;
    retentionDays?: number;
    storeTaskText?: boolean;
    storeEvidenceText?: boolean;
  }) =>
    request<AnalyticsStatus>("/api/v1/settings/analytics", {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  clearAnalyticsData: (confirm = true, vacuum = false) =>
    request<AnalyticsStatus>("/api/v1/analytics/data", {
      method: "DELETE",
      body: JSON.stringify({ confirm, vacuum }),
    }),
  upload: async (file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    return request<{ path: string; fileName: string; size: number }>("/api/v1/uploads", {
      method: "POST",
      body: fd,
    });
  },
};

export type Doc = {
  id: number;
  uri: string;
  title: string;
  sourceType: string;
  path: string;
  rootName?: string;
  authority?: string;
  productVersion?: string;
  updatedAt?: number;
};

export type RootGroup = {
  name: string;
  description?: string;
  members?: { rootName: string; priority: number }[];
};

export function normalizeList<T>(data: T[] | { [k: string]: T[] | number | null | undefined } | null | undefined, key: string): T[] {
  if (Array.isArray(data)) return data;
  if (!data || typeof data !== "object") return [];
  const v = (data as Record<string, unknown>)[key];
  return Array.isArray(v) ? (v as T[]) : [];
}

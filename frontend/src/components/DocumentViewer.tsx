import { useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, type Doc } from "../api";
import Button from "./Button";
import CodeBlock, { copyText, detectLanguage, getSelectedLineRange } from "./CodeBlock";
import Modal from "./Modal";

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

type DocSymbol = {
  id: number;
  name: string;
  kind?: string;
  language?: string;
  signature?: string;
  startLine?: number;
  endLine?: number;
  qualifiedName?: string;
};

type Tab = "source" | "chunks" | "symbols" | "raw";

/** Concatenate chunk bodies in order so viewer line numbers match indexer startLine. */
function formatSource(detail: DocDetail | undefined): string {
  if (!detail?.chunks?.length) return "";
  return [...detail.chunks]
    .sort((a, b) => (a.ordinal ?? 0) - (b.ordinal ?? 0))
    .map((c) => c.body || "")
    .join("\n");
}

function citationFor(doc: Doc, range?: { start: number; end: number } | null): string {
  const ver = doc.productVersion ? ` @ ${doc.productVersion}` : "";
  const lines =
    range && range.start > 0
      ? range.start === range.end
        ? `\nlines ${range.start}`
        : `\nlines ${range.start}–${range.end}`
      : "";
  return `${doc.title}${ver}\n${doc.uri}${lines}`;
}

export default function DocumentViewer({
  doc,
  onClose,
}: {
  doc: Doc | null;
  onClose: () => void;
}) {
  const [tab, setTab] = useState<Tab>("source");
  const [fullscreen, setFullscreen] = useState(false);
  const [copyMsg, setCopyMsg] = useState<string | null>(null);
  const [jumpLine, setJumpLine] = useState<number | null>(null);
  const sourcePaneRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setTab("source");
    setFullscreen(false);
    setCopyMsg(null);
    setJumpLine(null);
  }, [doc?.id]);

  const detail = useQuery({
    queryKey: ["document", doc?.id],
    queryFn: () => api.document(doc!.id) as Promise<DocDetail>,
    enabled: doc != null,
  });

  const symbols = useQuery({
    queryKey: ["document-symbols", doc?.id],
    queryFn: async () => {
      const res = (await api.documentSymbols(doc!.id, 500)) as { symbols?: DocSymbol[]; count?: number };
      return res.symbols || [];
    },
    enabled: doc != null && tab === "symbols",
  });

  const meta = detail.data?.document || doc;
  const title = meta?.title || (doc ? `Document ${doc.id}` : "Document");
  const uri = meta?.uri || doc?.uri || "";
  const sourceText = useMemo(() => formatSource(detail.data), [detail.data]);
  const language = useMemo(
    () => detectLanguage(uri, meta?.sourceType || doc?.sourceType, meta?.path || doc?.path),
    [uri, meta, doc],
  );
  const chunks = detail.data?.chunks || [];

  const flash = (msg: string) => {
    setCopyMsg(msg);
    window.setTimeout(() => setCopyMsg(null), 1500);
  };

  const doCopy = async (text: string, ok: string) => {
    flash((await copyText(text)) ? ok : "Copy failed");
  };

  const copyCitation = async () => {
    const range = getSelectedLineRange(sourcePaneRef.current);
    await doCopy(citationFor(meta || doc!, range), "Citation copied");
  };

  const goToLine = useCallback((line?: number) => {
    if (!line || line < 1) return;
    setTab("source");
    setJumpLine(line);
  }, []);

  const clearJump = useCallback(() => setJumpLine(null), []);

  if (!doc) return null;

  const sticky = (
    <>
      {fullscreen ? (
        <div className="doc-meta-compact" title={uri}>
          <span className="mono doc-meta-compact-uri">{uri}</span>
          <Button variant="ghost" onClick={() => doCopy(uri, "URI copied")}>
            Copy URI
          </Button>
          <Button variant="ghost" onClick={copyCitation}>
            Copy citation
          </Button>
          {copyMsg && <span className="muted">{copyMsg}</span>}
        </div>
      ) : (
        <dl className="drawer-meta doc-meta-grid">
          <div>
            <dt>ID</dt>
            <dd className="mono">{doc.id}</dd>
          </div>
          <div className="doc-meta-uri">
            <dt>URI</dt>
            <dd>
              <span className="mono">{uri}</span>
              <Button variant="ghost" onClick={() => doCopy(uri, "URI copied")}>
                Copy URI
              </Button>
            </dd>
          </div>
          <div>
            <dt>Type</dt>
            <dd>{meta?.sourceType || doc.sourceType}</dd>
          </div>
          <div>
            <dt>Root</dt>
            <dd className="mono">{meta?.rootName || doc.rootName || "—"}</dd>
          </div>
          <div>
            <dt>Version</dt>
            <dd>{meta?.productVersion || doc.productVersion || "—"}</dd>
          </div>
          {detail.data?.totalChunks != null && (
            <div>
              <dt>Chunks</dt>
              <dd>
                {chunks.length}
                {detail.data.truncated
                  ? ` of ${detail.data.totalChunks} (preview truncated)`
                  : ` / ${detail.data.totalChunks}`}
              </dd>
            </div>
          )}
        </dl>
      )}

      <div className="doc-view-toolbar">
        {(
          [
            ["source", "Source"],
            ["chunks", "Chunks"],
            ["symbols", "Symbols"],
            ["raw", "Raw JSON"],
          ] as const
        ).map(([id, label]) => (
          <Button key={id} variant={tab === id ? "primary" : "secondary"} onClick={() => setTab(id)}>
            {label}
          </Button>
        ))}
        {!fullscreen && (
          <Button variant="secondary" onClick={copyCitation}>
            Copy citation
          </Button>
        )}
        {!fullscreen && copyMsg && <span className="muted">{copyMsg}</span>}
      </div>
    </>
  );

  return (
    <Modal
      open
      title={title}
      onClose={onClose}
      wide
      sticky={sticky}
      fullscreen={fullscreen}
      onToggleFullscreen={() => setFullscreen((v) => !v)}
    >
      {detail.isLoading && <p className="muted">Loading detail…</p>}
      {detail.isError && <p className="error-box">{(detail.error as Error).message}</p>}

      {!detail.isLoading && !detail.isError && tab === "source" && (
        <div className="doc-source-pane" ref={sourcePaneRef}>
          <CodeBlock
            code={sourceText || "(no chunk text)"}
            language={language}
            scrollToLine={jumpLine}
            onScrollToLineHandled={clearJump}
          />
        </div>
      )}

      {!detail.isLoading && !detail.isError && tab === "raw" && (
        <CodeBlock code={JSON.stringify(detail.data, null, 2)} language="json" />
      )}

      {!detail.isLoading && !detail.isError && tab === "chunks" && (
        <div className="doc-chunks-list">
          {chunks.length === 0 && <p className="muted">No chunks in this preview.</p>}
          {chunks.map((c, i) => (
            <details key={c.id ?? i} className="doc-chunk">
              <summary>
                <span className="mono">#{c.ordinal ?? i}</span>
                {(c.startLine || c.endLine) && (
                  <span className="muted">
                    lines {c.startLine ?? "?"}–{c.endLine ?? "?"}
                  </span>
                )}
                {c.heading && <span>{c.heading}</span>}
                {c.startLine != null && c.startLine > 0 && (
                  <Button
                    variant="ghost"
                    onClick={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      goToLine(c.startLine);
                    }}
                  >
                    Go to source
                  </Button>
                )}
                <Button
                  variant="ghost"
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    doCopy(c.body || "", "Chunk copied");
                  }}
                >
                  Copy
                </Button>
              </summary>
              <CodeBlock code={c.body || ""} language={language} showSearch={false} />
            </details>
          ))}
        </div>
      )}

      {tab === "symbols" && (
        <div className="doc-symbols">
          {symbols.isLoading && <p className="muted">Loading symbols…</p>}
          {symbols.isError && <p className="error-box">{(symbols.error as Error).message}</p>}
          {!symbols.isLoading && !symbols.isError && (symbols.data?.length ?? 0) === 0 && (
            <p className="muted">No symbols for this document.</p>
          )}
          {!symbols.isLoading && !symbols.isError && (symbols.data?.length ?? 0) > 0 && (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Kind</th>
                  <th>Signature</th>
                  <th>Lines</th>
                </tr>
              </thead>
              <tbody>
                {symbols.data!.map((s) => (
                  <tr
                    key={s.id}
                    className={s.startLine ? "is-clickable" : undefined}
                    onClick={() => goToLine(s.startLine)}
                    title={s.startLine ? `Go to line ${s.startLine}` : undefined}
                  >
                    <td className="mono">{s.name}</td>
                    <td>{s.kind || "—"}</td>
                    <td className="mono">{s.signature || "—"}</td>
                    <td className="mono">
                      {s.startLine || s.endLine ? (
                        <button
                          type="button"
                          className="linkish"
                          onClick={(e) => {
                            e.stopPropagation();
                            goToLine(s.startLine);
                          }}
                        >
                          {s.startLine ?? "?"}–{s.endLine ?? "?"}
                        </button>
                      ) : (
                        "—"
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </Modal>
  );
}

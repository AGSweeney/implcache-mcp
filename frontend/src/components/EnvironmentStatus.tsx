import { useEffect, useRef, useState } from "react";
import type { ServerInfo } from "../api";
import StatusBadge from "./StatusBadge";

export default function EnvironmentStatus({
  server,
  error,
}: {
  server?: ServerInfo;
  error?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  if (error) {
    return (
      <div className="env-status">
        <StatusBadge variant="danger">Disconnected</StatusBadge>
      </div>
    );
  }

  if (!server) {
    return (
      <div className="env-status">
        <StatusBadge variant="neutral">Connecting…</StatusBadge>
      </div>
    );
  }

  return (
    <div className="env-status" ref={root}>
      <button
        type="button"
        className="env-status-trigger"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((v) => !v)}
      >
        <span className="env-dot ok" aria-hidden="true" />
        <span className="env-status-label">
          <strong>Connected</strong>
          <span className="mono">
            {server.serverVersion} · API v{server.apiVersion} · schema {server.schemaVersion}
          </span>
        </span>
      </button>
      {open && (
        <div className="env-popover" role="dialog" aria-label="Server environment">
          <dl>
            <div>
              <dt>Server</dt>
              <dd className="mono">{server.serverVersion}</dd>
            </div>
            <div>
              <dt>API</dt>
              <dd className="mono">v{server.apiVersion}</dd>
            </div>
            <div>
              <dt>Schema</dt>
              <dd className="mono">{server.schemaVersion}</dd>
            </div>
            <div>
              <dt>Mode</dt>
              <dd>
                <StatusBadge variant={server.readOnly ? "warning" : "info"}>
                  {server.readOnly ? "Read-only" : "Read-write"}
                </StatusBadge>
              </dd>
            </div>
            <div>
              <dt>Auth</dt>
              <dd className="mono">{server.authMode}</dd>
            </div>
            <div>
              <dt>Role</dt>
              <dd>
                <StatusBadge variant="neutral">{server.role || "—"}</StatusBadge>
              </dd>
            </div>
          </dl>
        </div>
      )}
    </div>
  );
}

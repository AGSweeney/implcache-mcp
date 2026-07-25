import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";

type Kind = "local" | "git" | "web" | "pdf";

export default function AddSource() {
  const nav = useNavigate();
  const [kind, setKind] = useState<Kind | "">("");
  const [step, setStep] = useState(0);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  // shared fields
  const [rootName, setRootName] = useState("");
  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [url, setUrl] = useState("");
  const [ref, setRef] = useState("main");
  const [acq, setAcq] = useState("managed_clone");
  const [profile, setProfile] = useState("sphinx");
  const [prefixes, setPrefixes] = useState("");
  const [mode, setMode] = useState<"markdown" | "project">("project");
  const [maxPages, setMaxPages] = useState(100);
  const [product, setProduct] = useState("");
  const [version, setVersion] = useState("");

  async function inspectPdf() {
    setBusy(true);
    setError("");
    try {
      setPreview(await api.inspectPdf({ path }));
      setStep(2);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function runPreview() {
    setBusy(true);
    setError("");
    try {
      if (kind === "local") {
        setPreview(await api.previewLocal({ path, mode, recursive: true, limit: 100 }));
      } else if (kind === "git") {
        setPreview(
          await api.inspectGit({
            remoteUrl: url || undefined,
            localPath: path || undefined,
            ref,
          }),
        );
      } else if (kind === "web") {
        const allowed = prefixes
          .split(/[\n,]+/)
          .map((s) => s.trim())
          .filter(Boolean);
        setPreview(
          await api.previewWeb({
            startUrl: url,
            allowedPrefixes: allowed.length ? allowed : [url],
            maxPages: Math.min(maxPages, 25),
          }),
        );
      }
      setStep(2);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function onUpload(file: File) {
    setBusy(true);
    setError("");
    try {
      const up = await api.upload(file);
      setPath(up.path);
      setPreview(up);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function commit() {
    setBusy(true);
    setError("");
    try {
      if (kind === "local") {
        await api.ingestLocal({ path, rootName, mode, recursive: true });
      } else if (kind === "git") {
        const res = await api.ingestGit({
          name: name || rootName || "repo",
          rootName: rootName || name,
          remoteUrl: url || undefined,
          localPath: path || undefined,
          ref,
          acquisitionMode: acq,
        });
        if (res.opId) nav("/jobs");
      } else if (kind === "web") {
        const startUrl = url;
        const allowed = prefixes
          .split(/[\n,]+/)
          .map((s) => s.trim())
          .filter(Boolean);
        await api.upsertWeb({
          name: name || rootName,
          rootName: rootName || name,
          startUrl,
          profile,
          allowedPrefixes: allowed.length ? allowed : [startUrl],
          product,
          declaredVersion: version,
          enabled: true,
        });
        const res = await api.ingestWeb(name || rootName, { maxPages });
        if (res.opId) nav("/jobs");
      } else if (kind === "pdf") {
        await api.ingestPdf({ path, rootName: rootName || "pdf-docs", product, version });
      }
      nav("/sources");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!kind) {
    return (
      <div>
        <h1>Add Source</h1>
        <div className="grid-metrics">
          {(
            [
              ["local", "Local directory"],
              ["git", "Git repository"],
              ["web", "Documentation website"],
              ["pdf", "PDF document"],
            ] as const
          ).map(([k, label]) => (
            <button key={k} type="button" className="metric" onClick={() => { setKind(k); setStep(0); }}>
              <div className="n" style={{ fontSize: "1rem" }}>
                {label}
              </div>
              <div className="l">{k}</div>
            </button>
          ))}
        </div>
      </div>
    );
  }

  const steps =
    kind === "pdf"
      ? ["Location", "Inspect", "Ingest"]
      : kind === "web"
        ? ["URL", "Scope", "Preview"]
        : kind === "git"
          ? ["Repository", "Scope", "Preview"]
          : ["Location", "Metadata", "Preview"];

  return (
    <div>
      <h1>Add {kind} source</h1>
      <div className="wizard-steps">
        {steps.map((s, i) => (
          <span key={s} className={i === step ? "on" : ""}>
            {i + 1}. {s}
          </span>
        ))}
      </div>
      {error && <div className="error-box">{error}</div>}

      <div className="panel stack">
        {kind === "local" && step === 0 && (
          <>
            <p className="muted">Path is on the ImplCache server host, not this browser workstation.</p>
            <label>
              Server path
              <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="D:/docs/sdk" />
            </label>
            <label>
              Mode
              <select value={mode} onChange={(e) => setMode(e.target.value as "markdown" | "project")}>
                <option value="project">project (source tree)</option>
                <option value="markdown">markdown / HTML docs</option>
              </select>
            </label>
          </>
        )}
        {kind === "local" && step === 1 && (
          <label>
            Root name
            <input value={rootName} onChange={(e) => setRootName(e.target.value)} />
          </label>
        )}

        {kind === "git" && step === 0 && (
          <>
            <label>
              Name
              <input value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              Remote URL
              <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://github.com/org/repo.git" />
            </label>
            <label>
              Or server-side local path
              <input value={path} onChange={(e) => setPath(e.target.value)} />
            </label>
            <label>
              Acquisition
              <select value={acq} onChange={(e) => setAcq(e.target.value)}>
                <option value="snapshot">snapshot</option>
                <option value="managed_clone">managed_clone</option>
                <option value="local_checkout">local_checkout</option>
              </select>
            </label>
          </>
        )}
        {kind === "git" && step === 1 && (
          <>
            <label>
              Root name
              <input value={rootName} onChange={(e) => setRootName(e.target.value)} />
            </label>
            <label>
              Ref
              <input value={ref} onChange={(e) => setRef(e.target.value)} />
            </label>
          </>
        )}

        {kind === "web" && step === 0 && (
          <>
            <label>
              Name
              <input value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              Start URL
              <input value={url} onChange={(e) => setUrl(e.target.value)} />
            </label>
            <label>
              Profile
              <select value={profile} onChange={(e) => setProfile(e.target.value)}>
                <option value="generic">generic</option>
                <option value="sphinx">sphinx</option>
                <option value="doxygen">doxygen</option>
              </select>
            </label>
          </>
        )}
        {kind === "web" && step === 1 && (
          <>
            <label>
              Root name
              <input value={rootName} onChange={(e) => setRootName(e.target.value)} />
            </label>
            <label>
              Allowed prefixes (newline)
              <textarea rows={3} value={prefixes} onChange={(e) => setPrefixes(e.target.value)} placeholder="defaults to start URL" />
            </label>
            <label>
              Max pages
              <input type="number" value={maxPages} onChange={(e) => setMaxPages(Number(e.target.value))} />
            </label>
            <label>
              Product
              <input value={product} onChange={(e) => setProduct(e.target.value)} />
            </label>
            <label>
              Declared version
              <input value={version} onChange={(e) => setVersion(e.target.value)} />
            </label>
          </>
        )}

        {kind === "pdf" && step === 0 && (
          <>
            <p className="muted">Upload a PDF or enter a server-side path.</p>
            <label>
              Server path
              <input value={path} onChange={(e) => setPath(e.target.value)} />
            </label>
            <label>
              Upload
              <input
                type="file"
                accept="application/pdf,.pdf"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) onUpload(f);
                }}
              />
            </label>
            <label>
              Root name
              <input value={rootName} onChange={(e) => setRootName(e.target.value)} />
            </label>
          </>
        )}
        {kind === "pdf" && step === 1 && (
          <>
            <button type="button" disabled={!path || busy} onClick={inspectPdf}>
              Inspect PDF
            </button>
            {preview != null && <pre>{JSON.stringify(preview, null, 2)}</pre>}
          </>
        )}
        {kind === "pdf" && step === 2 && (
          <>
            <label>
              Product
              <input value={product} onChange={(e) => setProduct(e.target.value)} />
            </label>
            <label>
              Version
              <input value={version} onChange={(e) => setVersion(e.target.value)} />
            </label>
            <p className="muted">Preview before ingest — inspect output is shown above when available.</p>
            {preview != null && <pre>{JSON.stringify(preview, null, 2)}</pre>}
          </>
        )}

        {(kind === "local" || kind === "git" || kind === "web") && step === 2 && (
          <>
            <p className="muted">Preview before ingest — no index writes until you confirm.</p>
            {preview != null && <pre>{JSON.stringify(preview, null, 2)}</pre>}
            {preview == null && (
              <button type="button" disabled={busy} onClick={runPreview}>
                {busy ? "Previewing…" : "Run preview"}
              </button>
            )}
          </>
        )}

        <div className="row">
          <button type="button" onClick={() => (step === 0 ? setKind("") : setStep(step - 1))}>
            Back
          </button>
          {step === 0 && kind !== "pdf" && (
            <button className="primary" type="button" onClick={() => setStep(1)}>
              Next
            </button>
          )}
          {step === 1 && kind !== "pdf" && (
            <button className="primary" type="button" disabled={busy} onClick={runPreview}>
              {busy ? "Previewing…" : "Preview"}
            </button>
          )}
          {kind === "pdf" && step === 0 && (
            <button className="primary" type="button" disabled={!path} onClick={() => setStep(1)}>
              Next
            </button>
          )}
          {kind === "pdf" && step === 1 && (
            <button className="primary" type="button" onClick={() => setStep(2)}>
              Skip to ingest
            </button>
          )}
          {step === steps.length - 1 && (
            <button className="primary" type="button" disabled={busy} onClick={commit}>
              {busy ? "Working…" : "Start ingest"}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

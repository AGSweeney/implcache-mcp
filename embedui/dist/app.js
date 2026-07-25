const TOKEN_KEY = "implcache.librarian.token";
const $ = (sel) => document.querySelector(sel);
const state = {
  page: "dashboard",
  token: localStorage.getItem(TOKEN_KEY) || "",
  server: null,
  selectedJobId: "",
  jobsTimer: null,
  jobsES: null,
};

async function api(path, init = {}) {
  const headers = new Headers(init.headers || {});
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (state.token) headers.set("Authorization", `Bearer ${state.token}`);
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const j = await res.json();
      msg = j.message || msg;
    } catch {}
    throw new Error(msg);
  }
  if (res.status === 204) return null;
  return res.json();
}

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function badge(text, cls = "") {
  return `<span class="badge ${cls}">${esc(text)}</span>`;
}

/** Prefer named list fields; never fall back to a bare object (null arrays from Go become objects via ||). */
function asList(raw, ...keys) {
  if (Array.isArray(raw)) return raw;
  if (raw && typeof raw === "object") {
    for (const k of keys) {
      if (Array.isArray(raw[k])) return raw[k];
    }
  }
  return [];
}

async function refreshServer() {
  try {
    state.server = await api("/api/v1/server");
    $("#conn").innerHTML = [
      badge("connected", "ok"),
      `server ${esc(state.server.serverVersion)}`,
      `api v${esc(state.server.apiVersion)}`,
      `schema ${esc(state.server.schemaVersion)}`,
      state.server.readOnly ? "read-only" : "read-write",
      `auth ${esc(state.server.authMode)}`,
      `role ${esc(state.server.role)}`,
    ].join(" · ");
  } catch (e) {
    $("#conn").innerHTML = badge("disconnected", "err") + " " + esc(e.message);
  }
}

async function renderDashboard() {
  const [stats, healthRaw, jobsRaw] = await Promise.all([
    api("/api/v1/library/stats"),
    api("/api/v1/health"),
    api("/api/v1/jobs"),
  ]);
  const issues = asList(healthRaw, "issues").filter((i) => i.severity !== "info").slice(0, 10);
  const jobs = asList(jobsRaw, "operations", "jobs");
  const metrics = [
    ["Sources", stats.sourcesTotal],
    ["Healthy", stats.sourcesOk],
    ["Failed", stats.sourcesFailed],
    ["Documents", stats.documents],
    ["Chunks", stats.chunks],
    ["Symbols", stats.symbols],
    ["Recipes", stats.recipes],
    ["DB bytes", stats.databaseBytes],
    ["Active jobs", stats.activeJobs],
  ];
  $("#main").innerHTML = `
    <h1>Dashboard</h1>
    <div class="grid panel">${metrics.map(([l, n]) => `<div class="metric"><div class="n">${esc(n)}</div><div class="l">${esc(l)}</div></div>`).join("")}</div>
    <div class="panel"><h2>Attention</h2>
      ${issues.length ? `<ul>${issues.map((i) => `<li>${badge(i.severity, i.severity === "error" ? "err" : "warn")} ${esc(i.description)}</li>`).join("")}</ul>` : `<p class="muted">No warnings.</p>`}
    </div>
    <div class="panel"><h2>Recent jobs</h2>
      <table><thead><tr><th>ID</th><th>Source</th><th>State</th></tr></thead>
      <tbody>${jobs.slice(0, 8).map((j) => `<tr><td class="mono">${esc(j.opId?.slice(0, 8))}</td><td>${esc(j.source?.kind)}/${esc(j.source?.id)}</td><td>${badge(j.state)}</td></tr>`).join("")}</tbody></table>
    </div>`;
}

async function renderSources() {
  const raw = await api("/api/v1/sources");
  const sources = asList(raw, "sources");
  $("#main").innerHTML = `
    <h1>Sources</h1>
    <div class="panel row"><button class="primary" data-go="add">Add Source</button></div>
    <div class="panel"><table>
      <thead><tr><th>Name</th><th>Type</th><th>Root</th><th>Status</th><th>Docs</th><th>Actions</th></tr></thead>
      <tbody>${sources
        .map(
          (s) => `<tr>
        <td>${esc(s.title || s.id)}<div class="mono muted">${esc(s.id)}</div></td>
        <td>${badge(s.kind)}</td><td class="mono">${esc(s.rootName)}</td>
        <td>${badge(s.lastStatus || "idle")}</td><td>${esc(s.documentCount)}</td>
        <td>
          ${s.kind === "web" || s.kind === "repo" ? `<button data-refresh="${esc(s.kind)}:${esc(s.id)}">Refresh</button>` : ""}
          ${s.kind !== "local" ? `<button class="danger" data-del="${esc(s.kind)}:${esc(s.id)}">Remove</button>` : ""}
        </td></tr>`,
        )
        .join("")}
      </tbody></table></div>`;
  $("#main").querySelector("[data-go=add]")?.addEventListener("click", () => navigate("add"));
  $("#main").querySelectorAll("[data-refresh]").forEach((btn) =>
    btn.addEventListener("click", async () => {
      const raw = btn.getAttribute("data-refresh") || "";
      const idx = raw.indexOf(":");
      const kind = idx >= 0 ? raw.slice(0, idx) : raw;
      const id = idx >= 0 ? raw.slice(idx + 1) : "";
      try {
        let res;
        if (kind === "web") {
          res = await api(`/api/v1/sources/web/${encodeURIComponent(id)}/refresh`, { method: "POST", body: "{}" });
        }
        if (kind === "repo") {
          res = await api(`/api/v1/sources/git/${encodeURIComponent(id)}/refresh`, { method: "POST", body: "{}" });
        }
        if (res?.opId) state.selectedJobId = res.opId;
        navigate("jobs");
      } catch (e) {
        alert(e.message);
      }
    }),
  );
  $("#main").querySelectorAll("[data-del]").forEach((btn) =>
    btn.addEventListener("click", async () => {
      const [kind, id] = btn.getAttribute("data-del").split(":");
      if (!confirm(`Remove ${kind}/${id}?`)) return;
      try {
        if (kind === "web") await api(`/api/v1/sources/web/${encodeURIComponent(id)}`, { method: "DELETE" });
        if (kind === "repo") await api(`/api/v1/sources/git/${encodeURIComponent(id)}`, { method: "DELETE" });
        if (kind === "pdf") await api(`/api/v1/sources/pdf?uri=${encodeURIComponent(id)}`, { method: "DELETE" });
        renderSources();
      } catch (e) {
        alert(e.message);
      }
    }),
  );
}

function renderAdd() {
  $("#main").innerHTML = `
    <h1>Add Source</h1>
    <div class="panel stack">
      <label>Type
        <select id="kind">
          <option value="local">Local directory</option>
          <option value="git">Git repository</option>
          <option value="web">Website</option>
          <option value="pdf">PDF</option>
        </select>
      </label>
      <p class="muted" id="kindHint"></p>

      <label class="f-name">Name <input id="name" placeholder="source name"/></label>
      <label class="f-root">Root name <input id="root" placeholder="knowledge root (e.g. sdk_docs)"/></label>

      <label class="f-path"><span id="pathLabel">Server path</span>
        <input id="path" placeholder=""/>
      </label>
      <p class="muted f-path-note" id="pathNote"></p>

      <label class="f-mode">Ingest mode
        <select id="mode">
          <option value="project">project (source tree)</option>
          <option value="markdown">markdown / HTML docs</option>
        </select>
      </label>

      <label class="f-ref">Git ref <input id="ref" value="main" placeholder="main, v1.0, commit SHA"/></label>
      <label class="f-acq">Acquisition
        <select id="acq">
          <option value="managed_clone">managed_clone (from remote URL)</option>
          <option value="local_checkout">local_checkout (server path)</option>
          <option value="snapshot">snapshot</option>
        </select>
      </label>

      <label class="f-profile">Crawl profile
        <select id="profile">
          <option value="generic">generic</option>
          <option value="sphinx">sphinx</option>
          <option value="doxygen">doxygen</option>
        </select>
      </label>
      <label class="f-max">Max pages <input id="maxPages" type="number" value="50" min="1"/></label>
      <label class="f-prefixes">Allowed URL prefixes (optional)
        <textarea id="prefixes" rows="2" placeholder="defaults to start URL"></textarea>
      </label>

      <label class="f-upload">PDF upload
        <input id="file" type="file" accept=".pdf,application/pdf"/>
      </label>

      <div class="row">
        <button id="preview">Preview</button>
        <button class="primary" id="go">Start ingest</button>
      </div>
      <pre id="out" class="muted"></pre>
    </div>`;

  const hints = {
    local: "Index a directory on the ImplCache server host (not this browser).",
    git: "Clone or index a Git repository. Use a remote URL or a server-side checkout path.",
    web: "Crawl a documentation site within allowed URL prefixes.",
    pdf: "Upload a PDF or give a server-side path to a .pdf file.",
  };
  const pathLabels = {
    local: "Server directory path",
    git: "Remote URL or server checkout path",
    web: "Start URL",
    pdf: "Server PDF path",
  };
  const pathPlaceholders = {
    local: "D:/docs/sdk",
    git: "https://github.com/org/repo.git",
    web: "https://docs.example.com/en/latest/",
    pdf: "D:/manuals/guide.pdf",
  };
  const pathNotes = {
    local: "Path must exist on the machine running implcache-mcp.",
    git: "HTTPS/SSH remotes use managed_clone; a local path uses local_checkout.",
    web: "Crawl stays inside the start URL prefix unless you list others.",
    pdf: "Upload fills the path automatically, or type a path on the server.",
  };

  function syncKind() {
    const kind = $("#kind").value;
    const show = (sel, on) =>
      document.querySelectorAll(sel).forEach((el) => {
        el.hidden = !on;
      });
    show(".f-name", kind === "git" || kind === "web");
    show(".f-root", true);
    show(".f-path", true);
    show(".f-path-note", true);
    show(".f-mode", kind === "local");
    show(".f-ref", kind === "git");
    show(".f-acq", kind === "git");
    show(".f-profile", kind === "web");
    show(".f-max", kind === "web");
    show(".f-prefixes", kind === "web");
    show(".f-upload", kind === "pdf");
    $("#kindHint").textContent = hints[kind] || "";
    $("#pathLabel").textContent = pathLabels[kind] || "Path";
    $("#path").placeholder = pathPlaceholders[kind] || "";
    $("#pathNote").textContent = pathNotes[kind] || "";
  }

  $("#kind").addEventListener("change", syncKind);
  syncKind();

  $("#preview").addEventListener("click", async () => {
    const kind = $("#kind").value;
    const path = $("#path").value.trim();
    try {
      let res;
      if (kind === "local") {
        res = await api("/api/v1/sources/local/preview", {
          method: "POST",
          body: JSON.stringify({ path, mode: $("#mode").value, recursive: true, limit: 100 }),
        });
      } else if (kind === "git") {
        res = await api("/api/v1/sources/git/inspect", {
          method: "POST",
          body: JSON.stringify({
            remoteUrl: path.includes("://") ? path : undefined,
            localPath: path.includes("://") ? undefined : path,
            ref: $("#ref").value.trim() || "main",
          }),
        });
      } else if (kind === "web") {
        const allowed = $("#prefixes")
          .value.split(/[\n,]+/)
          .map((s) => s.trim())
          .filter(Boolean);
        res = await api("/api/v1/sources/web/preview", {
          method: "POST",
          body: JSON.stringify({
            startUrl: path,
            allowedPrefixes: allowed.length ? allowed : [path],
            maxPages: Number($("#maxPages").value) || 10,
          }),
        });
      } else if (kind === "pdf") {
        res = await api("/api/v1/sources/pdf/inspect", { method: "POST", body: JSON.stringify({ path }) });
      }
      $("#out").textContent = JSON.stringify(res, null, 2);
    } catch (e) {
      $("#out").textContent = e.message;
    }
  });

  $("#file").addEventListener("change", async (e) => {
    const f = e.target.files?.[0];
    if (!f) return;
    const fd = new FormData();
    fd.append("file", f);
    try {
      const up = await api("/api/v1/uploads", { method: "POST", body: fd });
      $("#path").value = up.path;
      $("#out").textContent = JSON.stringify(up, null, 2);
    } catch (err) {
      $("#out").textContent = err.message;
    }
  });

  $("#go").addEventListener("click", async () => {
    const kind = $("#kind").value;
    const name = $("#name")?.value?.trim() || "";
    const root = $("#root").value.trim() || name;
    const path = $("#path").value.trim();
    try {
      let res;
      if (kind === "local") {
        res = await api("/api/v1/sources/local/ingest", {
          method: "POST",
          body: JSON.stringify({ path, rootName: root, mode: $("#mode").value, recursive: true }),
        });
      } else if (kind === "git") {
        const isURL = path.includes("://");
        res = await api("/api/v1/sources/git", {
          method: "POST",
          body: JSON.stringify({
            name: name || root || "repo",
            rootName: root || name,
            remoteUrl: isURL ? path : undefined,
            localPath: isURL ? undefined : path,
            acquisitionMode: $("#acq").value || (isURL ? "managed_clone" : "local_checkout"),
            ref: $("#ref").value.trim() || "main",
          }),
        });
      } else if (kind === "web") {
        const srcName = name || root;
        const allowed = $("#prefixes")
          .value.split(/[\n,]+/)
          .map((s) => s.trim())
          .filter(Boolean);
        await api("/api/v1/sources/web", {
          method: "POST",
          body: JSON.stringify({
            name: srcName,
            rootName: root || srcName,
            startUrl: path,
            profile: $("#profile").value || "generic",
            allowedPrefixes: allowed.length ? allowed : [path],
            enabled: true,
          }),
        });
        res = await api(`/api/v1/sources/web/${encodeURIComponent(srcName)}/ingest`, {
          method: "POST",
          body: JSON.stringify({ maxPages: Number($("#maxPages").value) || 50 }),
        });
      } else if (kind === "pdf") {
        res = await api("/api/v1/sources/pdf/ingest", {
          method: "POST",
          body: JSON.stringify({ path, rootName: root || "pdf-docs" }),
        });
      }
      $("#out").textContent = JSON.stringify(res, null, 2);
      if (res?.opId) {
        state.selectedJobId = res.opId;
        navigate("jobs");
      } else navigate("sources");
    } catch (e) {
      $("#out").textContent = e.message;
    }
  });
}

function stopJobsLive() {
  if (state.jobsTimer) {
    clearInterval(state.jobsTimer);
    state.jobsTimer = null;
  }
  if (state.jobsES) {
    state.jobsES.close();
    state.jobsES = null;
  }
}

function formatProgress(p) {
  if (!p) return "";
  const parts = [];
  if (p.phase) parts.push(p.phase);
  if (p.done != null) parts.push(p.total ? `${p.done}/${p.total}` : String(p.done));
  if (p.current) parts.push(p.current);
  if (p.message) parts.push(p.message);
  if (p.bytes) parts.push(`${p.bytes} B`);
  return parts.join(" · ");
}

async function renderJobs() {
  stopJobsLive();
  const raw = await api("/api/v1/jobs");
  const jobs = asList(raw, "operations", "jobs");
  if (!state.selectedJobId) {
    const running = jobs.find((j) => j.state === "running" || j.state === "cancelling");
    if (running) state.selectedJobId = running.opId;
  }
  const selected = state.selectedJobId;
  $("#main").innerHTML = `
    <h1>Jobs</h1>
    <div class="panel"><table>
      <thead><tr><th>ID</th><th>Source</th><th>State</th><th>Progress</th><th></th></tr></thead>
      <tbody id="jobsBody">${jobs
        .map(
          (j) => `<tr data-job="${esc(j.opId)}" class="${j.opId === selected ? "on" : ""}">
        <td class="mono"><button type="button" data-select="${esc(j.opId)}">${esc(j.opId?.slice(0, 8))}</button></td>
        <td>${esc(j.source?.kind)}/${esc(j.source?.id)}</td>
        <td class="job-state">${badge(j.state)}</td>
        <td class="job-prog">${esc(formatProgress(j.progress))}</td>
        <td>${j.state === "running" || j.state === "cancelling" ? `<button data-cancel="${esc(j.opId)}">Cancel</button>` : ""}</td>
      </tr>`,
        )
        .join("")}
      </tbody></table></div>
    <div class="panel"><h2>Live</h2><pre id="live" class="muted">${esc(
      selected ? formatProgress(jobs.find((j) => j.opId === selected)?.progress) || "Waiting for progress…" : "Select a job…",
    )}</pre></div>`;

  $("#main").querySelectorAll("[data-select]").forEach((btn) =>
    btn.addEventListener("click", () => {
      state.selectedJobId = btn.getAttribute("data-select");
      renderJobs();
    }),
  );
  $("#main").querySelectorAll("[data-cancel]").forEach((btn) =>
    btn.addEventListener("click", async () => {
      await api(`/api/v1/jobs/${btn.getAttribute("data-cancel")}/cancel`, { method: "POST" });
      renderJobs();
    }),
  );

  if (!selected) return;

  const paint = (op) => {
    if (!op) return;
    const live = $("#live");
    if (live) live.textContent = JSON.stringify(op.progress || {}, null, 2);
    const row = document.querySelector(`tr[data-job="${op.opId}"]`);
    if (row) {
      const stateCell = row.querySelector(".job-state");
      const progCell = row.querySelector(".job-prog");
      if (stateCell) stateCell.innerHTML = badge(op.state);
      if (progCell) progCell.textContent = formatProgress(op.progress);
    }
  };

  // Poll always (table + live). SSE is an accelerator when available.
  const tick = async () => {
    try {
      const op = await api(`/api/v1/jobs/${selected}`);
      paint(op);
      if (op.state !== "running" && op.state !== "queued" && op.state !== "cancelling") {
        stopJobsLive();
        // One more list refresh for final state.
        const list = asList(await api("/api/v1/jobs"), "operations", "jobs");
        const body = $("#jobsBody");
        if (body) {
          /* keep selection; refresh row via paint already */
        }
        const still = list.find((j) => j.opId === selected);
        if (still) paint(still);
      }
    } catch {
      /* ignore */
    }
  };
  tick();
  state.jobsTimer = setInterval(tick, 1000);

  const q = state.token ? `?access_token=${encodeURIComponent(state.token)}` : "";
  try {
    const es = new EventSource(`/api/v1/jobs/${selected}/events${q}`);
    state.jobsES = es;
    es.addEventListener("progress", (ev) => {
      try {
        const p = JSON.parse(ev.data);
        paint({ opId: selected, state: "running", progress: p });
      } catch {
        const live = $("#live");
        if (live) live.textContent = ev.data;
      }
    });
    es.onerror = () => {
      // Keep polling; close SSE to avoid reconnect spam.
      es.close();
      if (state.jobsES === es) state.jobsES = null;
    };
  } catch {
    /* poll only */
  }
}

async function renderLibrary() {
  const raw = await api("/api/v1/library/documents?limit=50&offset=0");
  const docs = asList(raw, "documents");
  $("#main").innerHTML = `
    <h1>Library</h1>
    <div class="panel"><table>
      <thead><tr><th>Title</th><th>URI</th><th>Type</th><th>Root</th></tr></thead>
      <tbody>${docs.map((d) => `<tr data-id="${d.id}"><td>${esc(d.title)}</td><td class="mono">${esc(d.uri)}</td><td>${esc(d.sourceType)}</td><td>${esc(d.rootName)}</td></tr>`).join("")}</tbody>
    </table>
    <p class="muted">Total ${esc(raw.total)}</p>
    <pre id="doc"></pre></div>`;
  $("#main").querySelectorAll("tr[data-id]").forEach((tr) =>
    tr.addEventListener("click", async () => {
      const doc = await api(`/api/v1/library/documents/${tr.getAttribute("data-id")}`);
      $("#doc").textContent = JSON.stringify(doc, null, 2);
    }),
  );
}

async function renderRoots() {
  const [rootsRaw, groupsRaw] = await Promise.all([api("/api/v1/roots"), api("/api/v1/root-groups")]);
  const roots = asList(rootsRaw, "roots");
  const groups = asList(groupsRaw, "groups", "rootGroups");
  $("#main").innerHTML = `
    <h1>Roots</h1>
    <div class="panel"><h2>Roots</h2><ul>${roots.map((r) => `<li class="mono">${esc(r)}</li>`).join("")}</ul></div>
    <div class="panel"><h2>Groups</h2>
      ${groups.map((g) => `<div><strong>${esc(g.name)}</strong><ul>${asList(g.members).map((m) => `<li class="mono">${esc(m.rootName)} (${esc(m.priority)})</li>`).join("")}</ul></div>`).join("") || "<p class='muted'>None</p>"}
    </div>
    <div class="panel stack">
      <h2>Upsert group</h2>
      <label>Name <input id="gname"/></label>
      <label>Members (comma) <input id="gmembers"/></label>
      <button class="primary" id="gsave">Save</button>
    </div>`;
  $("#gsave").addEventListener("click", async () => {
    const name = $("#gname").value.trim();
    const members = $("#gmembers")
      .value.split(/[,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
      .map((rootName, i) => ({ rootName, priority: 100 - i }));
    await api(`/api/v1/root-groups/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ members }),
    });
    renderRoots();
  });
}

async function renderSearch() {
  $("#main").innerHTML = `
    <h1>Search Lab</h1>
    <div class="panel stack">
      <label>Mode <select id="mode"><option value="knowledge">Knowledge</option><option value="symbol">Symbol</option><option value="context">Context</option></select></label>
      <label>Query / name / task <input id="q"/></label>
      <label>Root <input id="root"/></label>
      <label><input type="checkbox" id="sem"/> Semantic</label>
      <label><input type="checkbox" id="ex" checked/> Explain</label>
      <button class="primary" id="run">Run</button>
      <pre id="out"></pre>
    </div>`;
  $("#run").addEventListener("click", async () => {
    try {
      const mode = $("#mode").value;
      let out;
      if (mode === "symbol") {
        out = await api("/api/v1/search/symbols", {
          method: "POST",
          body: JSON.stringify({ name: $("#q").value, roots: $("#root").value ? [$("#root").value] : [], limit: 20 }),
        });
      } else if (mode === "context") {
        out = await api("/api/v1/search/context", {
          method: "POST",
          body: JSON.stringify({
            task: $("#q").value,
            projectRoot: $("#root").value || undefined,
            semantic: $("#sem").checked,
          }),
        });
      } else {
        out = await api("/api/v1/search", {
          method: "POST",
          body: JSON.stringify({
            query: $("#q").value,
            rootName: $("#root").value || undefined,
            semantic: $("#sem").checked,
            explain: $("#ex").checked,
            limit: 20,
          }),
        });
      }
      $("#out").textContent = JSON.stringify(out, null, 2);
    } catch (e) {
      $("#out").textContent = e.message;
    }
  });
}

async function renderHealth() {
  const raw = await api("/api/v1/health");
  const issues = asList(raw, "issues");
  $("#main").innerHTML = `
    <h1>Health</h1>
    <div class="panel"><table>
      <thead><tr><th>Severity</th><th>Code</th><th>Source</th><th>Description</th><th>Action</th></tr></thead>
      <tbody>${issues.map((i) => `<tr>
        <td>${badge(i.severity, i.severity === "error" ? "err" : i.severity === "warning" ? "warn" : "")}</td>
        <td class="mono">${esc(i.code)}</td>
        <td class="mono">${esc(i.sourceKind)}/${esc(i.sourceId)}</td>
        <td>${esc(i.description)}</td>
        <td class="muted">${esc(i.recommendedAction)}</td>
      </tr>`).join("")}</tbody></table></div>`;
}

function renderSettings() {
  $("#main").innerHTML = `
    <h1>Settings</h1>
    <div class="panel stack">
      <pre>${esc(JSON.stringify(state.server, null, 2))}</pre>
      <label>API bearer token <input id="tok" type="password" value="${esc(state.token)}"/></label>
      <div class="row">
        <button class="primary" id="save">Save token</button>
        <button id="clear">Clear</button>
      </div>
      <p class="muted">Use <code>-librarian-token</code> on the server for shared deployments. HTTPS + reverse proxy recommended for non-loopback binds (<code>-allow-remote-http</code>).</p>
    </div>`;
  $("#save").addEventListener("click", () => {
    state.token = $("#tok").value.trim();
    if (state.token) localStorage.setItem(TOKEN_KEY, state.token);
    else localStorage.removeItem(TOKEN_KEY);
    refreshServer();
  });
  $("#clear").addEventListener("click", () => {
    state.token = "";
    localStorage.removeItem(TOKEN_KEY);
    $("#tok").value = "";
    refreshServer();
  });
}

async function navigate(page) {
  if (page !== "jobs") stopJobsLive();
  state.page = page;
  document.querySelectorAll(".nav a").forEach((a) => a.classList.toggle("active", a.dataset.page === page));
  try {
    if (page === "dashboard") await renderDashboard();
    else if (page === "sources") await renderSources();
    else if (page === "add") renderAdd();
    else if (page === "jobs") await renderJobs();
    else if (page === "library") await renderLibrary();
    else if (page === "roots") await renderRoots();
    else if (page === "search") await renderSearch();
    else if (page === "health") await renderHealth();
    else if (page === "logs") await renderLogs();
    else if (page === "settings") renderSettings();
  } catch (e) {
    $("#main").innerHTML = `<div class="error">${esc(e.message)}</div>`;
  }
}

async function renderLogs() {
  const raw = await api("/api/v1/logs?limit=100");
  const lines = asList(raw, "lines").slice().reverse();
  $("#main").innerHTML = `
    <h1>Logs</h1>
    <div class="panel"><table>
      <thead><tr><th>At</th><th>Level</th><th>Message</th></tr></thead>
      <tbody>${lines.map((l) => `<tr><td class="mono">${esc(l.at)}</td><td>${esc(l.level)}</td><td>${esc(l.message)}</td></tr>`).join("") || "<tr><td colspan=3 class='muted'>No lines yet</td></tr>"}</tbody>
    </table></div>`;
}

document.querySelectorAll(".nav a").forEach((a) =>
  a.addEventListener("click", (e) => {
    e.preventDefault();
    navigate(a.dataset.page);
  }),
);

refreshServer().then(() => navigate("dashboard"));
setInterval(refreshServer, 15000);

# ImplCache MCP — Users Manual

Operator guide for running and using ImplCache with coding agents. This package is meant to be **installed and configured**, not rebuilt from source.

Related guides: [INSTALLATION.md](INSTALLATION.md) · [CONFIGURATION.md](CONFIGURATION.md) · [AGENT_GUIDE.md](AGENT_GUIDE.md)

---

## 1. What ImplCache does

ImplCache is a **local knowledge server** for coding agents. You load documentation and source into a SQLite database; agents then ask for a **small, cited implementation package** instead of searching the web or reading entire trees.

Typical results include:

- Exact API / symbol names and signatures  
- One or more project-local examples  
- Initialization order and constraints  
- Pitfalls and citations back to your corpus  

Full documents stay available when the agent needs more depth.

**Status:** Pre-1.0. Schema and tool details can still change between releases. An incompatible database is refused—delete it and re-ingest.

---

## 2. Core concepts

### Knowledge roots

A **root** is a named corpus (e.g. `my_app`, `vendor-sdk-docs`). Keep different products in **separate** roots. If a query is ambiguous, tools return `needsChoice` and a list of roots—choose explicitly; do not guess across families.

### Document URIs

Indexed files use portable URIs (not machine-specific absolute paths):

```text
project://{rootName}/{relative/path}   # docs, HTML, project, web pages
pdf://{rootName}/{file}                # PDF manuals
git://{rootName}/{relative/path}       # Git-ingested trees
```

### Authority (ranking)

Higher-authority material is preferred when ranking hits:

1. Current project code  
2. Related internal projects  
3. Human-reviewed recipes  
4. Official examples / documentation  
5. Generated summaries (e.g. auto-compiled playbooks)  
6. Third-party / unknown  

**Freshness** (current, stale, unknown, …) is tracked separately from authority.

### Agent vs admin

| Profile | Purpose |
|---------|---------|
| **Agent** (`-mode agent`) | Daily coding: retrieval tools only |
| **Admin** (`-mode admin`) | Ingest, delete, web/PDF/Git, recipes, inventory tools |

See [CONFIGURATION.md](CONFIGURATION.md).

---

## 3. Getting running

1. Install binaries — [INSTALLATION.md](INSTALLATION.md)  
2. Ingest corpora — [INGEST.md](INGEST.md)  
3. Configure Cursor (or another MCP client) with absolute `-db` paths — [CONFIGURATION.md](CONFIGURATION.md)  
4. Ask the agent to call `get_implementation_context` for a real task  

Optional: run the **Librarian** browser UI for corpus management — [LIBRARIAN.md](LIBRARIAN.md).

---

## 4. Loading knowledge

Prefer **`ingestcli`** for large trees so interactive MCP sessions stay responsive.

| Content | Typical command |
|---------|-----------------|
| Markdown / HTML docs | `ingestcli -db … -mode markdown -root NAME -path DIR` |
| Application source | `ingestcli -db … -mode project -root NAME -path DIR` |
| One documentation URL | `ingestcli -db … -mode url -root NAME -url URL` |
| PDF (local text) | `ingestcli -db … -mode pdf-ingest -root NAME -path FILE` |
| Git repository | `ingestcli -db … -mode repo-ingest …` |
| Site-wide crawl | Librarian UI / admin MCP tools (not the CLI yet) |

Unchanged files are skipped via content hash. Defaults cap about 50 000 files per run and 8 MiB per file.

**Naming tips**

- Use stable root ids (`my_app`, `acme-sdk-v5`).  
- Keep vendor help and your app in separate roots.  
- Re-run ingest after vendor or project updates.  

Full detail: [INGEST.md](INGEST.md).

---

## 5. Using with coding agents

### Default loop

```text
1. Identify task, language, technology, project
2. Call get_implementation_context (with preferredRoots when known)
3. Implement from APIs, sequence, examples, pitfalls
4. If stuck: find_symbol → search_knowledge → get_document
5. Avoid dumping many files or web-searching first
```

### Agent tools

| Tool | Use for |
|------|---------|
| `get_implementation_context` | **First** call for any coding task |
| `find_symbol` | Named API / type lookup |
| `search_knowledge` | Broader full-text search in a root |
| `get_document` | Full body of one cited document |
| `list_roots` | See which corpora are loaded |

When a tool returns `needsChoice` / `availableRoots`, ask which root applies and retry with `rootName` or `preferredRoots`.

Longer agent guidance: [AGENT_GUIDE.md](AGENT_GUIDE.md). Tool parameters: [TOOLS.md](TOOLS.md).

### Example request

```json
{
  "task": "register a menubar pushbutton in the device SDK",
  "language": "c",
  "technology": "device-sdk",
  "preferredRoots": ["my_app", "device-sdk"],
  "maxContextTokens": 2500
}
```

### Optional semantic search

Enable with `-enable-semantic` or `semantic: true` on supported calls. This adds sparse term similarity on top of full-text search; it is not an embedding model.

---

## 6. Librarian (browser console)

Run:

```bash
implcache-mcp -db ./implcache.db -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
```

Open `http://127.0.0.1:8080/`.

Pages include Dashboard, Sources, Jobs, Library, Roots, Search Lab, Health, Logs, and Settings.

Optional API tokens: `-librarian-token` (admin), `-librarian-viewer-token` (read-only). Details: [LIBRARIAN.md](LIBRARIAN.md).

---

## 7. Recipes (`vomit`)

Admin-only tool that compiles a longer Markdown playbook from your local knowledge.

| Do | Don’t |
|----|--------|
| Use for durable playbooks you will reuse | Call it for every small edit |
| Optionally save with `saveRecipe` | Treat generated recipes as more authoritative than project code |
| Have a human review important recipes | Write outside the `-output-root` jail |

Generated entries rank below human-reviewed recipes and project code.

---

## 8. Day-to-day operations

1. Keep agent MCP on `-mode agent` for coding.  
2. Use `ingestcli` or Librarian/admin when corpora change.  
3. After replacing binaries, **reload MCP** in the client.  
4. Prefer one writer at a time when ingesting (SQLite).  
5. Back up `implcache.db` if the corpus is costly to rebuild.  

### Useful maintenance

| Task | How |
|------|-----|
| See roots | `list_roots` or Librarian Roots |
| Delete one document | Admin `delete_document` |
| Delete a whole prefix | `delete_by_uri_prefix` / `ingestcli -mode delete-prefix` |
| Refresh web or Git | Librarian or admin refresh tools |
| Read-only serving | `-readonly` |

---

## 9. Troubleshooting

| Symptom | What to try |
|---------|-------------|
| No tools / old tools | Reload MCP; check absolute path to the binary |
| Empty search | `list_roots`; confirm `-db` matches the DB you ingested |
| `needsChoice` | Ask which root; pass `rootName` / `preferredRoots` |
| Ingest denied | Use `-mode admin`; ensure not `-readonly`; `-allow-ingest=true` |
| HTTP mutations denied | Pass `-enable-http-mutations`; check admin vs viewer token |
| Bind / remote HTTP refused | Stay on loopback, or pass `-allow-remote-http` deliberately |
| Librarian UI missing | Need both `-http` and `-enable-librarian` |
| Database refused on upgrade | Schema mismatch: delete DB (+ wal/shm) and re-ingest |
| DB locked | Stop other ingest/server writers |
| Agent inventing APIs | Prefer `get_implementation_context` + `find_symbol`; check `coverage` |

---

## 10. Security notes

- Keep HTTP on **loopback** unless you intentionally expose it.  
- For shared hosts: reverse proxy + HTTPS + `-librarian-token`.  
- Mutations over HTTP stay off unless `-enable-http-mutations`.  
- `vomit` file writes are jailed under `-output-root`.  
- Ingest skips or refuses symlink paths.  
- Do not commit knowledge databases that contain proprietary corpora.  

---

## 11. Limitations

- Pre-1.0: contracts and ranking may change.  
- Symbol extraction covers common languages (Go, C/C++/C#, Python, JS/TS, Java) heuristically.  
- Web crawl does not execute JavaScript; authenticated portals are limited.  
- PDF Stage 1 is local text extraction (OCR not included).  
- Git ingest needs system `git`; credentials stay in Git/SSH—not in the database.  
- Token estimates in responses are approximate.  
- Librarian job progress is in-memory (survives browser reload, not process restart).  

---

## 12. Glossary

| Term | Meaning |
|------|---------|
| **Root** | Named corpus scoping search and URIs |
| **Implementation package** | Budgeted answer from `get_implementation_context` |
| **Agent mode** | Retrieval-only MCP tools |
| **Admin mode** | Full tool surface including ingest and recipes |
| **Librarian** | Embedded browser UI + REST API |
| **ingestcli** | Offline CLI for loading corpora |
| **needsChoice** | Ambiguous root; choose and retry |
| **vomit** | Playbook/recipe compiler |
| **Semantic search** | Optional sparse term similarity (not embeddings) |

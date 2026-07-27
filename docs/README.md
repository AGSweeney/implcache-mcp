# Documentation index

ImplCache MCP is a **local implementation-context server** for coding agents. It stores documentation and source in SQLite, then returns compact, cited packages that help agents write correct software.

**Start here (developers / source tree):** [USERS_MANUAL.md](USERS_MANUAL.md) — install from source, ingest, Cursor/Librarian setup, agent usage, operations, and troubleshooting.

**End-user release package:** [../dist/](../dist/) — sanitized install/config docs and users manual for binary consumers (no source-tree recreation). Pack binaries with `scripts/pack-dist.ps1`.

| Document | Contents |
|----------|----------|
| [USERS_MANUAL.md](USERS_MANUAL.md) | **Source-tree operator manual** (developers) |
| [../dist/README.md](../dist/README.md) | **End-user package** index (ship with binaries) |
| [../README.md](../README.md) | Product overview, quick start, Cursor config |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Purpose, layers, retrieval pipeline, package layout |
| [AGENT_GUIDE.md](AGENT_GUIDE.md) | How coding agents should call tools |
| [TOOLS.md](TOOLS.md) | Full MCP tool reference (agent + admin + Librarian) |
| [MODES.md](MODES.md) | Agent/admin tool surface, HTTP safety, versioning |
| [MANIFEST.md](MANIFEST.md) | `.implcache.yaml` workspace configuration |
| [DATA_MODEL.md](DATA_MODEL.md) | URIs, schema, authority, recipes, symbols |
| [INGEST.md](INGEST.md) | Markdown/HTML/project, web, PDF, and Git ingest + CLI |
| [CREATE_LIBRARYDOCS.md](CREATE_LIBRARYDOCS.md) | create-librarydocs skill (author `LibraryDocs/` packages) |
| [LIBRARYDOCS.md](LIBRARYDOCS.md) | LibraryDocs ingest deliverable notes (no schema change) |
| [REMOTE.md](REMOTE.md) | Remote / Jetson Orin NX LAN deploy + Cursor `mcp.json` URL entry |
| [OPERATIONS.md](OPERATIONS.md) | Build, flags, security, validation, evaluation, [limitations/risks](OPERATIONS.md#limitations-and-risks) |
| [API_V1.md](API_V1.md) | Librarian REST admin API (`/api/v1`) + embedded UI |
| [ANALYTICS_USERS_GUIDE.md](ANALYTICS_USERS_GUIDE.md) | Analytics Dashboard User Guide (screenshots) |
| [PRD_LOCAL_ANALYTICS.md](PRD_LOCAL_ANALYTICS.md) | Analytics product requirements / metrics design |
| [../testdata/validation/README.md](../testdata/validation/README.md) | Real-source validation harness (`cmd/sourcevalidate`) |

# ImplCache MCP

Local implementation-context server for coding agents (Cursor and other MCP clients).

ImplCache indexes your documentation, source trees, PDFs, mirrored web docs, and Git repositories into a SQLite knowledge base, then returns compact, cited context so agents can implement features without guessing APIs or scanning the whole tree.

> Return the smallest sufficient package of accurate, implementation-ready context for the current coding task.

---

## What’s in this package

| Item | Purpose |
|------|---------|
| `implcache-mcp` / `implcache-mcp.exe` | MCP server (stdio or HTTP) + optional Librarian UI |
| `ingestcli` / `ingestcli.exe` | Offline tool to load large corpora into the database |
| `implcache.db` | **Sanitized empty** starter database (schema only — no corpora) |
| `docs/` | End-user documentation (includes Librarian screenshots) |
| `VERSION` | Build identifier from the pack step |
| `run-librarian.cmd` | Windows helper: start Librarian UI on `:8080` |
| `run-agent.cmd` | Windows helper: start stdio MCP in agent mode |
| `LICENSE` / `NOTICE` | License and third-party notices |

This folder is meant to be **self-contained**. After packing (or unzipping a release), you can run locally without cloning the source repository and without installing Go or Node.js.

If a binary is missing, rebuild from a source checkout with:

```powershell
pwsh ./scripts/pack-dist.ps1
```

---

## Start here

1. **[docs/INSTALLATION.md](docs/INSTALLATION.md)** — place binaries, create a database, smoke-test  
2. **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)** — Cursor MCP JSON, modes, flags, workspace file  
3. **[docs/USERS_MANUAL.md](docs/USERS_MANUAL.md)** — full operator manual (ingest, agents, Librarian, maintenance)  
4. **[docs/AGENT_GUIDE.md](docs/AGENT_GUIDE.md)** — how coding agents should call tools  

| Also useful | Contents |
|-------------|----------|
| [docs/INGEST.md](docs/INGEST.md) | Loading docs, source, web, PDF, Git |
| [docs/TOOLS.md](docs/TOOLS.md) | MCP tool reference |
| [docs/LIBRARIAN.md](docs/LIBRARIAN.md) | Browser UI, REST API, and screenshots |

---

## Quick start (from this folder)

Create or open a database next to the binaries, then either:

**Coding agent (stdio MCP)** — retrieval tools only:

```text
./implcache-mcp -db ./implcache.db -mode agent
```

Point Cursor (or another MCP client) at the absolute path of `implcache-mcp` with those args.

**Librarian / corpus admin** — browser UI on loopback:

```text
./implcache-mcp -db ./implcache.db -http :8080 ^
  -enable-librarian -enable-http-mutations -mode admin
```

Then open `http://127.0.0.1:8080/`.

**Load corpora** (examples):

```text
./ingestcli -db ./implcache.db -mode markdown -root my-docs -path "C:/path/to/docs"
./ingestcli -db ./implcache.db -mode project -root my-app -path "D:/work/my-app"
```

Suggested layout after first use:

```text
ImplCache/                 (this package)
  implcache-mcp.exe
  ingestcli.exe
  docs/
  LICENSE
  NOTICE
  VERSION
  README.md
  implcache.db             (empty starter; grows as you ingest)
```

---

## Requirements

- The release binaries for your OS (Windows, macOS, or Linux) in this folder  
- For Git repository ingest: a system `git` on `PATH`  
- An MCP-capable client (e.g. Cursor) for agent use  
- Optional: a browser for the Librarian UI  

No Go toolchain, Node.js, or C compiler is required to **run** this package.

---

## License

MIT — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

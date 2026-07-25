# Tool modes and HTTP safety

## Modes

| Mode | Flag | Registered tools |
|------|------|------------------|
| **agent** (default) | `-mode agent` | `get_implementation_context`, `find_symbol`, `search_knowledge`, `get_document`, `list_roots` |
| **admin** | `-mode admin` or `-enable-admin-tools` | agent tools + `ingest_*`, `delete_*`, `list_documents`, `vomit` |

Administrative schemas are **not registered** in agent mode. Call-time permission flags (`-readonly`, `-allow-ingest`, …) still apply when admin tools are present.

## Workspace defaults

| Flag | Purpose |
|------|---------|
| `-workspace DIR` | Load `DIR/.implcache.yaml` for default project / related roots |
| `-project-root NAME` | Default `projectRoot` when the agent omits it |

## HTTP

| Behavior | Default |
|----------|---------|
| Bind | Loopback; bare `:port` / `0.0.0.0` rewritten to `127.0.0.1` |
| Non-loopback | Refused unless `-allow-remote-http` (no built-in auth) |
| Mutations over HTTP | Off unless `-enable-http-mutations` |
| Auth | None built-in |

## Version

```bash
./implcache-mcp -version
go build -ldflags "-X main.version=$(git describe --tags --always)" -o implcache-mcp .
```

Current maturity line: **0.2.x** (pre-1.0; MCP contracts may still evolve).

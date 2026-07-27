# Remote ImplCache (LAN / Jetson)

Run ImplCache on a always-on host (for example a **Jetson Orin NX**), then point Cursor on your workstation at the remote MCP endpoint. The knowledge DB, ingest, Librarian UI, and analytics stay on the server; the coding agent only needs network access to `/mcp`.

This matches the lab setup used with:

- Server: Jetson Orin NX at `http://172.16.82.121:8080`
- Cursor MCP entry: `implCacheRemote` → `http://172.16.82.121:8080/mcp`
- Same process also serves Librarian + REST at `/api/v1`

Substitute your host IP or hostname everywhere below.

---

## Architecture

```text
┌────────────────────────────┐         LAN          ┌─────────────────────────────┐
│  Cursor (Windows / macOS)  │ ───────────────────► │  Jetson Orin NX (linux/arm64)│
│  ~/.cursor/mcp.json        │   Streamable HTTP    │  implcache-mcp               │
│  "implCacheRemote": {      │   /mcp               │  -http 0.0.0.0:8080          │
│    "url": "http://…/mcp"   │                      │  -allow-remote-http          │
│  }                         │ ───────────────────► │  -enable-librarian …         │
└────────────────────────────┘   browser / REST     │  implcache.db + usage DB     │
                                                     └─────────────────────────────┘
```

| Path | Role |
|------|------|
| `/mcp` | MCP Streamable HTTP (Cursor / other MCP clients) |
| `/api/v1` | Librarian REST (ingest, search playground, analytics) |
| `/` | Librarian UI (when `-enable-librarian`) |

Local stdio MCP (`command` + `args`) and remote URL MCP can both be registered; use distinct names (e.g. `implcache` vs `implCacheRemote`).

---

## 1. Pack a Jetson binary

On a machine with Go (cross-compile; no Jetson toolchain required):

```bash
./scripts/pack-dist-linux.sh jetson
# -> dist-jetson-orin-nx/  (linux/arm64)
```

Copy that folder to the Jetson (scp/rsync). Layout:

```text
dist-jetson-orin-nx/
  implcache-mcp
  ingestcli
  implcache.db          # empty starter; replace or ingest on device
  run-librarian.sh      # loopback only
  run-librarian-lan.sh  # LAN bind (remote clients)
  docs/
```

---

## 2. Start the server on the Jetson (LAN)

Non-loopback binds are **refused** unless you pass `-allow-remote-http`. Bare `:8080` / `0.0.0.0` is rewritten to loopback without that flag.

From the package directory:

```bash
chmod +x implcache-mcp ingestcli run-librarian-lan.sh
./run-librarian-lan.sh
```

Equivalent explicit command:

```bash
./implcache-mcp -db ./implcache.db \
  -http 0.0.0.0:8080 \
  -allow-remote-http \
  -enable-librarian \
  -enable-http-mutations \
  -mode admin
```

Optional hardening on a shared LAN (recommended):

```bash
./implcache-mcp -db ./implcache.db \
  -http 0.0.0.0:8080 \
  -allow-remote-http \
  -enable-librarian \
  -enable-http-mutations \
  -mode admin \
  -librarian-token "$ADMIN_TOKEN" \
  -librarian-viewer-token "$VIEWER_TOKEN"
```

Notes:

- Librarian Bearer tokens protect **`/api/v1`**, not `/mcp`. MCP over HTTP is still reachable by anyone who can hit the port.
- For a trusted lab VLAN this is often acceptable; for broader networks put TLS + auth at a reverse proxy and/or run `-mode agent` without `-enable-http-mutations` for a read-only remote MCP.
- Ensure the Jetson firewall allows TCP `8080` from your workstation subnet.

Confirm from the workstation:

```powershell
Invoke-RestMethod http://172.16.82.121:8080/api/v1/status
# Browser: http://172.16.82.121:8080/
```

---

## 3. Cursor `mcp.json` (client)

Edit the **client** machine’s Cursor MCP config (user-level: `~/.cursor/mcp.json` on Windows/macOS/Linux).

Example with both a local stdio server and the Jetson remote:

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/GitHub/implcache-mcp/implcache-mcp.exe",
      "args": [
        "-db", "D:/GitHub/implcache-mcp/implcache.db",
        "-mode", "agent"
      ]
    },
    "implCacheRemote": {
      "url": "http://172.16.82.121:8080/mcp"
    }
  }
}
```

| Field | Meaning |
|-------|---------|
| `"url"` | MCP Streamable HTTP endpoint (must end with `/mcp`) |
| Server name | Arbitrary label shown in Cursor (`implCacheRemote` is the lab convention) |

Reload MCP servers in Cursor after saving. You should see tools from `implCacheRemote` (`list_roots`, `get_implementation_context`, …) against the Jetson DB.

There is no `command`/`args` for the remote entry — Cursor opens the HTTP transport directly.

---

## 4. Ingest on the Jetson

Prefer ingest **on the server** (local disk / git clone on the Jetson):

```bash
./ingestcli -db ./implcache.db -mode project -root my_app -path /path/to/src
./ingestcli -db ./implcache.db -mode repo-ingest -name sdk -root sdk-main \
  -url https://github.com/org/sdk.git -ref main -acq managed_clone
```

Or use Librarian in the browser / REST (`-enable-http-mutations`). Remote Cursor agents in **agent** mode only retrieve; they do not need write access if you administer corpora on the device.

---

## 5. Day-to-day use

1. Leave `implcache-mcp` running on the Jetson (systemd/tmux/`run-librarian-lan.sh`).
2. Work in Cursor with `implCacheRemote` enabled.
3. Prefer explicit `preferredRoots` / `projectRoot` so root isolation stays correct across a multi-corpus DB.
4. Inspect Analytics on the Jetson UI (`/analytics`) — `latencyMs` is server-side; client RTT includes LAN.

Example lab battery (workstation → Jetson REST, same DB as MCP):

```powershell
pwsh ./_Scratch/remote-battery/run-battery.ps1 -Base http://172.16.82.121:8080/api/v1
```

(Scratch helper; not shipped in `dist/`.)

---

## 6. Checklist / troubleshooting

| Symptom | Likely cause |
|---------|----------------|
| Cursor cannot connect | Server not running; wrong IP; firewall; URL missing `/mcp` |
| Bind refused / only localhost works | Missing `-allow-remote-http` |
| UI open but MCP tools missing | Using UI URL instead of `…/mcp` in `mcp.json` |
| Mutations fail over REST | Missing `-enable-http-mutations` or Bearer token mismatch |
| `needsChoice` / HTTP 409 | Query spans multiple roots; set `preferredRoots` |
| Stale tools after upgrade | Restart Jetson process; reload MCP in Cursor |

---

## Related docs

- [CONFIGURATION.md](CONFIGURATION.md) — flags and local stdio MCP JSON  
- [USERS_MANUAL.md](USERS_MANUAL.md) — operator walkthrough  
- [MODES.md](MODES.md) — agent vs admin + HTTP safety  
- [API_V1.md](API_V1.md) — REST surface  
- [OPERATIONS.md](OPERATIONS.md) — security summary  
- Pack script: [`scripts/pack-dist-linux.sh`](../scripts/pack-dist-linux.sh) (`jetson` → `dist-jetson-orin-nx/`)

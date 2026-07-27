# Installation

This guide assumes you received an ImplCache **release package** (binaries + this `docs/` folder). You do not need a compiler or the project source tree.

---

## 1. Choose an install directory

Pick a stable folder and keep binaries, database, and docs together. Examples:

```text
Windows:  D:\Tools\ImplCache\
macOS:    /Users/you/Tools/ImplCache/
Linux:    /home/you/tools/implcache/
```

Suggested layout (this release package already includes the binaries and docs):

```text
ImplCache/
  implcache-mcp.exe   (or implcache-mcp)
  ingestcli.exe        (or ingestcli)
  docs/
  LICENSE
  NOTICE
  VERSION
  README.md
  implcache.db         (empty starter schema; ingest your corpora into it)
```

Use **absolute paths** everywhere you configure Cursor or scripts. Relative paths break when the client’s working directory changes.

---

## 2. Place the binaries

If you unzipped a full release package, the binaries are already next to this `docs/` folder — skip to [§3](#3-create-the-knowledge-database).

Otherwise, copy `implcache-mcp` and `ingestcli` into the install directory.

| Platform | Server binary | Ingest CLI |
|----------|---------------|------------|
| Windows | `implcache-mcp.exe` | `ingestcli.exe` |
| macOS / Linux | `implcache-mcp` | `ingestcli` |

On macOS/Linux, make them executable if needed:

```bash
chmod +x implcache-mcp ingestcli
```

Check the reported version:

```bash
./implcache-mcp -version
```

---

## 3. Knowledge database

This release package includes a **sanitized empty** `implcache.db` (schema only — no vendor or project corpora). Point the server at it:

```bash
./implcache-mcp -db ./implcache.db -mode agent
```

If the file is missing, the server creates a new empty database at `-db` on first open.

Prefer keeping the database next to the binaries, or in a dedicated data directory you back up. After you ingest your own corpora, back up that DB — it is not interchangeable with anyone else’s.

**Important:** ImplCache is pre-1.0. The database schema is versioned. If a future release refuses an old database, delete `implcache.db` (and `implcache.db-wal` / `implcache.db-shm` if present) and re-ingest your corpora. There is no automatic migration.

---

## 4. Load some knowledge

Before agents are useful, ingest at least one documentation or source root. Prefer the offline CLI for large trees:

```bash
./ingestcli -db ./implcache.db -mode markdown -root my-sdk-docs -path "C:/path/to/docs"
./ingestcli -db ./implcache.db -mode project -root my-app -path "D:/work/my-app"
```

Details: [INGEST.md](INGEST.md).

---

## 5. Connect Cursor (or another MCP client)

Add an MCP server entry that runs the binary with absolute paths. Example Cursor config:

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/Tools/ImplCache/implcache-mcp.exe",
      "args": [
        "-db", "D:/Tools/ImplCache/implcache.db",
        "-mode", "agent"
      ]
    }
  }
}
```

Reload the MCP server in the client after you replace binaries or change args.

For a **remote** ImplCache (e.g. Jetson on the LAN), use a URL entry instead of `command`:

```json
"implCacheRemote": {
  "url": "http://172.16.82.121:8080/mcp"
}
```

Server must run with `-allow-remote-http` and a non-loopback bind. See [REMOTE.md](REMOTE.md) and [CONFIGURATION.md](CONFIGURATION.md).

---

## 6. Smoke test

1. In the MCP client, confirm tools such as `list_roots` and `get_implementation_context` appear.  
2. Call `list_roots` — you should see the roots you ingested.  
3. Call `get_implementation_context` with a real task and `preferredRoots`.  

If tools are missing or stale, reload MCP and verify the `command` path points at the binary you just installed.

---

## Optional: Librarian UI

For a browser console (sources, jobs, search playground):

```bash
./implcache-mcp -db ./implcache.db -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
```

Open `http://127.0.0.1:8080/`. See [LIBRARIAN.md](LIBRARIAN.md).

---

## Upgrading

1. Stop any running `implcache-mcp` / MCP session.  
2. Replace the binaries with the new release.  
3. Keep or replace `docs/` as you prefer.  
4. Start the server; if the database schema is incompatible, delete the DB and re-ingest.  
5. Reload MCP in your client.  

---

## Uninstall

1. Remove the MCP entry from your client config.  
2. Delete the install directory (binaries, docs, and database).  
3. Remove any leftover `-output-root` folder (default `./vomit-output`) and upload directories if you used them.

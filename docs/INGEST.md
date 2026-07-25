# Ingest

Ingest loads files into SQLite as `documents` + `chunks` (+ `symbols` when extractable). Unchanged content is skipped via content hash.

## Modes

### Markdown / HTML (`ingest_markdown` / CLI `-mode markdown`)

- Accepts `.md` and `.html` (HTML converted to Markdown)
- Directory walk optional (`recursive`)
- Chunks by headings / size heuristics
- Assigns `source_type=markdown`
- URI: `project://{rootName}/{rel}`

### Project source (`ingest_project` / CLI `-mode project`)

- Walks a source tree for text-like files
- Skips common junk dirs, binaries, and **symlinks**
- Assigns `source_type=source`
- Runs pragmatic symbol extraction (Go, C/C++/C#, Python, JavaScript/TypeScript, Java)
- Infers language and a best-effort authority

### Delete by prefix (CLI `-mode delete-prefix` / tool `delete_by_uri_prefix`)

Removes all documents whose URI starts with a given prefix (cascades chunks/symbols).

## CLI (`cmd/ingestcli`)

```bash
go build -o ingestcli ./cmd/ingestcli

# Documentation tree
./ingestcli -db ./implcache.db -mode markdown -root example-control-app -path "C:/path/to/help"

# Application sources
./ingestcli -db ./implcache.db -mode project -root my_app -path "D:/work/my_app"

# Remove legacy file:// URIs
./ingestcli -db ./implcache.db -mode delete-prefix -prefix "file:///"
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-db` | `./implcache.db` | Database path |
| `-mode` | `markdown` | `markdown` \| `project` \| `delete-prefix` |
| `-path` | | File or directory (markdown/project) |
| `-root` | | `rootName` (default: basename of path) |
| `-recursive` | `true` | Markdown directory recursion |
| `-prefix` | | URI prefix for delete-prefix |

## MCP ingest tools

MCP ingest/delete/`vomit` tools are registered only in **`-mode admin`** (or with `-enable-admin-tools`). Agent mode does not expose them.

When admin tools are registered, calls are still gated by `-allow-ingest` / `-readonly`.

Prefer the offline CLI for large corpora. Server-wide caps:

- `-max-ingest-files` (default 50000)
- `-max-document-bytes` (default 8 MiB)

## Choosing `rootName`

Use stable, descriptive ids:

| Example | Use |
|---------|-----|
| `example-control-app` | Control-app help corpus |
| `example-device-sdk` | Device SDK C docs |
| `example-plugin-sdk` | Plugin SDK C++ docs |
| `example-network-sdk` | Network SDK docs |
| `my_app` | Current project sources |

Root names feed cue-based inference (`store/roots.go`). Add aliases there when you introduce a new long-lived corpus.

## What gets stored

For each file:

1. Read + hash; skip if unchanged  
2. Convert (HTML→MD if needed)  
3. Chunk  
4. Upsert document metadata (authority, language, technology when inferred)  
5. Replace chunks + FTS  
6. Replace symbols (project/source ingest)

## Tips

- Ingest **project code** as its own root (`current_project` authority when inferred) so ranking prefers it.
- Keep vendor help in separate roots; never mix example-control-app and example-device-sdk into one root.
- Re-run ingest after vendor updates; hash skip makes repeats cheap.
- Large corpora are fine; responses stay small via context budgets.
- Symlinked trees are refused/skipped for safety.

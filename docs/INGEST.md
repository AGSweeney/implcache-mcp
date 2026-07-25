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

### Single URL (`ingest_url` / CLI `-mode url`)

- Fetches one HTTPS (or explicitly allowed HTTP) documentation page
- Blocks private/loopback/metadata addresses (SSRF controls)
- Cleans HTML with profile `generic` \| `sphinx` \| `doxygen`
- Assigns `source_type=web`, URI `project://{rootName}/{url-path}`

Site-wide mirroring uses admin tools `add_web_source` / `ingest_site` / `refresh_web_source` (not the CLI yet). Crawls stay within allowed URL prefixes, respect `robots.txt` Disallow (best-effort), and may seed URLs from same-host `sitemap.xml`. Sphinx/Doxygen profiles strip nav chrome, normalize titles, and may set `web_sources.detected_version` from page titles. `ingest_site` / `refresh_web_source` report an `opId` for in-process progress via Librarian `get_operation`.

### PDF Stage 1 (`inspect_pdf` / `ingest_pdf` / CLI `-mode pdf-*`)

- Local `.pdf` files only (no remote download in this stage)
- Pure-Go extractors (`pdfcpu` metadata, `ledongthuc/pdf` text)
- Text PDFs chunked with `start_page` / `end_page` citations; URI `pdf://{root}/{file}`
- Image-only / encrypted PDFs are classified; OCR is **not** implemented (`ocrMode=off`)

### Git repositories (`inspect_repo` / `ingest_repo` / CLI `-mode repo-*`)

- Admin-only; **not** web crawling (GitHub HTML/`*.git` URLs are rejected by `ingest_url` / site crawl)
- Acquisition: `snapshot`, `managed_clone`, or `local_checkout` (`HEAD` or `working_tree`)
- System `git` with hooks disabled; shallow clone default; optional sparse paths / partial clone filter
- Reuses local-tree ingest; URIs `git://{root}/{rel}`; every root pinned to **resolved commit SHA**
- Refresh updates changed files and deletes removed paths only after success; failed refresh keeps prior root
- Private remotes: use Git Credential Manager / SSH agent; optional `credentialReference` label (no secrets in DB)

## CLI (`cmd/ingestcli`)

```bash
go build -o ingestcli ./cmd/ingestcli

# Documentation tree
./ingestcli -db ./implcache.db -mode markdown -root example-control-app -path "C:/path/to/help"

# Application sources
./ingestcli -db ./implcache.db -mode project -root my_app -path "D:/work/my_app"

# One documentation URL
./ingestcli -db ./implcache.db -mode url -root esp-idf-docs -url "https://docs.example.com/en/latest/api.html" -profile sphinx

# PDF inspect / ingest / remove
./ingestcli -db ./implcache.db -mode pdf-inspect -path ./manual.pdf
./ingestcli -db ./implcache.db -mode pdf-ingest -root device-manuals -path ./manual.pdf
./ingestcli -db ./implcache.db -mode pdf-remove -uri "pdf://device-manuals/manual.pdf"

# Git repository
./ingestcli -db ./implcache.db -mode repo-inspect -url https://github.com/org/sdk.git -ref v5.5
./ingestcli -db ./implcache.db -mode repo-ingest -name sdk -root sdk-main -url https://github.com/org/sdk.git -ref main -acq managed_clone
./ingestcli -db ./implcache.db -mode repo-refresh -name sdk
./ingestcli -db ./implcache.db -mode repo-add -name local-app -path "D:/Projects/App" -acq local_checkout -working-tree
./ingestcli -db ./implcache.db -mode repo-list
./ingestcli -db ./implcache.db -mode repo-remove -name sdk -remove-index -remove-clone

# Remove legacy file:// URIs
./ingestcli -db ./implcache.db -mode delete-prefix -prefix "file:///"
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-db` | `./implcache.db` | Database path |
| `-mode` | `markdown` | includes `repo-inspect` \| `repo-add` \| `repo-ingest` \| `repo-refresh` \| `repo-list` \| `repo-remove` |
| `-path` | | File or directory (markdown/project/pdf/local repo) |
| `-root` | | `rootName` (default: basename of path) |
| `-recursive` | `true` | Markdown directory recursion |
| `-prefix` | | URI prefix for delete-prefix |
| `-url` | | URL for `url` / repo remote |
| `-uri` | | Document URI for `pdf-remove` |
| `-name` | | Repo source name |
| `-ref` | | Branch / tag / commit |
| `-acq` | | `snapshot` \| `managed_clone` \| `local_checkout` |
| `-sparse` | | Comma-separated sparse-checkout paths |
| `-working-tree` | `false` | Index dirty working tree for local checkout |
| `-profile` | `generic` | Web cleanup profile |
| `-allow-http` | `false` | Permit `http://` in `url` mode |
| `-page-start` / `-page-end` | | Optional PDF page range |

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

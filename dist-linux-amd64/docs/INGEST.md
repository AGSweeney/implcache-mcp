# Ingesting knowledge

Ingest loads files into the ImplCache database as documents, searchable chunks, and (when possible) symbols. Unchanged content is skipped via content hash.

Prefer **`ingestcli`** for large trees. Use admin MCP tools or the Librarian UI for interactive / web-crawl workflows.

---

## Choosing a root name

Use stable, descriptive ids:

| Example | Use |
|---------|-----|
| `my_app` | Your current project sources |
| `acme-sdk-docs` | Vendor SDK documentation |
| `device-manuals` | PDF manuals |
| `sdk-main` | Git-ingested SDK tree |

Keep unrelated product families in **separate** roots. Never mix two vendor products into one root.

---

## Offline CLI (`ingestcli`)

Run from your install directory (paths shown for illustration):

```bash
# Documentation tree (Markdown / HTML)
./ingestcli -db ./implcache.db -mode markdown -root acme-sdk-docs -path "C:/path/to/help"

# Application sources
./ingestcli -db ./implcache.db -mode project -root my_app -path "D:/work/my_app"

# One documentation URL
./ingestcli -db ./implcache.db -mode url -root web-docs \
  -url "https://docs.example.com/en/latest/api.html" -profile sphinx

# PDF inspect / ingest / remove
./ingestcli -db ./implcache.db -mode pdf-inspect -path ./manual.pdf
./ingestcli -db ./implcache.db -mode pdf-ingest -root device-manuals -path ./manual.pdf
./ingestcli -db ./implcache.db -mode pdf-remove -uri "pdf://device-manuals/manual.pdf"

# Git repository
./ingestcli -db ./implcache.db -mode repo-inspect -url https://github.com/org/sdk.git -ref v5.5
./ingestcli -db ./implcache.db -mode repo-ingest -name sdk -root sdk-main \
  -url https://github.com/org/sdk.git -ref main -acq managed_clone
./ingestcli -db ./implcache.db -mode repo-refresh -name sdk
./ingestcli -db ./implcache.db -mode repo-list
./ingestcli -db ./implcache.db -mode repo-remove -name sdk -remove-index -remove-clone

# Remove documents by URI prefix
./ingestcli -db ./implcache.db -mode delete-prefix -prefix "project://old-root/"
```

### Common CLI flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-db` | `./implcache.db` | Database path |
| `-mode` | `markdown` | See modes below |
| `-path` | | File or directory (markdown/project/pdf/local repo) |
| `-root` | | Root name (default: directory basename) |
| `-recursive` | `true` | Markdown directory recursion |
| `-prefix` | | URI prefix for `delete-prefix` |
| `-url` | | URL for `url` / repo remote |
| `-uri` | | Document URI for `pdf-remove` |
| `-name` | | Repo source name |
| `-ref` | | Branch / tag / commit |
| `-acq` | | `snapshot` \| `managed_clone` \| `local_checkout` |
| `-sparse` | | Comma-separated sparse-checkout paths |
| `-working-tree` | `false` | Index dirty working tree for local checkout |
| `-profile` | `generic` | Web cleanup: `generic` \| `sphinx` \| `doxygen` |
| `-allow-http` | `false` | Permit `http://` in `url` mode |
| `-page-start` / `-page-end` | | Optional PDF page range |

### Modes summary

| Mode | What it does |
|------|----------------|
| `markdown` | `.md` / `.html` → `project://` URIs |
| `project` | Source tree walk + symbol extraction |
| `url` | Single page fetch (SSRF-safe) |
| `pdf-inspect` / `pdf-ingest` / `pdf-remove` | Local PDF Stage 1 |
| `repo-*` | Git inspect / add / ingest / refresh / list / remove |
| `delete-prefix` | Delete all docs whose URI starts with a prefix |

**Site-wide web crawl** is not in `ingestcli` yet—use the Librarian UI or admin MCP tools (`add_web_source` / `ingest_site`).

---

## Pipelines at a glance

| Content | Result URI | Notes |
|---------|------------|-------|
| Markdown / HTML | `project://{root}/…` | HTML converted to Markdown |
| Project source | `project://{root}/…` | Skips junk dirs, binaries, symlinks |
| Web page / site | `project://{root}/…` | Crawl respects prefixes / robots (best-effort) |
| PDF | `pdf://{root}/{file}` | Local text PDFs; OCR not included |
| Git | `git://{root}/…` | Commit-pinned; needs system `git` |

Private Git remotes: use Git Credential Manager or SSH agent. Optional `credentialReference` is a label only—secrets are never stored in the database.

Managed clones are cached under `<db-dir>/.implcache/repos/`.

---

## Caps and behavior

Server-wide defaults (also apply when ingesting via MCP/Librarian):

- About **50 000** files per ingest operation (`-max-ingest-files`)  
- About **8 MiB** per file (`-max-document-bytes`)  
- Unchanged content skipped by hash  
- Symbols extracted for Go, C/C++/C#, Python, JS/TS, Java (heuristic)  

---

## Admin MCP ingest

When the server runs with `-mode admin` (and ingest is allowed), agents or admin clients can call tools such as `ingest_markdown`, `ingest_project`, `ingest_url`, `ingest_pdf`, `ingest_repo`, plus web/Git refresh and remove tools.

See [TOOLS.md](TOOLS.md). For a GUI, use [LIBRARIAN.md](LIBRARIAN.md).

---

## Tips

- Ingest **project code** as its own root so ranking prefers it.  
- Re-run ingest after vendor updates; hash skip makes repeats cheap.  
- Large corpora are fine; agent responses stay small via context budgets.  
- Prefer one writer at a time when loading data.  

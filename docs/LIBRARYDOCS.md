# LibraryDocs ingestion — deliverable notes

ImplCache LibraryDocs-aware ingest (phases 1–2 + light phase 3). **No schema changes**: `store/schema.sql` / `user_version` unchanged; no new tables or columns; no DB recreation required.

## Files / packages

| Area | Location |
|------|----------|
| Parsers + PackageMeta | `librarydocs/` |
| Git ingest wiring | `gitrepo/ingest.go` |
| Project ingest wiring | `ingest/project.go` |
| Manifest key | `manifest/manifest.go` → `libraryDocsHandling` |
| HTTP git ingest | `httpapi/git.go` |
| Librarian detail / Search Lab | `librarian/inventory.go`, `librarian/preview.go` |
| Context citations | `implctx/implctx.go` |
| Search hit enrichment | `store.SearchHit.LibraryDocs`, `librarydocs.EnrichHits` |
| Fixtures | `testdata/librarydocs/` |
| Operator docs | [INGEST.md](INGEST.md#librarydocs-aware-ingest), [USERS_MANUAL.md](USERS_MANUAL.md) pointer |
| Authoring skill | [CREATE_LIBRARYDOCS.md](CREATE_LIBRARYDOCS.md) · [`tools/create-librarydocs/`](../tools/create-librarydocs/) · zip [`tools/create-librarydocs.zip`](../tools/create-librarydocs.zip) |

## Schema changes

**None.**

## Metadata keys (synthetic document)

URI: `{git\|project}://{root}/.implcache/librarydocs-meta.json`  
Marker: `technology = librarydocs-meta`, title `LibraryDocs package metadata`.

Canonical body schema: `implcache.librarydocs` / `schema_version: 1`, including:

- `librarydocs`, `librarydocs_package_state`, `handling`, `standard_version`
- `validation`, `components`, `index`, `artifacts`
- `documents[relPath]` → `content_class`, `component_id`, `evidence_level`, `source_paths`, …
- `summary` (counts for UI/API)

## Parsers

Detect → classify paths → frontmatter → COMPONENT_INVENTORY / INDEX / artifacts README tables → VALIDATION.md (`result: pass|fail`). Path security rejects `..` and absolute/`URI` `source_paths`.

## UI / API fields

- Git ingest request/response: `libraryDocsHandling`, report `libraryDocs` summary
- Librarian source detail: `Detail["libraryDocs"]` from meta (or last summary)
- Search Lab: LibraryDocs fields on hits; filters `libraryDocsOnly` / `excludeLibraryDocs` / level / status

## Import setting

`libraryDocsHandling`: `auto` (default) | `normal` | `exclude`  
API on repo add/refresh + `.implcache.yaml`. Reindex required after changes.

## Tests

- Unit: `librarydocs/*_test.go` (states, classify, paths, trust, ranking, persist)
- Integration: `ingest/librarydocs_test.go` (validated mqtt-client, exclude, reimport removes meta, ordinary repo unchanged)
- Fixtures: missing / unstructured / structured / validated / invalid_validation / malformed / mqtt-client

## Compatibility

Ordinary repos without `LibraryDocs/` behave as before (summary `detected=false`, no meta document). Invalid/malformed LibraryDocs never fail tree ingest.

## Limitations

- Enrichment is document-scoped via synthetic JSON, not relational joins
- Project ingest does not write `repo_files` rows (git ingest does); content class still lives in PackageMeta / hit enrichment
- Freshness / analytics / scripted validation are deferred (below)

## Deferred phases

- Dedicated `librarydocs_*` tables (can ingest the same JSON shape)
- Source-hash freshness / `potentially_stale`
- LibraryDocs analytics counters in the usage DB
- Deeper mixed-package builder using `source_paths`
- Executing VALIDATION scripts (**never**)

## Authoring skill

Packages are produced with the **create-librarydocs** Cursor skill (v2.1.0), shipped in this repo as [`tools/create-librarydocs/`](../tools/create-librarydocs/) and [`tools/create-librarydocs.zip`](../tools/create-librarydocs.zip). Install globally once; do not embed the skill in every consumer repo. Full install, workflow, and standards index: [CREATE_LIBRARYDOCS.md](CREATE_LIBRARYDOCS.md).

## mqtt-client unified-package example

Fixture tree: [`testdata/librarydocs/mqtt-client/`](../testdata/librarydocs/mqtt-client/)

- Source: `src/mqtt_client.go` (`Connect` / `Publish`)
- Curated docs: `LibraryDocs/libraries/mqtt-client/README.md` (verified, E1)
- INDEX + COMPONENT_INVENTORY + VALIDATION (`pass`) + artifact pattern
- Single ImplCache root `mqtt-client`; meta URI `project://mqtt-client/.implcache/librarydocs-meta.json`
- Auto ingest maps the library README to `curated_internal_recipe` when package state is `validated`

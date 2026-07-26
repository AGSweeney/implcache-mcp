# Component Inventory — Template

**Mandatory deliverable:** `LibraryDocs/project/COMPONENT_INVENTORY.md`

Create this file in **Phase 1** (discover) and keep it updated through validation. It is the authoritative component list — README indexes are derived from it.

Copy this template into each new project and fill every row before writing component docs.

---

```markdown
---
title: Component inventory
level: project
status: draft | verified
inventory_version: 1
repo_root: <path/to/firmware/or/app>
last_updated: YYYY-MM-DD
---

# Component Inventory

## Summary

| Metric | Count |
|--------|-------|
| Libraries | |
| Project subsystems | |
| Platform modules | |
| Artifacts required | |
| Verified (E1/E2) | |
| Inferred/draft | |

## Inventory table

| ID | Name | Level | Folder | Source paths | Reuse | Owner task | Socket/storage | Artifact IDs | Doc status | Evidence |
|----|------|-------|--------|--------------|-------|------------|----------------|--------------|------------|----------|
| L01 | ExampleClient | library | libraries/example-client | src/client.h, src/client.c | high | AppTask | TCP fd | A-L01-if, A-L01-pat | verified | E1 |
| P01 | GatewayTask | project | project/subsystems/gateway | src/gateway.c | app-only | GatewayTask | — | A-P01-pat | verified | E1 |
| PL01 | Build/toolchain | platform | platform/build | makefile | n/a | — | flash / config store | A-PL01-bld | verified | E1 |

### Column definitions

| Column | Required | Description |
|--------|----------|-------------|
| **ID** | Yes | Stable ID: `L`=library, `P`=project subsystem, `PL`=platform, `D`=diagnostic |
| **Name** | Yes | Human name (ExampleClient, PollTask) |
| **Level** | Yes | `library` \| `project` \| `platform` |
| **Folder** | Yes | Target doc path under LibraryDocs/ |
| **Source paths** | Yes | Repo-relative paths; comma-separated |
| **Reuse** | Libraries only | `high` \| `medium` \| `low` \| `app-only` |
| **Owner task** | If applicable | RTOS task, thread, or `HTTP pool` |
| **Socket/storage** | If applicable | What I/O or persistence it owns |
| **Artifact IDs** | Yes | Cross-ref to artifacts/README.md IDs |
| **Doc status** | Yes | `missing` \| `draft` \| `inferred` \| `verified` |
| **Evidence** | Yes | Highest level present: E1–E4 |

## Excluded (grouped under parent)

| Symbol/file | Parent ID | Reason |
|-------------|-----------|--------|
| helper_fn() | L01 | Single function, no public API |

## Coupling register

| From ID | To ID | Coupling type | Notes |
|---------|-------|---------------|-------|
| P01 | L01 | calls API | Must run on same task |
| P01 | P02 | global config | gSettings blob |

Coupling types: `calls API`, `global state`, `op queue`, `config blob`, `include-only`.

## Retrieval keywords

Comma-separated terms for ImplCache/search (one line per component):

| ID | keywords |
|----|----------|
| L01 | client, tls, session, reconnect |
| P01 | gateway task, op queue, marshalling |
```

---

## Agent rules

1. **No doc without inventory row** — every folder in `libraries/` and `project/subsystems/` must have an ID here first.
2. **Artifact IDs** — assign before writing artifacts; use prefix `A-{ID}-` (e.g. `A-L01-if` → `artifacts/interfaces/example_client.hpp`).
3. **Doc status sync** — when README is finished, update row to match evidence level.
4. **Excluded list** — prevents spurious component folders for helpers.
5. **Coupling register** — required for honest project-level docs; drives `subsystems/README.md`.

## Validation

Inventory passes when:

- [ ] Every ID has a corresponding doc file or is in Excluded
- [ ] Every `libraries/` and `subsystems/` folder has an ID
- [ ] All `source_paths` exist on disk
- [ ] All `Artifact IDs` appear in `artifacts/README.md`
- [ ] No orphan docs (doc exists but no inventory row)

Run: `python .cursor/skills/create-librarydocs/scripts/validate_librarydocs.py`

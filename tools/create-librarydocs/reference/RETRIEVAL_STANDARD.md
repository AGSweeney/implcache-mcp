# Retrieval Quality Standard — LibraryDocs

LibraryDocs must be optimized for **agent and ImplCache retrieval**, not only human reading.

## Frontmatter schema (required on every doc)

```yaml
---
title: Human-readable title
component: kebab-case-id          # matches COMPONENT_INVENTORY ID suffix
level: library | project | platform
reuse: high | medium | low        # libraries only; omit for platform
platforms:
  - Target-A                      # tested or targeted (infer names from repo)
topics:                           # 3–8 retrieval keywords
  - connect
  - session
  - tls
source_paths:
  - path/to/file.cpp
status: verified | inferred | draft
retrieval:
  questions:                      # natural-language queries this doc answers
    - How do I initialize the client?
    - Which task or thread owns the session?
  related:
    - libraries/example-client/README.md
    - project/architecture/system-overview.md
---
```

## INDEX.md requirements

Each row **must** include:

| Column | Purpose |
|--------|---------|
| Path | File path |
| ID | Inventory ID (L01, P01, …) |
| Level | library / project / platform |
| Component | kebab name |
| Purpose | One sentence |
| Topics | Comma-separated keywords |
| Status | verified / inferred / draft |

Agents use INDEX as the **routing table** — incomplete INDEX = poor retrieval.

## Document shape for retrieval

1. **First H1** — component name only (matches `title`).
2. **First paragraph** — one-sentence purpose + reuse classification.
3. **H2 sections** — use predictable names from the 20-section library template (do not invent synonyms like "How it works" instead of "Runtime lifecycle").
4. **Tables over prose** — limits, error codes, routes, config fields.
5. **Link up, link sideways** — every doc links to parent index and 2+ related docs.

## Question coverage

For each library, at least **3** `retrieval.questions` must be answerable from that doc alone:

- "How do I initialize X?"
- "What task/thread owns X?"
- "What breaks when Y fails?"

Project architecture docs must answer:

- "What is the startup order?"
- "What are the data flows between subsystems?"
- "Where is configuration persisted?"

## Duplication control

| If the question is about… | Canonical doc |
|---------------------------|---------------|
| Protocol wire format | owning library under `libraries/` |
| App-specific mapping / blob layout | owning subsystem under `project/subsystems/` |
| SDK / build quirk | owning doc under `platform/` (+ pattern artifact if needed) |
| Bench / flash / deploy procedure | `project/recipes` or platform deploy doc |

Non-canonical docs: one-line answer + link.

## ImplCache ingestion hints

When preparing for ImplCache or similar:

- Prefer **self-contained sections** (avoid "see above").
- Keep sections under ~80 lines; split into `api-reference.md` if needed.
- Include **symbol names** in prose (functions, config keys, REST paths).
- `artifacts/` files are separate chunks — link by ID from inventory.

## Retrieval quality checklist

- [ ] Every doc has `topics` (≥3) and `component`
- [ ] Every library has `retrieval.questions` (≥3)
- [ ] INDEX has ID column synced with COMPONENT_INVENTORY
- [ ] No two docs use conflicting terms for the same concept (alias table in project README)
- [ ] Architecture doc names every subsystem ID from inventory

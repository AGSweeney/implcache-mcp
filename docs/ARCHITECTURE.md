# Architecture

**Tagline:** Local implementation context for coding agents.

## Purpose

ImplCache MCP is a **local implementation-context compiler and retrieval server**. It is not a generic document search engine.

Primary goal: reduce the tokens, retrieval time, web searches, file reads, compile failures, hallucinated APIs, and correction cycles required for a coding agent to produce working software.

## Design principles

1. **Smallest sufficient package** — prefer a budgeted answer over many hits.
2. **Exact grounding** — cite URIs, titles, sections, and authority; do not invent APIs.
3. **Root isolation** — do not mix unrelated product corpora without an explicit choice.
4. **Authority-aware ranking** — project code and curated recipes outrank generated summaries.
5. **Staged depth** — escalate from package → symbols → examples → surrounding source → full document.
6. **Local-first, honest about staleness** — keep knowledge on disk; surface when web check may still be needed.
7. **No CGO** — SQLite via `modernc.org/sqlite` for portable builds.
8. **FTS-first search** — authority/root ranking by default; optional sparse term-vector semantic uses indexed term postings with persisted DF for query-time IDF (`-enable-semantic`); neural embeddings and classic per-chunk TF-IDF remain deferred.

**Schema** is `PRAGMA user_version = 11` (canonical schema in `store/schema.sql`, created directly on open; incompatible databases are refused — delete and re-ingest). Includes web, PDF, and Git `repo_sources`/`repo_files` tables. **Symbol languages:** Go, C/C++/C#, Python, JS/TS, Java. **Freshness** is independent of authority. **`contextFingerprint`** hashes the final trimmed response (not pre-trim candidates).

Known limitations: [OPERATIONS.md](OPERATIONS.md#limitations-and-risks).

## Agent workflow

```text
Task identified (technology, language, project)
  → select knowledge roots / root group / preferredRoots
  → get_implementation_context (budgeted package)
  → agent writes code
  → staged follow-ups only if needed:
        find_symbol → search_knowledge → get_document
```

## System layers

```text
┌─────────────────────────────────────────────────────────────┐
│  MCP tools (tools/)                                         │
│  agent: get_implementation_context · find_symbol · search   │
│  admin: ingest/crawl · Librarian inventory · vomit          │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│  Librarian (librarian/) — admin inventory / health / preview│
│  Acquisition: web/ · pdf/ · gitrepo/                        │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│  Implementation context assembly (implctx/)                 │
│  recipes → symbols → FTS hits → examples → budget clip      │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│  Store (store/)                                             │
│  documents · chunks · FTS5 · symbols · recipes · aliases    │
│  web/pdf/repo sources · authority ranking · root inference  │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│  Shared ingest (ingest/)                                    │
│  Markdown / HTML→MD / project walk · chunk · symbols · hash │
└─────────────────────────────────────────────────────────────┘
```

## Data flow: implementation context

```text
Request{task, language, technology, preferredRoots|rootGroup|projectRoot, maxContextTokens}
  → resolve roots (explicit / group / infer / needsChoice)
  → search knowledge_entries (recipes; human_reviewed preferred)
  → find symbols from identifier-like task tokens
  → FTS search with composite score (BM25 + authority + example bias)
  → diversify + clip excerpts under ContextBudget
  → Response{summary, apis, symbols, sequence, examples, citations, coverage, …}
```

## Source authority (highest first)

Used for ranking and presentation style:

| Rank | Authority value | Typical content |
|------|-----------------|-----------------|
| 1 | `current_project` | The app the agent is editing |
| 2 | `related_internal_project` | Sibling / shared internal code |
| 3 | `curated_internal_recipe` | Human-reviewed `knowledge_entries` |
| 4 | `official_example` | Vendor samples |
| 5 | `official_documentation` | Vendor help / API docs |
| 6 | `generated_summary` | Auto recipes from `vomit` (`saveRecipe`) |
| 7 | `third_party_reference` / `unknown` | Everything else |

Generated recipes keep source lineage and must not outrank human-reviewed recipes.

## Context budget

Defaults (`store.DefaultContextBudget`):

| Cap | Default |
|-----|---------|
| Primary results | 5 |
| Examples | 2 |
| Excerpt chars | 600 |
| Total chars | ~10000 |
| Token estimate | 2500 |
| Chunks per document | 2 |

Token estimate in API responses is **labeled as estimate** (`utf8_runes / 4`).

## Root resolution

Search and context tools scope by `root_name`:

1. Explicit `rootName` / `preferredRoots` / `projectRoot` / `rootGroup`
2. Else cue matching against known aliases (e.g. `control-app`, `RegisterHandler`, `plugin-sdk`)
3. If multiple product families match → `needsChoice` + `availableRoots` (agent must ask the user)

Roots are never silently merged across conflicting product families.

## Recipe compiler (`vomit`)

`vomit` searches a resolved root, extracts APIs/includes/sequences from cited bodies, and emits Markdown. It always returns the body to the agent. Optionally:

- writes under `-output-root` (path-jailed)
- `saveRecipe` → `knowledge_entries` with `review_status=generated` and `knowledge_entry_sources` lineage

## Package map

| Package | Responsibility |
|---------|----------------|
| `main` | Flags, DB open, MCP stdio/HTTP, loopback rewrite, shutdown |
| `tools` | Tool schemas, permission gates, root-need error shaping |
| `librarian` | Unified source inventory, health, preview, search playground, in-process op progress |
| `implctx` | Budgeted implementation package assembly |
| `store` | Schema bootstrap, CRUD, FTS, symbols, recipes, roots, ranking, source tables |
| `ingest` | Walk, convert, chunk, hash skip, symbol extract, authority infer |
| `web` | URL fetch, HTML cleanup profiles, site crawl / refresh |
| `pdf` | Local PDF inspect / Stage 1 text ingest |
| `gitrepo` | Git acquire / sparse / classify / ingest / refresh |
| `vomit` | Playbook/recipe compilation |
| `internal/netsafe` | SSRF host/prefix guards for web fetch |
| `internal/safePath` | Confine output paths under a root |
| `cmd/ingestcli` | Offline ingest / delete-prefix / url / pdf / repo |
| `cmd/sourcevalidate` | Controlled real-source ingest reports |
| `cmd/evaltasks` | Small task-based retrieval harness |
| `cmd/semscale` | Offline semantic scale harness |

## Non-goals

- Chatbot / open-ended Q&A  
- Dumping entire documentation trees into context  
- Replacing official vendor docs when freshness is unknown  
- Vector embeddings unless SQLite ranking proves insufficient  
- Invented performance claims or fabricated eval numbers  

## Related docs

- [TOOLS.md](TOOLS.md) — tool contracts  
- [DATA_MODEL.md](DATA_MODEL.md) — schema and URIs  
- [AGENT_GUIDE.md](AGENT_GUIDE.md) — usage patterns  
- [OPERATIONS.md](OPERATIONS.md) — run, harden, evaluate  

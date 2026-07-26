# Evidence Standard — LibraryDocs Knowledge Extraction

Every factual claim in LibraryDocs must be traceable. This standard applies to all projects.

## Evidence levels

| Level | Label | When to use | Frontmatter `status` |
|-------|-------|-------------|----------------------|
| **E1** | Verified | Read directly from source file cited; line/function named | `verified` |
| **E2** | Bench-verified | Observed on hardware/CI with date and target recorded | `verified` + note in evidence block |
| **E3** | Inferred | Logical conclusion from partial reading; not directly confirmed | `inferred` |
| **E4** | Unknown | Not determined; must appear in `OPEN_QUESTIONS.md` | `draft` |

**Rule:** Never use `status: verified` for E3 or E4.

## Required evidence block (every component README)

End each library and subsystem README with:

```markdown
## Source evidence

| Claim | Evidence | Level |
|-------|----------|-------|
| Single-owner client contract | `client.hpp` comment L8–16 | E1 |
| Poll interval 50–60000 ms | `app_config.hpp` `m_pollIntervalMs` limits | E1 |
| Target-A bench connect | Bench log 2026-07-26, host 172.16.82.55 | E2 |
```

Minimum **3 rows** per library; **2 rows** per subsystem doc.

## Citation format

**In prose:** reference path and symbol — e.g. `OpenSession()` in `src/session_client.cpp`.

**In frontmatter:**

```yaml
source_paths:
  - src/session_client.hpp
  - src/session_client.cpp
evidence:
  - file: src/session_client.hpp
    symbol: OpenSession
    lines: 61-62
    level: E1
```

**In artifacts:** first line must be:

```cpp
// EXCERPT — source: relative/path/from/repo/root
// EVIDENCE: E1 | symbol: SaveSettings | lines: 16-25
```

## Prohibited evidence

- Vendor stereotypes (“typically X projects…”) without a file citation
- API shapes copied from memory without reading the header
- Bench claims without target, date, or log reference
- Duplicated claims with no new citation (link to canonical doc instead)

## Canonical vs derived docs

| Doc type | Holds canonical detail | Others must |
|----------|------------------------|-------------|
| Library README | Wire format, API, memory | Link here |
| Project architecture | Task model, data flow | Link to libraries for protocol |
| Platform | Build flags, SDK quirks | Link once |
| Recipe | Steps only | Link to subsystem + platform |

If two docs state the same fact, one must be marked **canonical** in the evidence table of the other: `See libraries/example-client/README.md (canonical)`.

## E2 bench evidence (retained artifacts)

E2 claims must reference a **file in the repository**, not prose-only dates.

Store under `LibraryDocs/artifacts/bench/`:

```text
artifacts/bench/
├── README.md
├── target-a-connect-2026-07-26.log
└── target-b-build-2026-07-26.md
```

Source evidence table example:

| Claim | Evidence | Level |
|-------|----------|-------|
| TLS connect on Target-B | `artifacts/bench/target-b-connect-2026-07-26.log` | E2 |

Any E4 item **must** appear in `project/OPEN_QUESTIONS.md`:

```markdown
| Question | Component | Blocker for | Date |
|----------|-----------|-------------|------|
| Max concurrent sessions on device X? | session-client | scalability doc | 2026-07-26 |
```

## Agent procedure

1. Read source before writing API or behavior sections.
2. Assign E1–E4 per major claim.
3. Downgrade to `inferred` or `draft` when uncertain.
4. Do not finish until every `verified` doc has ≥1 E1 or E2 row.

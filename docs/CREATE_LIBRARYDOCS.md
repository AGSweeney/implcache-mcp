# create-librarydocs skill

Cursor Agent skill (**v2.1.0**) that builds a conventional `LibraryDocs/` knowledge package in a **target repository**. ImplCache then **detects and ingests** that package ([LibraryDocs-aware ingest](INGEST.md#librarydocs-aware-ingest)).

| Item | Location |
|------|----------|
| Unpacked skill (tracked) | [`tools/create-librarydocs/`](../tools/create-librarydocs/) |
| Zip bundle | [`tools/create-librarydocs.zip`](../tools/create-librarydocs.zip) |
| Skill entry | [`SKILL.md`](../tools/create-librarydocs/SKILL.md) |
| Install steps | [`INSTALL.md`](../tools/create-librarydocs/INSTALL.md) |
| Ingest deliverable notes | [LIBRARYDOCS.md](LIBRARYDOCS.md) |

## What it is for

Use the skill when you want agents (or yourself) to extract reusable library / project / platform knowledge into `LibraryDocs/` with:

- Evidence levels **E1–E4**
- Component inventory + INDEX routing table
- Retrieval-oriented frontmatter (topics, `source_paths`, questions)
- Artifacts registry (usefulness U1–U6)
- Strict validation → `VALIDATION.md` with `result: pass`

It is **technology-agnostic**: stack is inferred from the target repo ([PLATFORM_INFERENCE](../tools/create-librarydocs/reference/PLATFORM_INFERENCE.md)).

**Do not** copy the full skill into every consumer project. Those repos should receive only `LibraryDocs/` (optional short `CREATE_LIBRARYDOCS.md` pointer). The skill itself installs globally under `%USERPROFILE%\.cursor\skills\create-librarydocs\`.

## Install (once)

From this repo (or after expanding the zip):

```powershell
$Src = "D:\path\to\implcache-mcp\tools\create-librarydocs"
$SkillRoot = "$env:USERPROFILE\.cursor\skills\create-librarydocs"
New-Item -ItemType Directory -Force "$SkillRoot\reference", "$SkillRoot\scripts" | Out-Null
Copy-Item -Recurse -Force "$Src\*" $SkillRoot
```

Confirm in **Cursor Settings → Rules, Skills, Subagents → Skills**.  
Do **not** install under `%USERPROFILE%\.cursor\skills-cursor\` (Cursor-managed built-ins).

Project-local pin (optional): `<consumer-repo>\.cursor\skills\create-librarydocs\` for team/CI version pinning.

## Generated layout (target repo)

```text
LibraryDocs/
├── README.md
├── INDEX.md
├── VALIDATION.md
├── artifacts/
├── libraries/
├── project/          # includes COMPONENT_INVENTORY.md
└── platform/
```

## Workflow (summary)

| Phase | Action |
|-------|--------|
| 0 | Infer stack from the repo |
| 1–2 | Discover components → write `project/COMPONENT_INVENTORY.md` |
| 3–4 | Classify library / project / platform → scaffold folders |
| 5–8 | Write docs, artifacts, INDEX |
| 9 | Run validator `--strict`; write `VALIDATION.md` |

Definition of done requires validator exit 0 with `--strict` and `VALIDATION.md` → `result: pass`, `standard_version: 2.1.0`. See [`SKILL.md`](../tools/create-librarydocs/SKILL.md).

### Validate (from target repo root)

```powershell
python "$env:USERPROFILE\.cursor\skills\create-librarydocs\scripts\validate_librarydocs.py" `
  --repo-root . `
  --strict
```

Default (non-strict) mode is for incremental work only — exit 0 without `--strict` does **not** mean conformance.

## Reference standards (in the skill)

| Standard | File |
|----------|------|
| Evidence E1–E4 | [`reference/EVIDENCE_STANDARD.md`](../tools/create-librarydocs/reference/EVIDENCE_STANDARD.md) |
| Component inventory | [`reference/COMPONENT_INVENTORY.template.md`](../tools/create-librarydocs/reference/COMPONENT_INVENTORY.template.md) |
| Retrieval / ImplCache | [`reference/RETRIEVAL_STANDARD.md`](../tools/create-librarydocs/reference/RETRIEVAL_STANDARD.md) |
| Artifact usefulness | [`reference/ARTIFACT_STANDARD.md`](../tools/create-librarydocs/reference/ARTIFACT_STANDARD.md) |
| Validation | [`reference/VALIDATION_STANDARD.md`](../tools/create-librarydocs/reference/VALIDATION_STANDARD.md) |
| Platform inference | [`reference/PLATFORM_INFERENCE.md`](../tools/create-librarydocs/reference/PLATFORM_INFERENCE.md) |

## Relationship to ImplCache

| Skill (authoring) | ImplCache (ingest / retrieval) |
|-------------------|--------------------------------|
| Builds `LibraryDocs/` in a source repo | Detects package during git/project ingest |
| Runs `validate_librarydocs.py` in the **authoring** workflow | Reads `VALIDATION.md` result; **never executes** validation scripts |
| Evidence / status / frontmatter conventions | Maps to `content_class`, authority, synthetic meta doc, ranking |

After a package validates, ingest the repo (or project root) as usual. Handling: `auto` (default) | `normal` | `exclude` — see [INGEST.md](INGEST.md#librarydocs-aware-ingest).

Fixture example: [`testdata/librarydocs/mqtt-client/`](../testdata/librarydocs/mqtt-client/).

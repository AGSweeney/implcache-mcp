---
name: create-librarydocs
version: 2.1.0
description: >-
  Knowledge extraction standard for LibraryDocs — evidence, component inventory,
  retrieval schema, artifacts, validation. Use when building LibraryDocs or
  ImplCache packages on any repository.
---

# LibraryDocs Knowledge Extraction Standard (v2.1.0)

Build **`LibraryDocs/`** in the **target repository** as a repeatable knowledge extraction package. Structure: **libraries / project / platform** + **artifacts**.

**Do not copy this skill into every project.** Generated repos normally contain only `LibraryDocs/`. This skill lives globally at `%USERPROFILE%\.cursor\skills\create-librarydocs\`.

This skill is **technology-agnostic**. Infer languages, platforms, SDKs, and runtime models from the target repository; do not assume a vendor or board family.

## Reference standards

| Standard | File |
|----------|------|
| Evidence E1–E4 | [reference/EVIDENCE_STANDARD.md](reference/EVIDENCE_STANDARD.md) |
| Component inventory | [reference/COMPONENT_INVENTORY.template.md](reference/COMPONENT_INVENTORY.template.md) |
| Retrieval / ImplCache | [reference/RETRIEVAL_STANDARD.md](reference/RETRIEVAL_STANDARD.md) |
| Artifact usefulness U1–U6 | [reference/ARTIFACT_STANDARD.md](reference/ARTIFACT_STANDARD.md) |
| Validation | [reference/VALIDATION_STANDARD.md](reference/VALIDATION_STANDARD.md) |
| Platform & tech inference | [reference/PLATFORM_INFERENCE.md](reference/PLATFORM_INFERENCE.md) |

---

## Validation (location-independent)

Run the **`validate_librarydocs.py` script bundled with this skill** against the **target repository root**. Do not hard-code project-local or global paths in instructions — resolve the script relative to the loaded skill.

**Required arguments:**

- `--repo-root <repository-root>` — directory containing `LibraryDocs/` (use `.` when cwd is the repo root)
- `--strict` — required for definition-of-done (warnings fail)

**Example (Windows, from repository root):**

```powershell
python "$env:USERPROFILE\.cursor\skills\create-librarydocs\scripts\validate_librarydocs.py" `
  --repo-root . `
  --strict
```

Record in `LibraryDocs/VALIDATION.md`:

```yaml
standard: create-librarydocs
standard_version: 2.1.0
mode: strict
result: pass
```

Default mode (no `--strict`) is for incremental work only. Exit 0 without `--strict` does **not** establish conformance.

---

## Generated repository layout

The target repo receives **only**:

```text
LibraryDocs/
├── README.md
├── INDEX.md
├── CREATE_LIBRARYDOCS.md    # brief human pointer to this skill (optional)
├── VALIDATION.md
├── artifacts/
├── libraries/
├── project/
└── platform/
```

Optional: `LibraryDocs/CREATE_LIBRARYDOCS.md` — short overview; must **not** duplicate this skill.

---

## Workflow

| Phase | Action |
|-------|--------|
| 0 | **Infer stack** — languages, build, runtime, targets, I/O, SDKs ([PLATFORM_INFERENCE.md](reference/PLATFORM_INFERENCE.md)) |
| 1 | Discover → draft inventory rows |
| 2 | Write `project/COMPONENT_INVENTORY.md` (IDs, coupling, artifact IDs) |
| 3 | Classify libraries vs subsystems vs platform (using inferred stack) |
| 4 | Scaffold folders |
| 5–8 | Write docs, artifacts, index |
| 9 | Run validator `--repo-root . --strict`; write VALIDATION.md |

See reference files for evidence, retrieval, artifacts, and how to choose platform docs from repository signals.

**Educated choices:** Prefer workspace MCP/SDK tools when available for builds and API facts. Cite repository files (E1) over training-data stereotypes about a vendor.

---

## Library README — 20 concerns

Address all 20 concerns; omit or mark **Not applicable** only when obviously irrelevant (see ARTIFACT_STANDARD for exemption path).

---

## Global vs project-local skill

| Install | Path | When |
|---------|------|------|
| **Global (default)** | `%USERPROFILE%\.cursor\skills\create-librarydocs\` | Normal Cursor use across repos |
| **Project copy** | `<repo>\.cursor\skills\create-librarydocs\` | Pin version, team share via Git, CI without user profile, custom extensions |

Do **not** install under `%USERPROFILE%\.cursor\skills-cursor\` (Cursor-managed built-ins).

For CI: pin `scripts/validate_librarydocs.py` (or a tagged skill release) in the repo; pass `--repo-root` explicitly.

---

## Definition of done

1. `COMPONENT_INVENTORY.md` complete  
2. All inventory IDs documented with evidence  
3. Artifacts registered and linked (or exempted)  
4. Validator: `--repo-root . --strict` → exit 0  
5. `VALIDATION.md` → `result: pass`, `standard_version: 2.1.0`

Do **not** commit or ingest to ImplCache unless the user asks.

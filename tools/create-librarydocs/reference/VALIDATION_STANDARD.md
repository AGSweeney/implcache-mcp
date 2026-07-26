# Validation Standard — LibraryDocs

## Modes

| Mode | Command | Use |
|------|---------|-----|
| **Default** | `python …/validate_librarydocs.py --repo-root .` | Incremental work; errors fail, warnings report |
| **Strict** | `python …/validate_librarydocs.py --repo-root . --strict` | **Required** for definition-of-done |

The script is bundled with the **create-librarydocs** skill. Pass **`--repo-root`** explicitly — the script location (global skill dir vs project `tools/`) does not imply the repository root.

**Example (Windows, from repository root):**

```powershell
python "$env:USERPROFILE\.cursor\skills\create-librarydocs\scripts\validate_librarydocs.py" `
  --repo-root . `
  --strict
```

For CI: pin `validate_librarydocs.py` in the repo (e.g. `tools/`) and invoke with `--repo-root .`.

**Strict mode:** errors **and** standards-related warnings fail (exit 1).

Exit code 0 in default mode with warnings does **not** establish conformance. Agents must run **`--strict`** for completion.

---

## Automated checks (implemented)

| ID | Check |
|----|-------|
| V-STR | `LibraryDocs/` structure: README, INDEX, VALIDATION, libraries/, project/, platform/, artifacts/ |
| V-INV | `COMPONENT_INVENTORY.md` parsed; inventory table present |
| V-INV2 | Every inventory folder has documentation; orphan folders warned |
| V-INV3 | Bidirectional ID ↔ folder mapping |
| V-SRC | `source_paths` exist (supports `*` globs under `repo_root`) |
| V-CPG | Coupling register includes every `P##` project subsystem |
| V-OQ | Inventory rows with E4 appear in `OPEN_QUESTIONS.md` |
| V-IDX | All inventory IDs appear in `INDEX.md` |
| V-ART1 | Inventory artifact IDs registered and files exist on disk |
| V-ART2 | Registry paths resolve to on-disk files |
| V-ART3 | Unregistered on-disk artifacts → warning (strict: fail) |
| V-ART4 | Code artifacts: EXCERPT + EVIDENCE headers, ≤120 lines |
| V-ART5 | Inventory-required artifacts linked from component docs |
| V-ART6 | Libraries without artifact IDs require `artifacts_required: false` + `artifact_exemption` |
| V-FM | All non-exempt `.md` files: frontmatter (`title`, `component`, `level`, `status`) |
| V-FM2 | Non–api-reference docs: `topics` ≥ 3 |
| V-FM3 | Library READMEs: `retrieval.questions` ≥ 3 |
| V-FM4 | `status: verified` → Source evidence section with E1 or E2 |
| V-LNK | All relative markdown links under `LibraryDocs/` resolve |
| V-VAL | Strict: `VALIDATION.md` exists with `result: pass` |

### Exempt from full frontmatter schema

- Root/meta: `INDEX.md`, `CREATE_LIBRARYDOCS.md`, `VALIDATION.md`, `COMPONENT_INVENTORY.md`, `OPEN_QUESTIONS.md`, …
- Index READMEs: `libraries/README.md`, `project/README.md`, `platform/README.md`, architecture/recipes/subsystems index READMEs
- `api-reference.md` — requires `title`, `component`, `level`, `status` only (canonical README holds topics/evidence)
- Recipes — require `level`, `topics`, `status`; **no** Source evidence requirement

---

## Manual checklist (when automation cannot decide)

- [ ] 20 library concerns addressed (sections or explicit N/A with reason)
- [ ] E2 bench claims point to `artifacts/bench/` files, not prose-only dates
- [ ] No false “reusable library” labels on coupled subsystems
- [ ] Canonical doc chosen for each cross-cutting fact
- [ ] Platform docs match inferred stack ([PLATFORM_INFERENCE.md](PLATFORM_INFERENCE.md)); no empty vendor stubs

---

## Validation report

Write `LibraryDocs/VALIDATION.md` after running strict mode:

```yaml
---
title: LibraryDocs validation report
standard: create-librarydocs
standard_version: 2.1.0
validated: YYYY-MM-DD
validator: agent | human
result: pass | fail
mode: strict
repo_root: .
---
```

---

## Definition of done

1. `python …/validate_librarydocs.py --repo-root . --strict` → exit 0  
2. `VALIDATION.md` → `result: pass`, `standard_version` recorded  
3. `COMPONENT_INVENTORY.md` complete and synced with INDEX  
4. All blocking manual items reviewed  

**Phase 1 note:** Default mode remains useful for incremental work. **Strict mode is the authoritative gate.**

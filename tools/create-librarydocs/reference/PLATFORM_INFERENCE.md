# Platform & Technology Inference — LibraryDocs Extraction

LibraryDocs is **technology-agnostic**. Do not assume a vendor, RTOS, language, or build system. Infer the stack from the repository, then choose platform docs and artifacts that match what you find.

## Phase 0 — Discover the stack (before writing docs)

Scan the target repository and record findings in working notes (or draft inventory rows). Prefer **observed signals** over guesses.

### Signals to collect

| Signal class | Where to look | Examples of evidence (illustrative, not exhaustive) |
|--------------|---------------|-----------------------------------------------------|
| **Language(s)** | extensions, lockfiles, build files | `.c`/`.cpp` + Makefile; `package.json`; `Cargo.toml`; `*.csproj` |
| **Build system** | root build files | Make, CMake, MSBuild, Gradle, npm/pnpm scripts, Bazel |
| **Runtime / OS** | includes, linker scripts, SDK roots | bare-metal RTOS headers, Linux userspace, browser, container base images |
| **Hardware / board** | PLATFORM flags, board pkgs, device trees | MCU family, SOM name, cloud-only (no hardware) |
| **Networking / I/O** | sockets, serial, fieldbus, HTTP servers | TCP/UDP, MQTT, REST, CAN, EtherNet/IP |
| **Persistence** | flash FS, DB, config blobs | SQLite, onboard flash FS, JSON/YAML config, registry |
| **Security / crypto** | TLS libs, auth middleware | OpenSSL, mbedTLS, JWT, mTLS |
| **Concurrency model** | tasks/threads/async | RTOS tasks, pthreads, asyncio, worker pools |
| **External SDKs** | env vars, include paths, vendor dirs | `*_ROOT`, vendored `third_party/`, MCP/SDK skills in workspace |

### Output of discovery

Write a short **stack summary** into `LibraryDocs/platform/README.md` (or the first platform doc):

```markdown
## Detected stack

| Layer | Inference | Evidence |
|-------|-----------|----------|
| Language | C++17 | `makefile` CXXFLAGS, `.cpp` sources |
| Build | GNU Make + vendor SDK | SDK include path, `PLATFORM=` / toolchain file |
| Runtime | Vendor RTOS or OS | RTOS/OS headers in entrypoint |
| Targets | Board A, Board B | supported-target list in build files |
```

Use E1 for paths you read; E3 when inferring from partial signals; E4 + OPEN_QUESTIONS when unclear.

---

## Infer platform document set

Create only the platform docs that the stack needs. Generic home: `LibraryDocs/platform/`.

| Concern | Create when… | Typical path |
|---------|--------------|--------------|
| Build / toolchain | Any non-trivial build | `platform/build/build-instructions.md` |
| Target matrix | Multiple boards, OS, or deploy targets | `platform/build/platform-requirements.md` |
| Memory / resource limits | Embedded, constrained heap, stack sizes | `platform/build/memory-configuration.md` |
| Vendor/SDK configuration | Persistent config objects, device settings APIs | `platform/<vendor-or-sdk>-configuration.md` |
| Portability | Multi-arch, multi-OS, or feature gates by target | `platform/build/portability-notes.md` |
| Deploy / flash / CI | Distinct load or release procedure | `platform/build/deploy.md` or project recipes |

**Do not** create empty stub docs for concerns that do not apply. Mark N/A in inventory or omit the row.

---

## Infer extractions from concurrency & I/O model

From discovery, decide what must appear in architecture + inventory:

| If you observe… | Extract… |
|-----------------|----------|
| Named RTOS tasks / priorities | Startup order, task ownership of sockets/resources |
| Thread pools / async runtimes | Who may call what; marshalling rules |
| HTTP/REST route tables | Route → handler map, auth gates |
| Message brokers / fieldbus | Session ownership, reconnect, LWT/session semantics if present |
| Global config / NV storage | Save/load call sites, schema, known SDK bugs |
| Feature compile flags | Per-target gates (TLS, SNMP, debug) with makefile/cmake proof |

Ownership columns in `COMPONENT_INVENTORY.md` (`Owner task`, `Socket/storage`) should use terms that fit the project (task, thread, process, request scope)—not a fixed RTOS vocabulary.

---

## Infer required artifacts

| Stack signal | Prefer artifacts like… |
|--------------|------------------------|
| Custom makefile/cmake flags | `artifacts/build/` excerpt with flags explained |
| Non-obvious SDK workaround | `artifacts/patterns/` with E1 citation |
| Public library API | `artifacts/interfaces/` header excerpt |
| Wire/protocol codec | `artifacts/patterns/` + `artifacts/data/` notes |
| REST/HTTP API | `artifacts/data/*.http` or route table |
| Bench/CI proof | `artifacts/bench/` logs or reports (for E2) |

Artifact IDs stay `A-{inventory-id}-…`. Content follows [ARTIFACT_STANDARD.md](ARTIFACT_STANDARD.md).

---

## Inventory platform rows

Add `PL##` rows for each **real** platform concern you document:

| ID | Name | Folder (example) |
|----|------|------------------|
| PL01 | Build / toolchain | platform/build |
| PL02 | Config / persistence | platform/… |
| PL03 | Concurrency / startup | project/architecture/… (if not pure platform) |

Names and folders should match **this** repository’s stack—not a canned vendor checklist.

---

## Failure modes

Document failure modes that are **evidenced or highly likely** for the detected stack (build races, config save bugs, TLS target gates, session teardown). Put them in the platform or library doc that owns the concern. Prefer a short table:

| Symptom | Likely cause | Where documented |
|---------|--------------|------------------|
| … | … | … |

Do not invent vendor-specific failure catalogs for stacks you did not detect.

---

## Target / platform matrix (when hardware or multi-target)

If the project targets devices or multiple deploy environments, include a matrix in `platform/README.md` or `platform-requirements.md`:

| Target | Built | Bench/CI verified | Notable features | Notes |
|--------|-------|-------------------|------------------|-------|
| … | Y/N | Y/N | … | … |

E2 only with files under `artifacts/bench/` (or equivalent retained proof). Otherwise E4 → OPEN_QUESTIONS.

---

## Agent decision rules

1. **Evidence first** — cite files; do not fill platform docs from training-data stereotypes about a vendor.
2. **Infer, then prune** — draft concerns from signals; drop what the repo does not support.
3. **Prefer project MCP/SDK skills** when the workspace provides them (build, flash, API search)—use those tools instead of guessing APIs.
4. **Stay portable** — LibraryDocs structure (`libraries/` / `project/` / `platform/` / `artifacts/`) is fixed; **content** is stack-specific.
5. **No false reuse** — if a “library” is tightly coupled to app tasks or globals, classify as project subsystem.

# Artifact Usefulness Criteria — LibraryDocs

An artifact is **useful** only if it reduces future agent or engineer work measurably. Prose-only LibraryDocs fails this bar.

## Usefulness test (all must pass)

| # | Test | Pass criterion |
|---|------|----------------|
| U1 | **Actionable** | Reader can copy-paste or adapt without opening full source |
| U2 | **Non-obvious** | Not trivially guessable from API name alone |
| U3 | **Bounded** | ≤120 lines; one concern per file |
| U4 | **Anchored** | EXCERPT header + EVIDENCE line with source path |
| U5 | **Linked** | Listed in `artifacts/README.md` with ID matching inventory |
| U6 | **Used** | Referenced from ≥1 component README |

Fail any test → revise or remove the artifact.

## Required artifacts by component type

| Component type | Required artifact types |
|--------------|-------------------------|
| **Library** | `interfaces/` header excerpt + ≥1 `patterns/` if non-trivial impl |
| **Subsystem with queue/marshal** | `patterns/` showing submit/execute loop |
| **Subsystem with persistence** | `data/` example blob + `patterns/` save/load |
| **REST/HTTP surface** | `data/*.http` or route table pattern |
| **Protocol encoder/decoder** | `patterns/` wire format + `data/` field notes |
| **Platform build** | `build/` makefile/cmake excerpt with flags explained |

## Artifact ID convention

Register in `artifacts/README.md`:

```markdown
| ID | File | Component | Usefulness | Description |
|----|------|-----------|------------|-------------|
| A-L01-if | interfaces/example_client.hpp | L01 example-client | U1–U6 | Public API surface |
| A-L01-pat | patterns/connect-handshake.cpp | L01 | U2 | Non-obvious handshake / level field |
```

## Good vs useless artifacts

| Useless | Useful |
|---------|--------|
| Empty stub with `// TODO` | 15-line SDK workaround with bug comment + E1 citation |
| Entire 800-line .cpp file | Op queue struct + SubmitOp claim/done loop |
| Header with only `#ifndef` guards | Public methods + constexpr limits |
| REST doc without example URLs | `.http` file with real query params |
| Duplicate of README prose | Wire format bytes not in README |

## patterns/ — when required

Create a pattern artifact when **any** of:

- SDK workaround or bug bypass
- Thread marshalling or op queue
- Serialization/blob layout
- Non-standard wire encoding (field 10 vs 11)
- Route registration idiom for the platform

## data/ — when required

Create data artifacts when **any** of:

- Persisted blob or config format
- REST/API examples
- Topic naming templates
- Protobuf/field number notes for decoders

## Artifact exemption (trivial public APIs)

When a library header is ≤15 lines and fully duplicated in `source_paths`, an agent may set in the **library README frontmatter**:

```yaml
artifacts_required: false
artifact_exemption: Public API fully represented by header; separate excerpt adds no retrieval value.
```

Validator accepts **no inventory artifact IDs** only when both fields are present. Do not create duplicate excerpts solely to pass validation.

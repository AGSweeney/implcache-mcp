# LibraryDocs test fixtures

Trees used by `librarydocs` unit tests and ingest integration tests.

| Fixture | Expected state |
|---------|----------------|
| `missing/` | `not_present` (no `LibraryDocs/`) |
| `unstructured/` | `unstructured` (folder only, no INDEX/inventory) |
| `structured/` | `structured` (INDEX + inventory, no VALIDATION) |
| `validated/` | `validated` (VALIDATION `result: pass`) |
| `invalid_validation/` | `invalid` (VALIDATION `result: fail`) |
| `malformed/` | warnings: bad frontmatter, duplicate IDs, traversal `source_paths` |
| `mqtt-client/` | unified-package example (validated mqtt-client library docs + source) |

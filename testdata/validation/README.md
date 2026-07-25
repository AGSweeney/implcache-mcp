# Real-source validation

Run:

```bash
go run ./cmd/sourcevalidate -out testdata/validation/reports -max-pages 25 -max-depth 2
```

JSON reports under `reports/` include per-scenario documents/chunks/symbols growth, errors, warnings, database growth, and elapsed time. The SQLite file is gitignored.

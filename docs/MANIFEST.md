# Workspace manifest (`.implcache.yaml`)

Optional file at the repository root. Loaded with `-workspace DIR`.

```yaml
rootName: example-control-app

technology:
  - Example Device SDK
  - Message Queue

languages:
  - cpp

authority: current_project

relatedRoots:
  - example-device-sdk
  - example-device-examples
  - protocol-reference

versions:
  device_sdk: "3.x"
  protocol: "1.0"

# How LibraryDocs/ is handled on git/project ingest (optional)
# auto | normal | exclude — see CREATE_LIBRARYDOCS.md and INGEST.md
libraryDocsHandling: auto
```

## Rules

- `rootName` is required and must not contain path separators
- Missing manifests do not break existing workflows
- Workspace filesystem paths are never written into portable `project://` document URIs
- Invalid YAML or missing `rootName` returns a clear startup error

# Agent guide

How coding agents should use ImplCache MCP.

## Default loop

```text
1. Identify: task, language, technology, project
2. Call get_implementation_context with those hints + preferredRoots when known
3. Write code from the package (APIs, sequence, examples, pitfalls)
4. Only if stuck:
     find_symbol for a specific identifier
     search_knowledge for a narrower question in the same root
     get_document(includeBody) for one cited URI
5. Avoid opening many Markdown files or web-searching first
```

## Prefer the primary tool

**Do:** `get_implementation_context`

**Don’t (as the first move):** dump `search_knowledge` with a high limit, or `get_document` on every hit.

The package is already budgeted. Extra tools spend tokens.

## Always scope roots when you can

Pass one of:

- `projectRoot` — current app corpus  
- `preferredRoots` — ordered list, e.g. `["my_app", "example-device-sdk"]`  
- `rootGroup` — named DB group  

If the tool returns `needsChoice` / `availableRoots`:

1. Ask the user which corpus applies  
2. Retry with explicit `rootName` or `preferredRoots`  
3. Do **not** guess across product families (e.g. example-control-app vs example-device-sdk)

## Reading the package

Treat these fields as authoritative for local knowledge:

| Field | Use |
|-------|-----|
| `requiredApis` / `relevantSymbols` | Call only these names unless you verify more |
| `includes` | Headers / imports to add |
| `sequence` | Initialization / call order |
| `examples` | Copy patterns; keep citations |
| `constraints` / `pitfalls` | Avoid known failure modes |
| `citations` | Grounding; open full docs only from these URIs |
| `coverage` | `low` → ask user or stage deeper retrieval |
| `webSearchRecommended` | Local KB may be incomplete or stale |
| `estimatedTokens` | Rough size; not a billing meter |

## Symbol precision

When the package names an API you’re unsure about:

```text
find_symbol { "name": "RegisterCommand", "preferredRoots": ["example-device-sdk"] }
```

Use the returned signature / URI. Do not invent overloads.

## When to use `vomit`

Use `vomit` when you need a **durable playbook** (longer Markdown recipe), not for every small edit.

- Always gets `body` back  
- Optional disk write under server `-output-root`  
- Optional `saveRecipe` stores a **generated** entry with lineage (ranked below human-reviewed)

After important recipes prove correct, a human should mark them `human_reviewed` in the DB (ops/process outside the agent default loop).

## Ingest during a session

Agents may call `ingest_project` / `ingest_markdown` when the user asks to index a tree **and** the server allows ingest.

Prefer the offline CLI for large corpora so the MCP session stays responsive.

## Anti-patterns

- Searching all roots with a vague query and mixing results  
- Requesting full bodies for every search hit  
- Treating generated recipes as more authoritative than project code  
- Ignoring `needsChoice` and continuing anyway  
- Replacing vendor version notes with guesses when `freshness` is unknown  

## Minimal example

**User:** Add an example-device-sdk menubar pushbutton in our app.

**Agent:**

```json
{
  "task": "register an example-device-sdk menubar pushbutton in RegisterHandler",
  "language": "c",
  "technology": "example-device-sdk",
  "preferredRoots": ["my_app", "example-device-sdk"],
  "maxContextTokens": 2500
}
```

Then implement from `sequence`, `requiredApis`, and `examples`, citing the returned URIs if asked.

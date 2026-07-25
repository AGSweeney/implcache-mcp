# Agent guide

How coding agents should use ImplCache MCP. Operators: keep the server on **`-mode agent`** for daily coding so ingest/delete tools are not exposed.

---

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

---

## Prefer the primary tool

**Do:** `get_implementation_context`

**Don’t (as the first move):** dump `search_knowledge` with a high limit, or `get_document` on every hit.

The package is already budgeted. Extra tools spend tokens.

---

## Always scope roots when you can

Pass one of:

- `projectRoot` — current app corpus  
- `preferredRoots` — ordered list, e.g. `["my_app", "vendor-sdk"]`  
- `rootGroup` — named group configured in the database / Librarian  

If the tool returns `needsChoice` / `availableRoots`:

1. Ask the user which corpus applies  
2. Retry with explicit `rootName` or `preferredRoots`  
3. Do **not** guess across product families  

---

## Reading the package

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

---

## Symbol precision

When the package names an API you’re unsure about:

```json
{
  "name": "RegisterCommand",
  "preferredRoots": ["vendor-sdk"]
}
```

Use the returned signature / URI. Do not invent overloads.

---

## When to use `vomit`

`vomit` is available only when the server runs in **admin** mode. Use it for a **durable playbook**, not for every small edit.

- Body is always returned to the agent  
- Optional disk write under the server `-output-root`  
- Optional `saveRecipe` stores a **generated** entry (ranked below human-reviewed)  

Important recipes should be human-reviewed before you trust them as canonical.

---

## Ingest during a session

Agents can call ingest tools only when the server is in **admin** mode and ingest is allowed.

Prefer the offline `ingestcli` for large corpora so the MCP session stays responsive.

---

## Anti-patterns

- Searching all roots with a vague query and mixing results  
- Requesting full bodies for every search hit  
- Treating generated recipes as more authoritative than project code  
- Ignoring `needsChoice` and continuing anyway  
- Replacing vendor version notes with guesses when `freshness` is unknown  

---

## Minimal example

**User:** Add a device-SDK menubar pushbutton in our app.

**Agent call:**

```json
{
  "task": "register a device-SDK menubar pushbutton in RegisterHandler",
  "language": "c",
  "technology": "device-sdk",
  "preferredRoots": ["my_app", "device-sdk"],
  "maxContextTokens": 2500
}
```

Then implement from `sequence`, `requiredApis`, and `examples`, citing the returned URIs if asked.

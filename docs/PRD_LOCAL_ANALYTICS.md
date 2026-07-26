# Product Requirements Document: Local Usage Analytics and Statistics Dashboard

**Product:** ImplCache MCP  
**Feature:** Local Usage Analytics, Outcome Reporting, and Statistics Dashboard  
**Status:** Proposed  
**Target Release:** Pre-release / Alpha hardening  
**Owner:** ImplCache project  
**Document Version:** 1.0

---

## 1. Executive Summary

ImplCache should add an optional local analytics subsystem that records how the product is used, what implementation evidence it returns, how much context it saves, and whether the returned package appears to help the coding agent complete its task.

Analytics data must be stored in a **separate SQLite database** from the primary ImplCache knowledge database.

Recommended files:

```text
implcache.db          # Ingested documents, symbols, roots, recipes, curated knowledge
implcache-usage.db    # Usage events, package metrics, outcomes, and dashboard data
```

The feature will include:

- Local metadata-only usage logging
- A WebUI setting to enable or disable analytics
- Configurable retention and data-clearing controls
- A Statistics page with summary cards, graphs, filters, and drill-down
- Optional agent-reported outcomes
- Privacy-safe defaults
- No external telemetry transmission

The purpose is not merely to count calls. The purpose is to provide defensible evidence that ImplCache is returning grounded implementation knowledge, reducing unnecessary context, and helping agents avoid unsupported generic answers.

---

## 2. Problem Statement

ImplCache currently returns structured implementation context, but there is no durable way to measure:

- How often agents use ImplCache
- Which roots and knowledge sources are most useful
- How often local evidence is available
- How often root selection is required
- How much context ImplCache returns
- How much raw source context is avoided
- Whether curated knowledge is being used
- Whether the first context package is sufficient
- Whether agents require additional retrieval or outside sources
- Whether implementation outcomes pass compilation or testing

Without analytics, users cannot easily demonstrate the value of ImplCache or identify weaknesses in retrieval quality.

ImplCache must also avoid contaminating the primary knowledge database with operational telemetry. The main database should remain portable, deterministic, and rebuildable from ingested sources.

---

## 3. Goals

### 3.1 Primary Goals

1. Record local usage and retrieval metrics without modifying the existing primary database schema.
2. Provide a WebUI Statistics page that makes ImplCache usage, grounding quality, context efficiency, and outcomes visible.
3. Keep analytics private, local, metadata-only, and easy to disable or erase.
4. Allow optional outcome reporting from agents or users.
5. Ensure analytics failures never interrupt MCP requests or retrieval.
6. Use stable identifiers and fingerprints so analytics remain meaningful after the main database is rebuilt.
7. Establish metrics that can support product claims such as:

```text
ImplCache supplied grounded local evidence for 87% of requests,
reduced delivered context by 92%, and eliminated further source
searches in 74% of reported tasks.
```

### 3.2 Secondary Goals

- Identify underused or high-value roots
- Identify stale, weak, or low-coverage knowledge areas
- Measure the value of curated knowledge
- Provide regression signals when retrieval quality changes
- Support local export of aggregate statistics
- Help users tune ingestion, recipes, root groups, and retrieval policy

---

## 4. Non-Goals

The first release will not:

- Transmit analytics to an external server
- Track users across installations
- Claim that every returned result was used by the model
- Automatically determine whether a final answer is “AI slop”
- Store full prompts by default
- Store full model answers by default
- Store returned proprietary source excerpts by default
- Require agents to report outcomes
- Add foreign keys from the usage database into the primary database
- Turn analytics into a cloud account or licensing system

---

## 5. Product Principles

### 5.1 Separate Knowledge from Usage

The primary database describes what ImplCache knows.

The usage database describes what ImplCache returned, how it performed, and what happened afterward.

### 5.2 Local by Default

All analytics remain on the local machine unless a future user explicitly exports them.

### 5.3 Metadata-Only by Default

The default configuration must avoid storing sensitive prompts, code, proprietary documentation, generated answers, IP addresses, usernames, and machine names.

### 5.4 Honest Metrics

ImplCache must distinguish:

```text
Tool invocation
Retrieval candidate
Selected evidence
Context package
Cited evidence
Reported successful outcome
```

A returned document is not automatically a successful hit.

### 5.5 Analytics Must Never Break Retrieval

If the usage database is unavailable, locked, corrupt, read-only, or out of disk space, the MCP request must still complete normally.

---

## 6. User Personas

### 6.1 Individual Developer

Wants to know whether ImplCache is saving context and improving local coding-agent performance.

### 6.2 Engineering Team Lead

Wants evidence that agents are using approved local documentation and curated implementation knowledge.

### 6.3 Knowledge Maintainer

Wants to identify low-coverage roots, missing recipes, stale sources, and frequently requested implementation areas.

### 6.4 Product Evaluator

Wants measurable proof that ImplCache performs better than raw file loading, generic search, or broad RAG retrieval.

---

## 7. User Stories

### Analytics Configuration

- As a user, I can enable or disable local analytics from the WebUI.
- As a user, I can see that no analytics data leaves my machine.
- As a user, I can choose a retention period.
- As a user, I can clear all analytics without affecting indexed knowledge.
- As a user, I can see the exact analytics database path.
- As a user, I can keep prompt and evidence-text capture disabled.

### Statistics Dashboard

- As a user, I can see how many implementation requests ImplCache handled.
- As a user, I can see how often local evidence was found.
- As a user, I can see how often curated knowledge contributed.
- As a user, I can see high, medium, low, and insufficient coverage trends.
- As a user, I can see estimated context reduction over time.
- As a user, I can see which roots, recipes, symbols, and documents are most used.
- As a user, I can filter statistics by date, root, client, model, tool, coverage, and outcome.
- As a user, I can drill into a request without exposing sensitive content by default.

### Outcome Reporting

- As an agent, I can optionally report whether the returned package was used.
- As an agent, I can report whether additional sources were needed.
- As an agent, I can report compile and test status.
- As a user, I can rate whether the package was helpful.
- As a knowledge maintainer, I can see missing or incorrect information reports.

---

## 8. Functional Requirements

## 8.1 Separate Usage Database

The system must support a separate SQLite database:

```text
implcache-usage.db
```

The usage database must:

- Have its own schema version
- Be independently created and migrated
- Be removable without affecting `implcache.db`
- Use WAL mode where appropriate
- Avoid hard foreign keys to the primary database
- Store stable external identifiers instead of volatile row IDs
- Support configurable retention cleanup
- Support disabled mode where the database is not opened for writes

Recommended stable identifiers:

```text
root_key
root_name
root_group_key
source_uri
document_uri
symbol_key
recipe_key
curated_entry_key
context_fingerprint
```

Internal numeric IDs from `implcache.db` may be stored only as optional diagnostic values and must not be required for analytics integrity.

---

## 8.2 Analytics Settings

Add a WebUI settings section:

### Usage Analytics

```text
[✓] Enable local usage analytics

Store local usage and retrieval metrics in implcache-usage.db.
No data leaves this machine.
```

Additional controls:

```text
[ ] Store request text for diagnostics
[ ] Store returned evidence excerpts
Retention: [90 days ▼]
Database path: ./data/implcache-usage.db
[Export Aggregate Metrics]
[Clear Analytics Data]
```

### Default Settings

| Setting | Default |
|---|---|
| Enable local usage analytics | On |
| Store request text | Off |
| Store evidence excerpts | Off |
| Store generated answers | Off |
| Retention period | 90 days |
| External transmission | Not supported |
| Anonymous session hashing | On |

Disabling analytics must:

- Stop new writes
- Avoid opening the database for normal write activity
- Preserve existing analytics until the user clears them
- Display “Analytics disabled” on the Statistics page
- Not affect retrieval or MCP operation

---

## 8.3 Automatic Request Telemetry

Each eligible MCP or HTTP retrieval request should create one request event.

Eligible tools may include:

- `get_implementation_context`
- `search_knowledge`
- `search_symbols`
- `get_document`
- `get_source_excerpt`
- recipe or curated-knowledge retrieval tools
- root-selection or root-resolution calls

The request record should include:

- Request UUID
- Timestamp
- Anonymous session hash
- Client name, when supplied
- Model name, when supplied
- Tool name
- Task hash
- Optional redacted task summary
- Selected roots and root groups
- Result status
- Root-selection-required flag
- Coverage level
- Freshness status
- Latency
- Estimated package tokens
- Estimated source tokens represented or avoided
- Context fingerprint
- Evidence counts
- Curated knowledge count
- Exact-symbol count
- Citation count
- Follow-up recommendation
- Error category, if applicable

### Result Status Enumeration

Recommended values:

```text
grounded_curated
grounded_local
grounded_mixed
root_selection_required
local_insufficient
no_local_match
request_error
```

---

## 8.4 Evidence Telemetry

For each request, record evidence-level metadata for:

- Curated knowledge entries
- Generated recipes
- Exact symbols
- Documents
- Code examples
- Constraints
- Pitfalls
- Sequence evidence
- Citations

Each evidence record should include:

- Request UUID
- Evidence type
- Stable evidence key
- Root key
- Source URI
- Authority level
- Rank position
- Selected-for-package flag
- Included-after-budget-trimming flag
- Estimated tokens
- Source hash, where available

The analytics database must not store the full evidence excerpt unless the user explicitly enables diagnostic evidence capture.

---

## 8.5 Context Efficiency Metrics

ImplCache should estimate and record:

- Final context package token count
- Candidate evidence token count
- Estimated full-source token count
- Estimated tokens avoided
- Reduction percentage
- Number of source files represented
- Number of source files actually returned
- Number of follow-up retrieval calls

Recommended formula:

```text
estimated_tokens_avoided =
max(estimated_full_source_tokens - returned_package_tokens, 0)
```

Recommended reduction percentage:

```text
context_reduction_percent =
estimated_tokens_avoided / estimated_full_source_tokens × 100
```

The UI must label these values as estimates.

---

## 8.6 Optional Outcome Reporting

Add an optional MCP tool:

```text
report_implementation_outcome
```

Suggested request:

```json
{
  "requestId": "uuid",
  "contextFingerprint": "sha256...",
  "outcome": "implemented",
  "usedImplCacheEvidence": true,
  "additionalSourcesUsed": false,
  "compileStatus": "passed",
  "testStatus": "passed",
  "firstPackageSufficient": true,
  "helpfulness": 5,
  "missingInformation": "",
  "incorrectInformation": ""
}
```

### Outcome Values

```text
implemented
partially_implemented
not_implemented
abandoned
unknown
```

### Compile/Test Values

```text
passed
failed
not_run
unknown
```

Outcome reporting must be optional and must not be interpreted as complete coverage of all requests.

The WebUI should clearly distinguish:

- Measured request metrics
- Inferred metrics
- Agent-reported outcomes
- User-reported outcomes

---

## 8.7 Statistics Page

Add a primary navigation entry:

```text
Analytics
```

The first release may use one page with tabs:

```text
Overview
Usage
Retrieval Quality
Context Efficiency
Knowledge Usage
Outcomes
```

---

## 9. Statistics Dashboard Requirements

## 9.1 Summary Cards

The Overview tab must show:

- Total implementation requests
- Requests answered from local evidence
- Requests using curated knowledge
- High-coverage rate
- Medium-coverage rate
- Low/insufficient-coverage rate
- Root-selection-required rate
- First-package success rate
- Average context package size
- Estimated total tokens avoided
- Average context reduction
- Reported compile success rate
- Reported test success rate

Each card should show:

- Current value
- Change versus previous matching period
- Tooltip explaining the metric
- Click action to filter or drill down

---

## 9.2 Required Graphs

### A. Requests Over Time

Chart type: line or bar chart

Series:

- Total requests
- Successful grounded requests
- Insufficient/no-match requests
- Errors

Granularity:

- Hourly
- Daily
- Weekly

### B. Coverage Over Time

Chart type: stacked area or stacked bar

Series:

- High
- Medium
- Low
- Insufficient
- No local match

### C. Grounding Source Breakdown

Chart type: donut or horizontal bar

Categories:

- Curated knowledge
- Generated recipes
- Exact symbols
- Raw indexed evidence
- Mixed local evidence
- No local match

### D. Context Efficiency

Chart type: line chart

Series:

- Average returned package tokens
- Average estimated source tokens
- Average tokens avoided
- Average reduction percentage

### E. Retrieval Depth

Chart type: stacked bar

Categories:

- First package sufficient
- Follow-up search required
- Raw document opened
- External verification recommended
- No useful local evidence

### F. Knowledge Usage

Chart type: sortable horizontal bar or table

Rank by:

- Root
- Root group
- Curated entry
- Recipe
- Symbol
- Source document

Metrics:

- Times selected
- Times included after trimming
- Reported successful outcomes
- Average coverage
- Average package contribution

### G. Outcomes

Chart type: grouped bar or donut

Categories:

- Implemented
- Partially implemented
- Not implemented
- Compile passed/failed
- Tests passed/failed
- Additional sources used
- First package sufficient

The page must indicate the number of requests with outcome reports so users do not mistake incomplete reporting for complete population statistics.

---

## 9.3 Filters

Global filters:

- Last 24 hours
- Last 7 days
- Last 30 days
- Last 90 days
- All time
- Custom date range
- Root
- Root group
- Client
- Model
- MCP tool
- Coverage level
- Result status
- Curated/generated/raw evidence
- Outcome status

Filters should update:

- Summary cards
- Graphs
- Ranked tables
- Request drill-down list

Filter state should be represented in the URL where practical.

---

## 9.4 Request Drill-Down

Clicking a chart point, card, or table row should open matching request records.

The request detail view should show:

- Timestamp
- Tool name
- Client
- Model
- Anonymous session identifier
- Selected roots
- Root resolution result
- Coverage
- Freshness
- Context fingerprint
- Estimated package tokens
- Estimated source tokens
- Estimated reduction
- Evidence counts
- Curated knowledge used
- Exact symbols used
- Follow-up retrieval recommendation
- Reported outcome
- Compile/test status
- Error details, when applicable

By default, it must not show:

- Full task text
- Full prompt
- Source excerpts
- Generated response
- User identity
- Client IP address

When diagnostic capture is enabled, sensitive fields must display a warning.

---

## 9.5 Privacy Status Banner

The Analytics page should show a persistent status block:

```text
Local analytics enabled
Metadata only
Database: ./data/implcache-usage.db
Retention: 90 days
No data leaves this machine
```

When disabled:

```text
Local analytics disabled
No new usage data is being recorded.
Existing analytics remain available until cleared.
```

---

## 10. Suggested Database Schema

The following is a starting point and may be adjusted during implementation.

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE usage_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE request_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL UNIQUE,
    occurred_at TEXT NOT NULL,

    session_hash TEXT,
    client_name TEXT,
    model_name TEXT,
    tool_name TEXT NOT NULL,

    task_hash TEXT,
    task_summary TEXT,

    result_status TEXT NOT NULL,
    coverage TEXT,
    freshness TEXT,

    latency_ms INTEGER,
    estimated_tokens INTEGER,
    estimated_source_tokens INTEGER,
    estimated_tokens_avoided INTEGER,
    context_reduction_percent REAL,

    context_fingerprint TEXT,

    root_selection_required INTEGER NOT NULL DEFAULT 0,
    additional_retrieval_recommended INTEGER NOT NULL DEFAULT 0,

    root_count INTEGER NOT NULL DEFAULT 0,
    source_count INTEGER NOT NULL DEFAULT 0,
    citation_count INTEGER NOT NULL DEFAULT 0,
    curated_count INTEGER NOT NULL DEFAULT 0,
    recipe_count INTEGER NOT NULL DEFAULT 0,
    symbol_count INTEGER NOT NULL DEFAULT 0,

    error_category TEXT,
    error_message TEXT
);

CREATE TABLE request_roots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    root_key TEXT,
    root_name TEXT,
    root_group_key TEXT,
    root_role TEXT,
    selected INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE evidence_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,

    evidence_type TEXT NOT NULL,
    evidence_key TEXT,
    root_key TEXT,
    source_uri TEXT,
    authority TEXT,

    rank_position INTEGER,
    selected_for_package INTEGER NOT NULL DEFAULT 0,
    included_after_trimming INTEGER NOT NULL DEFAULT 0,
    estimated_tokens INTEGER,

    source_hash TEXT
);

CREATE TABLE retrieval_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    occurred_at TEXT NOT NULL,

    retrieval_type TEXT NOT NULL,
    query_hash TEXT,
    root_key TEXT,

    candidate_count INTEGER,
    selected_count INTEGER,
    latency_ms INTEGER
);

CREATE TABLE outcome_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,

    request_id TEXT,
    context_fingerprint TEXT,

    reporter_type TEXT,
    outcome TEXT,

    used_implcache_evidence INTEGER,
    additional_sources_used INTEGER,
    first_package_sufficient INTEGER,

    compile_status TEXT,
    test_status TEXT,
    helpfulness INTEGER,

    missing_information TEXT,
    incorrect_information TEXT
);

CREATE INDEX idx_request_events_time
    ON request_events(occurred_at);

CREATE INDEX idx_request_events_status
    ON request_events(result_status);

CREATE INDEX idx_request_events_coverage
    ON request_events(coverage);

CREATE INDEX idx_request_events_fingerprint
    ON request_events(context_fingerprint);

CREATE INDEX idx_request_roots_request
    ON request_roots(request_id);

CREATE INDEX idx_request_roots_root
    ON request_roots(root_key);

CREATE INDEX idx_evidence_events_request
    ON evidence_events(request_id);

CREATE INDEX idx_evidence_events_key
    ON evidence_events(evidence_key);

CREATE INDEX idx_outcome_events_request
    ON outcome_events(request_id);

CREATE INDEX idx_outcome_events_fingerprint
    ON outcome_events(context_fingerprint);
```

---

## 11. Configuration Requirements

### CLI Flags

```text
--telemetry off|local
--usage-db <path>
--telemetry-retention-days <days>
--telemetry-store-task-text
--telemetry-store-evidence-text
```

### Environment Variables

```text
IMPLCACHE_TELEMETRY=local
IMPLCACHE_USAGE_DB=./data/implcache-usage.db
IMPLCACHE_TELEMETRY_RETENTION_DAYS=90
IMPLCACHE_TELEMETRY_STORE_TASK_TEXT=false
IMPLCACHE_TELEMETRY_STORE_EVIDENCE_TEXT=false
```

### Precedence

Recommended precedence:

```text
CLI flags
Environment variables
WebUI persisted configuration
Built-in defaults
```

---

## 12. Logging Architecture

Preferred architecture:

```text
MCP request completes
        ↓
Response returned to caller
        ↓
Telemetry event placed on bounded queue
        ↓
Background writer batches SQLite inserts
```

Requirements:

- Logging must be best-effort
- Queue must be bounded
- Dropped event count must be tracked
- Database failures must be logged locally
- Request completion must not wait indefinitely for analytics
- Shutdown should attempt a short bounded flush
- A synchronous fallback may be used for very small deployments, but retrieval must still survive logging failures

Recommended batch behavior:

- Flush every 250–1000 ms
- Flush at 50–100 queued events
- Use one writer goroutine
- Use transactions for batches
- Avoid one SQLite connection per event

---

## 13. Retention and Maintenance

The system must support:

- 30-day retention
- 90-day retention
- 365-day retention
- Unlimited retention
- Manual clear
- Optional VACUUM after clear or large purge
- Display of database size
- Display of oldest and newest records

Retention cleanup should run:

- At startup
- Once per day while running
- On explicit user request

Clearing analytics must require confirmation:

```text
This will permanently delete local analytics and outcome records.
It will not delete indexed documents, roots, recipes, or curated knowledge.
```

---

## 14. Export Requirements

The first release should support aggregate export as:

- JSON
- CSV

Export should include:

- Selected time range
- Active filters
- Summary metrics
- Time-series data
- Root usage
- Knowledge usage
- Outcome aggregates

Raw request export may be added later and should remain separate from aggregate export.

---

## 15. Security and Privacy Requirements

- No analytics data leaves the machine
- No network transmission code is required for this feature
- No prompt text is stored by default
- No evidence excerpts are stored by default
- No generated answer is stored by default
- No client IP address is stored
- No username or machine name is stored
- Session IDs should be hashed with an installation-local salt
- Task hashes should not use unsalted low-entropy values
- Sensitive diagnostic capture must be visibly labeled
- Disabling analytics must stop new writes
- Clearing analytics must be easy and complete
- Analytics endpoints must follow existing WebUI authentication and authorization rules

---

## 16. UI/UX Requirements

### Navigation

Add:

```text
Analytics
```

Suggested icon: chart, pulse, or activity icon.

### Empty State

When no analytics exist:

```text
No usage analytics yet

ImplCache will record local metadata as agents use retrieval tools.
No prompts or source text are stored with the default settings.
```

### Disabled State

```text
Analytics are disabled

Enable local analytics in Settings to begin recording retrieval metrics.
No data will leave this machine.
```

### Error State

If the analytics database cannot be opened:

```text
Analytics unavailable

ImplCache retrieval remains operational, but usage metrics could not
be recorded. Check the usage database path and file permissions.
```

### Accessibility

- Graphs must include textual summaries
- Do not rely on color alone
- Tables must be keyboard accessible
- Tooltips must have accessible labels
- Charts must support light and dark themes
- Values must remain readable at common desktop resolutions

---

## 17. API Requirements

Suggested HTTP endpoints:

```text
GET    /api/analytics/status
GET    /api/analytics/summary
GET    /api/analytics/timeseries
GET    /api/analytics/coverage
GET    /api/analytics/efficiency
GET    /api/analytics/knowledge
GET    /api/analytics/outcomes
GET    /api/analytics/requests
GET    /api/analytics/requests/:requestId
POST   /api/analytics/outcomes
POST   /api/analytics/export
DELETE /api/analytics/data
PUT    /api/settings/analytics
```

Common query parameters:

```text
from
to
bucket
root
rootGroup
client
model
tool
coverage
status
outcome
limit
offset
```

All analytics query endpoints should be read-only except:

- Outcome submission
- Settings update
- Clear/delete
- Export generation

---

## 18. Metrics Definitions

### Local Evidence Rate

```text
Requests with grounded_curated, grounded_local, or grounded_mixed
divided by eligible implementation requests
```

### Curated Knowledge Usage Rate

```text
Requests containing one or more curated entries
divided by grounded requests
```

### Root Selection Rate

```text
Requests returning root_selection_required
divided by eligible requests
```

### High Coverage Rate

```text
High-coverage requests
divided by grounded requests
```

### First-Package Success Rate

Use reported outcomes only:

```text
Reported requests where firstPackageSufficient = true
divided by requests containing that reported field
```

### Context Reduction

```text
1 - returned_package_tokens / estimated_full_source_tokens
```

Only calculate when estimated full-source tokens are greater than zero.

### Compile Success Rate

Use reported outcomes only:

```text
compile_status = passed
divided by compile_status in {passed, failed}
```

### Test Success Rate

Use reported outcomes only:

```text
test_status = passed
divided by test_status in {passed, failed}
```

All outcome-based charts must display the outcome-report sample size.

---

## 19. Acceptance Criteria

### Database Separation

- [ ] Analytics uses a separate SQLite database.
- [ ] Existing `implcache.db` schema remains unchanged.
- [ ] Deleting `implcache-usage.db` does not affect retrieval.
- [ ] Rebuilding `implcache.db` does not invalidate all aggregate analytics.
- [ ] Stable keys and fingerprints are used instead of mandatory cross-database row IDs.

### Settings

- [ ] Analytics can be enabled and disabled in the WebUI.
- [ ] Analytics is local-only and metadata-only by default.
- [ ] Prompt and evidence capture are off by default.
- [ ] Retention is configurable.
- [ ] Users can clear analytics without deleting knowledge.
- [ ] Database path and size are visible.

### Reliability

- [ ] Retrieval succeeds when analytics database writes fail.
- [ ] Logging does not materially delay MCP responses.
- [ ] Dropped analytics events are counted.
- [ ] Database lock and disk-full conditions are handled gracefully.

### Dashboard

- [ ] Summary cards are available.
- [ ] Requests-over-time graph is available.
- [ ] Coverage graph is available.
- [ ] Grounding-source breakdown is available.
- [ ] Context-efficiency graph is available.
- [ ] Retrieval-depth graph is available.
- [ ] Knowledge-usage ranking is available.
- [ ] Outcome graphs are available.
- [ ] Global filters update all applicable views.
- [ ] Request drill-down works.
- [ ] Sensitive content is not shown by default.

### Outcome Reporting

- [ ] An optional outcome-reporting API or MCP tool exists.
- [ ] Outcome reports can link by request ID or context fingerprint.
- [ ] Compile/test status is supported.
- [ ] Additional-source usage is supported.
- [ ] Missing and incorrect information can be reported.
- [ ] Outcome sample sizes are visible in the UI.

---

## 20. Phased Delivery

### Phase 1 — Telemetry Foundation

Deliver:

- Separate `implcache-usage.db`
- Request event logging
- Evidence metadata logging
- Config flags and environment variables
- Enable/disable setting
- Best-effort writer
- Basic retention
- No dashboard beyond status

### Phase 2 — Analytics Overview

Deliver:

- Analytics navigation
- Summary cards
- Requests-over-time graph
- Coverage graph
- Grounding breakdown
- Date filters
- Root/tool/client filters

### Phase 3 — Context Efficiency and Knowledge Usage

Deliver:

- Token savings estimates
- Context efficiency graphs
- Retrieval-depth reporting
- Root/recipe/symbol/document rankings
- Request drill-down
- Aggregate export

### Phase 4 — Outcomes

Deliver:

- `report_implementation_outcome`
- Compile/test reporting
- First-package sufficiency
- Additional-source reporting
- User helpfulness rating
- Outcome charts

### Phase 5 — Quality and Regression Analytics

Potential future work:

- Compare retrieval algorithm versions
- Track package fingerprints across builds
- Detect coverage regressions
- Benchmark curated versus uncurated results
- Compare models and clients
- Flag frequently requested low-coverage topics
- Suggest candidates for curated knowledge creation

---

## 21. Open Questions

1. Should local analytics be enabled automatically on first launch or enabled through a first-run consent screen?
2. How should estimated full-source tokens be calculated consistently across document types?
3. Should HTTP API requests and MCP requests share the same event model?
4. Should an agent be able to report multiple outcomes against one context fingerprint?
5. Should user ratings be stored as separate events or merged with outcome records?
6. Should task summaries be generated locally when full prompt storage is disabled?
7. How should imported or copied usage databases handle installation-specific session hashes?
8. Should analytics configuration live in the existing settings store or a standalone configuration file?
9. Should the dashboard distinguish generated recipes from curated knowledge at all times?
10. Should source freshness and authority be charted in the first release or deferred?

---

## 22. Recommended Initial Product Decision

For the first implementation:

- Keep analytics **enabled by default**
- Make the default strictly **local and metadata-only**
- Do not store prompt text
- Do not store source excerpts
- Do not store generated answers
- Use a separate SQLite database
- Provide a visible disable switch
- Provide a 90-day default retention period
- Provide one Analytics page with tabs
- Treat outcome reporting as optional
- Label token savings and context reduction as estimates
- Ensure analytics failure never affects retrieval

This provides useful product evidence without compromising the simplicity, privacy, or determinism of ImplCache’s primary knowledge database.

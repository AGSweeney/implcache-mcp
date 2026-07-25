# Product Requirements Document

## ImplCache Librarian Web Application

**Product:** ImplCache  
**Component:** Browser-based administration and curation frontend  
**Status:** Proposed  
**Primary implementation:** TypeScript web application served by the ImplCache Go backend  
**Target environments:** Local workstation, on-premises server, and shared team deployment  
**Primary users:** Developers, technical administrators, and maintainers of ImplCache knowledge libraries

---

## 1. Summary

ImplCache Librarian is the browser-based administration, inspection, curation, and validation interface for ImplCache.

ImplCache already provides or is developing:

- an MCP retrieval server;
- administrative tools;
- local-tree ingestion;
- Git repository ingestion;
- online documentation ingestion;
- PDF ingestion;
- SQLite-backed storage;
- full-text and sparse semantic retrieval;
- roots and root groups;
- source versioning and metadata;
- symbols, recipes, citations, and implementation-context assembly.

The Librarian web application will provide a unified interface over these capabilities.

The frontend must not duplicate ingestion, schema, ranking, or storage logic. It will communicate with the ImplCache server through versioned administrative APIs and real-time job channels.

The intended architecture is:

```text
Browser
   │
   ▼
ImplCache Librarian Web UI
   │
   ├── REST/JSON Admin API
   ├── WebSocket or Server-Sent Events
   └── Authenticated file upload endpoints
   │
   ▼
ImplCache Go Server
   │
   ├── source acquisition
   ├── ingestion
   ├── indexing
   ├── search
   ├── health checks
   └── SQLite database
```

The compiled frontend should be embeddable into the Go server so a normal deployment can provide the server and Librarian from one executable.

---

## 2. Product vision

ImplCache Librarian should make building and maintaining a high-quality implementation-context library understandable, visual, and safe.

A user should be able to answer:

```text
What sources are in the library?
Which version or commit is indexed?
Which sources are stale or unhealthy?
What content was extracted?
What will be indexed before I start?
What was skipped?
Why did a retrieval result rank highly?
What requires attention?
```

The Librarian should make the internal state of the library visible without requiring:

- direct SQLite access;
- manually constructed MCP tool calls;
- shell scripts;
- raw JSON editing;
- log-file inspection for normal workflows.

---

## 3. Problem

ImplCache is gaining several source types and administrative workflows:

```text
local directories
local files
Git repositories
documentation websites
single web pages
PDF documents
manual knowledge
roots and root groups
```

Command-line tools and MCP admin calls are useful for automation but become cumbersome for routine administration.

Without a GUI:

- source configuration is difficult to inspect;
- include and exclude rules are easy to misconfigure;
- crawl and repository scope may be unclear;
- version conflicts may go unnoticed;
- long-running jobs are difficult to monitor;
- extraction quality is not visible;
- failed or partial ingestion can be overlooked;
- retrieval tuning requires manual tool calls;
- shared server administration becomes inconvenient.

The Librarian should provide one web interface for all source and library operations.

---

## 4. Goals

The Librarian shall:

- provide a unified view of all registered sources;
- support local and remote ImplCache deployments;
- guide users through source setup;
- preview source scope before ingestion;
- monitor long-running jobs in real time;
- inspect extracted documents, chunks, symbols, recipes, and metadata;
- manage roots and root groups;
- expose library health issues;
- provide a retrieval testing workbench;
- preserve exact server behavior and ranking;
- support responsive layouts for desktop and tablet browsers;
- require no separate desktop installation;
- be served from the ImplCache backend in standard deployments.

---

## 5. Non-goals

The Librarian is not:

- a replacement for the ImplCache server;
- a direct SQLite browser;
- a source-code IDE;
- a Git client replacement;
- a full web crawler UI for arbitrary internet use;
- a PDF editor;
- a document-authoring system;
- a general-purpose AI chat interface;
- a cloud SaaS product in the initial release;
- a full role-based enterprise administration portal;
- a continuous-integration system.

---

## 6. Users

### 6.1 Technical administrator

The administrator:

- registers sources;
- configures versions, roots, and scope;
- starts ingestion and refresh jobs;
- resolves failures;
- manages source credentials;
- reviews health and warnings;
- removes stale content.

### 6.2 Developer

The developer:

- verifies that required source material is indexed;
- searches symbols and documentation;
- inspects citations;
- compares semantic and non-semantic retrieval;
- reports retrieval gaps.

### 6.3 Team librarian

A team librarian maintains a shared ImplCache server for multiple developers.

The initial design should support this use case while deferring complex enterprise permissions.

---

## 7. Product principles

### 7.1 Server owns business logic

The server owns:

```text
validation
source acquisition
ingestion
refresh
deletion
schema
storage
ranking
search
health
```

The frontend:

```text
collects input
calls APIs
renders results
shows progress
orchestrates user workflows
```

### 7.2 No direct database access

The frontend must never open or modify the SQLite database.

### 7.3 Preview before committing

Whenever practical, show:

```text
what will be indexed
what will be excluded
what will change
what will be removed
what warnings exist
```

before an operation begins.

### 7.4 Safe defaults

The UI should default to:

```text
bounded crawls
shallow Git clones
no Git history indexing
no OCR unless enabled
no automatic deletion after temporary web failures
no private-network web crawling
no execution of repository content
```

### 7.5 Explainability

The UI should expose server-provided:

```text
source identity
version or commit
authority
freshness
citation
ranking components
warnings
```

---

## 8. Technical architecture

## 8.1 Frontend stack

Recommended stack:

```text
TypeScript
React
Vite
React Router
TanStack Query
TanStack Table or equivalent
WebSocket or EventSource client
Monaco or CodeMirror for source/code previews
Markdown renderer with syntax highlighting
```

A comparable TypeScript framework may be used if the team prefers it, but the application must remain a separately buildable static frontend.

## 8.2 Backend integration

The Go server shall expose:

```text
/api/v1/
```

for administrative and inspection operations.

The frontend production build shall be embedded using:

```go
//go:embed
```

or served from a configured static directory during development.

## 8.3 Development workflow

Recommended:

```text
frontend dev server
→ API proxy to local Go server

production build
→ static assets embedded into Go executable
```

No Node.js runtime shall be required in production.

## 8.4 Real-time transport

Preferred transport:

```text
Server-Sent Events for job progress and notifications
```

Use WebSockets if bidirectional real-time communication becomes necessary.

Polling shall be supported as a fallback.

---

## 9. Deployment models

### 9.1 Local single-user

```text
Browser on workstation
→ localhost ImplCache server
```

The server may open the Librarian automatically after startup.

### 9.2 Shared on-premises server

```text
Multiple browsers
→ HTTPS ImplCache server
→ shared knowledge library
```

### 9.3 Headless deployment

The server may run with the Librarian disabled.

Configuration:

```text
-enable-librarian
-librarian-base-path
```

or equivalent.

### 9.4 Reverse proxy

The web app should support deployment behind:

```text
IIS
Nginx
Caddy
corporate reverse proxy
```

The frontend must support a configurable base path such as:

```text
/implcache/
```

---

## 10. Main navigation

Primary navigation:

```text
Dashboard
Sources
Library
Roots
Search Lab
Jobs
Health
Logs
Settings
```

A persistent connection-status indicator shall show:

```text
server name
connection state
server version
API version
schema version
read-only state
```

---

## 11. Dashboard

The Dashboard shall summarize library condition.

### 11.1 Summary metrics

Display:

```text
total sources
healthy sources
sources needing refresh
failed sources
documents
chunks
symbols
recipes
database size
active jobs
```

### 11.2 Attention queue

Show actionable issues:

```text
failed ingestion
incomplete crawl
repository refresh failure
dirty local checkout
source unreachable
low-confidence PDF extraction
version conflict
target conflict
missing page pending prune
damaged schema
duplicate content
```

### 11.3 Recent activity

Show:

```text
source added
source refreshed
source removed
job completed
job failed
root changed
configuration changed
```

---

## 12. Unified source management

The Sources page shall list:

```text
Local Tree
Local File
Git Repository
Documentation Website
Single Web Page
PDF Document
Manual Knowledge
```

### 12.1 Source list columns

```text
Name
Type
Root
Product
Version / Commit
Target
Authority
Status
Last Successful Ingest
Documents
Warnings
```

### 12.2 Actions

```text
Add
Inspect
Preview
Ingest
Refresh
Edit
Enable
Disable
Remove
View Documents
View Jobs
View Errors
```

### 12.3 Filters

```text
type
status
root
product
version
target
authority
warning state
```

---

## 13. Add Source workflow

Selecting **Add Source** shall open a source-type chooser.

Available source cards:

```text
Local Directory
Local File
Git Repository
Documentation Website
Single Web Page
PDF Document
Manual Knowledge
```

Each type shall use a dedicated multi-step wizard.

---

## 14. Local directory workflow

Steps:

### Location

```text
server-side path
root name
```

For remote servers, the UI shall clearly state that the path is on the server, not the browser workstation.

### Scope

```text
include patterns
exclude patterns
language filters
maximum file size
```

### Metadata

```text
product
version
target
authority
```

### Preview

Show:

```text
included files
excluded files
file tree
estimated bytes
estimated documents
warnings
```

### Ingest

Create a server-side job and display progress.

---

## 15. File upload workflow

For PDFs or individual local files, the browser may upload content to the server.

Requirements:

- configurable maximum upload size;
- resumable upload deferred;
- upload progress;
- temporary storage cleanup;
- filename sanitization;
- content-type validation;
- server-generated upload token;
- explicit ingestion after upload.

The user shall be able to choose:

```text
upload and ingest
upload for inspection only
```

---

## 16. Git repository workflow

Steps:

### Repository

```text
remote URL or server-side local checkout
snapshot / managed clone / local checkout
credential reference
```

### Ref

```text
branch
tag
commit
resolved commit SHA
```

### Scope

```text
sparse paths
include patterns
exclude patterns
submodule policy
symlink policy
working-tree mode
```

### Inspect

Show:

```text
repository identity
default branch
resolved ref
file counts
content classes
dirty state
warnings
```

### Preview

Show filterable file tree.

### Ingest

Display:

```text
clone/fetch
checkout
discovery
parsing
indexing
finalization
```

---

## 17. Documentation website workflow

Steps:

### Start URL

```text
URL
HTTPS status
host
```

### Profile

```text
auto
generic
Sphinx
Doxygen
```

### Scope

```text
allowed prefixes
include patterns
exclude patterns
max pages
max depth
response limit
total byte limit
robots policy
```

### Metadata

```text
root
product
declared version
detected version
target
language
authority
```

### Preview

Show:

```text
discovered URLs
included pages
excluded pages
blocked URLs
external links
duplicate canonicals
warnings
```

### Ingest

Display page-level crawl progress.

---

## 18. PDF workflow

Steps:

### Upload or server path

```text
file
size
page count
encryption state
```

### Inspect

Show:

```text
title
author
detected version
text/image/mixed classification
bookmarks
pages with insufficient text
OCR requirement
```

### Scope

```text
page range
OCR mode
split mode
table mode
```

### Metadata

```text
root
product
version
authority
language
```

### Preview

Show:

```text
section tree
sample extracted pages
page citations
warnings
```

### Ingest

Display page extraction and indexing progress.

---

## 19. Preview system

Preview operations shall not alter indexed content.

A preview response should include:

```text
source identity
detected metadata
included items
excluded items
estimated documents
estimated chunks
estimated bytes
warnings
blocking errors
```

The user shall be able to export preview results as JSON.

---

## 20. Jobs

Long-running work shall execute as server-side jobs.

Examples:

```text
Git clone
Git refresh
documentation crawl
PDF extraction
large tree ingestion
prune
source removal
```

### 20.1 States

```text
queued
running
pausing
paused
cancelling
cancelled
completed
completed_with_warnings
failed
```

### 20.2 Job display

Show:

```text
source
operation
stage
current item
completed count
total count
bytes processed
elapsed time
warnings
errors
```

### 20.3 Recovery

The frontend shall reconnect to active jobs after:

```text
page reload
browser restart
temporary network loss
server reconnect
```

---

## 21. Library browser

The Library page shall expose indexed content.

Hierarchy:

```text
Root
  Source
    Document
      Chunks
      Symbols
      Recipes
```

### Document columns

```text
Title
URI
Source Type
Product
Version
Path / URL
Chunks
Symbols
Updated
Warnings
```

### Document details

Tabs:

```text
Overview
Normalized Content
Chunks
Symbols
Recipes
Metadata
Source
Extraction Report
```

### Chunk view

Display:

```text
heading path
body
source page or line range
token estimate
authority
freshness
citations
```

### Symbol view

Display:

```text
name
qualified name
kind
signature
namespace
language
source path
line or page
root
authority
```

---

## 22. Roots and root groups

The Roots page shall manage:

```text
roots
root groups
product
version
target
preferred ordering
workspace mappings
```

Example:

```text
ESP-IDF 5.5 / ESP32
├── official web documentation
├── tagged source repository
├── public headers
└── examples
```

Actions:

```text
create group
rename
add root
remove root
reorder
set preferred root
assign metadata
```

Warnings:

```text
mixed versions
mixed targets
missing source
duplicate root
```

---

## 23. Search Lab

Search Lab shall use the same backend retrieval functions as coding agents.

Modes:

```text
Search Knowledge
Find Symbol
Get Implementation Context
Get Document
```

Controls:

```text
query
roots
root group
product
version
target
authority
result limit
token budget
semantic on/off
include examples
include recipes
```

Display:

```text
rank
score
score components
authority
freshness
semantic score
symbol match
source
citation
estimated tokens
fingerprint
```

Support comparison:

```text
semantic off versus on
root group A versus B
version A versus B
```

---

## 24. Health

Health checks shall include:

```text
documents with no chunks
chunks with no postings
symbols with missing sources
failed refreshes
unreachable web sources
repository refs that moved
dirty local checkouts
missing pages
duplicate documents
version conflicts
target conflicts
low-confidence PDF pages
schema mismatch
sources never successfully ingested
```

Each issue shall include:

```text
severity
source
description
recommended action
related link
```

---

## 25. Logs

The Logs page shall provide:

```text
severity filtering
component filtering
source filtering
job filtering
text search
time range
```

Actions:

```text
copy
export
open source
open job
```

Secrets must remain redacted.

---

## 26. Authentication and authorization

### 26.1 Local mode

Loopback-only deployments may allow unauthenticated access when explicitly configured.

### 26.2 Shared mode

Shared deployments shall require authentication.

Initial supported method:

```text
bearer API token
```

Future methods:

```text
OIDC
Windows integrated authentication
mutual TLS
```

### 26.3 Permissions

Initial roles:

```text
Viewer
Administrator
```

Viewer:

```text
browse
search
view jobs
view health
```

Administrator:

```text
add
edit
ingest
refresh
remove
prune
configure
```

Role-based access may be implemented after the first single-admin release, but API boundaries should anticipate it.

---

## 27. Credential handling

Repository credentials and other secrets shall not be entered as raw values into normal source forms.

The UI should use named credential references.

Examples:

```text
github-work
gitlab-vendor
internal-docs
```

The frontend must never display stored secret values.

Uploaded tokens shall use dedicated secure endpoints and shall not be returned by the API.

---

## 28. Admin API

Recommended endpoint groups:

```text
/api/v1/server
/api/v1/sources
/api/v1/sources/local
/api/v1/sources/git
/api/v1/sources/web
/api/v1/sources/pdf
/api/v1/roots
/api/v1/root-groups
/api/v1/library
/api/v1/search
/api/v1/jobs
/api/v1/health
/api/v1/logs
/api/v1/uploads
```

Required API capabilities:

```text
list sources
inspect source
preview source
add source
edit source
ingest source
refresh source
remove source
list jobs
read job
cancel job
list roots
manage root groups
browse documents
browse chunks
browse symbols
run retrieval
read health checks
upload files
```

---

## 29. API compatibility

The frontend shall query server capabilities at startup.

Capability response should include:

```text
server version
API version
schema version
supported source types
supported job features
semantic-search support
OCR support
read-only status
authentication mode
```

The UI shall disable unsupported features rather than fail unexpectedly.

---

## 30. Error handling

Errors shall contain:

```text
stable code
message
technical detail
source ID
job ID
retryable flag
recommended action
```

UI categories:

```text
validation
authentication
authorization
connection
source
ingestion
server
schema
unsupported feature
```

Detailed technical data should be available in an expandable panel.

---

## 31. Responsive design

Primary target:

```text
desktop browser
```

Secondary target:

```text
tablet
```

Mobile phones may support monitoring and simple actions but are not a primary editing surface.

Desktop layouts may use:

```text
resizable panes
data grids
tree views
drawers
modal wizards
```

---

## 32. Accessibility

Requirements:

```text
keyboard navigation
screen-reader labels
focus indicators
sufficient contrast
scalable text
non-color status indicators
ARIA labeling
```

---

## 33. Performance

The frontend shall support large libraries through:

```text
server-side pagination
lazy loading
virtualized tables
request cancellation
debounced search
incremental tree loading
```

The browser must not load the full library or all symbols at once.

Target scenarios:

```text
100,000 documents
millions of chunks
large Git trees
large job histories
```

---

## 34. Caching

The frontend may cache:

```text
server capabilities
source summaries
recent queries
table preferences
navigation state
```

Use query-cache invalidation after administrative operations.

The frontend shall not duplicate the ImplCache database.

---

## 35. Security

The application shall:

- use secure cookies or authorization headers;
- support HTTPS;
- protect state-changing requests from CSRF;
- validate uploaded files server-side;
- apply Content Security Policy;
- avoid unsafe HTML rendering;
- sanitize Markdown and extracted content;
- never expose filesystem paths unnecessarily;
- redact credentials;
- require explicit confirmation for destructive actions;
- avoid exposing admin APIs when the server runs in agent-only mode.

---

## 36. Destructive actions

Actions requiring confirmation:

```text
remove source
remove indexed content
remove managed clone
prune missing pages
delete uploaded file
cancel job during finalization
```

Confirmation dialogs shall state exactly what will be removed.

Typed confirmation may be used for large destructive operations.

---

## 37. Observability

The frontend shall expose:

```text
connection status
job status
server health
API latency
recent errors
```

Client-side errors may be logged locally and optionally reported to a server endpoint.

Do not send telemetry externally by default.

---

## 38. Appearance

Support:

```text
system theme
light theme
dark theme
```

Design principles:

```text
high information density
clear status indicators
minimal decorative graphics
readable code and metadata
persistent column widths
resizable panels
```

---

## 39. Packaging and distribution

The frontend build shall produce static assets.

Standard distribution:

```text
ImplCache executable
├── embedded MCP server
├── embedded admin API
└── embedded Librarian assets
```

Alternative development distribution:

```text
Go server
+
separate frontend dev server
```

No Node.js runtime shall be required for users.

---

## 40. Update strategy

The Librarian version should normally match the server release because the assets are embedded.

Display:

```text
server version
frontend build version
API version
schema version
```

A mismatched externally hosted frontend shall receive a compatibility warning.

---

## 41. Testing

### Unit tests

Test:

```text
form validation
API models
error rendering
permissions
source-type workflows
query-state handling
formatting
```

### Component tests

Test:

```text
source tables
wizards
preview trees
job progress
document inspector
Search Lab
health issues
```

### API integration tests

Use a controlled test server.

Test:

```text
source list
preview
add
ingest
refresh
remove
job events
pagination
search
connection loss
read-only mode
permission denial
```

### End-to-end tests

Recommended framework:

```text
Playwright
```

Flows:

```text
connect
add local source
add Git source
add web source
upload PDF
monitor job
browse document
run search
remove source
```

### Security tests

Test:

```text
XSS in extracted content
CSRF
authorization failures
malicious filenames
oversized uploads
credential redaction
unsafe Markdown
```

---

## 42. Acceptance criteria

The first release is complete when:

- the Librarian is accessible from a browser;
- the frontend is served by the Go server;
- users can view server and schema status;
- all source types appear in one source list;
- users can inspect and preview sources;
- users can add and ingest supported sources;
- long-running jobs report live progress;
- users can browse documents, chunks, and symbols;
- users can manage roots and root groups;
- Search Lab calls real retrieval APIs;
- health issues are visible and actionable;
- read-only and admin permissions are enforced;
- destructive operations require confirmation;
- the application works without direct database access;
- production requires no Node.js runtime;
- unit, integration, security, and end-to-end tests pass.

---

## 43. Release stages

### Stage 1 — Operational shell

Deliver:

```text
connection
dashboard
source list
jobs
logs
server capabilities
```

### Stage 2 — Source workflows

Deliver:

```text
local tree
Git repository
documentation website
PDF upload
preview
ingestion
refresh
remove
```

### Stage 3 — Library inspection

Deliver:

```text
documents
chunks
symbols
recipes
metadata
roots
root groups
```

### Stage 4 — Search and health

Deliver:

```text
Search Lab
ranking explanations
health checks
comparison tools
```

### Stage 5 — Shared deployment

Deliver:

```text
authentication
viewer/admin roles
HTTPS guidance
reverse-proxy support
```

---

## 44. Risks

### Backend API instability

The frontend may be built before source APIs stabilize.

Mitigation:

```text
versioned API
capability discovery
generated TypeScript clients
contract tests
```

### Overly complex first release

Trying to implement every source wizard and inspector at once may delay delivery.

Mitigation:

```text
stage releases
reuse common form components
start with source list and jobs
```

### Large-library browser performance

Large tables and trees may overwhelm the browser.

Mitigation:

```text
pagination
virtualization
lazy loading
server-side filtering
```

### Security exposure

A web admin interface increases attack surface.

Mitigation:

```text
loopback-only default
authentication for remote access
HTTPS
CSRF protection
strict API authorization
CSP
```

### Local-path confusion

Browser users may mistake server paths for workstation paths.

Mitigation:

```text
explicit server-path labels
file upload workflow
server identity always visible
```

---

## 45. Future enhancements

Potential future work:

- saved source templates;
- scheduled refresh UI;
- multi-user audit history;
- OIDC integration;
- source configuration import/export;
- side-by-side document diff;
- retrieval evaluation suites;
- knowledge coverage reports;
- browser notifications;
- desktop helper for local-folder handoff;
- PWA installation;
- mobile monitoring view;
- plugin architecture for source-specific inspectors.

---

## 46. Recommended implementation order

1. define and version the admin API;
2. implement server capability discovery;
3. implement server-side jobs and progress events;
4. create frontend application shell;
5. create source list and source details;
6. implement Git and web source workflows;
7. implement PDF upload and inspection;
8. implement Library browser;
9. implement Roots and root groups;
10. implement Search Lab;
11. implement Health;
12. add authentication and shared deployment controls;
13. embed the production build in the Go server.

---

## 47. Definition of done

ImplCache Librarian is ready for initial release when an administrator can open a browser, connect to an ImplCache server, add and maintain knowledge sources, monitor ingestion, inspect indexed content, manage roots, test retrieval, and resolve health issues without using direct database access or manually constructed MCP calls.

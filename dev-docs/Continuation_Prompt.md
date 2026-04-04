# Continue Prompt: Mock Salesforce Server

**Last Updated:** 2026-04-03 (Session 2)
**Phase:** Feature complete. Docker dev environment + "Back to FalconMode" button added. 363 tests passing across 27 packages.

---

## Current State Summary (VERIFIED 2026-04-03)

**File counts:** 73 Go source files, 41 test files — 363 tests passing in 27 packages

### Architecture Overview

Self-contained Salesforce REST API mock. Provides OAuth2 username-password grant, SOQL query execution with FK relationship traversal, SObject CRUD, and a Salesforce Lightning-style HTML UI — backed by JSON seed files or optional SQLite. Includes a multi-phase LLM-driven data generation pipeline and a standalone `endpoint` ChatClient (zero monorepo dependencies, stdlib-only transport, supports Zai/OpenAI/Anthropic/Ollama).

### Component Health

| Component | Status | Details |
|-----------|--------|---------|
| Mock HTTP Server | COMPLETE | net/http mux, graceful shutdown, `MOCK_USERS` multi-user support, `BASE_PATH` prefix |
| SOQL Engine | COMPLETE | Parser + executor: SELECT, WHERE, ORDER BY, LIMIT, OFFSET, IN, LIKE, NOT, AND/OR, FK traversal |
| Storage Layer | COMPLETE | MemoryStore (default) + SQLiteStore (`-db-path`); lazy seed load if DB empty |
| OAuth2 Auth | COMPLETE | Username-password grant → HMAC-signed Bearer token; session cookie auth for UI |
| Lightning UI | COMPLETE | Case/Account list + detail views at `/lightning/...` paths; session cookie login form |
| ChatClient / endpoint | COMPLETE | Standalone multi-provider client (Zai, OpenAI, Anthropic, Ollama); `ModelRegistry`, streaming, embeddings |
| Data Generation | COMPLETE | 4-phase pipeline (Foundation → Cases → Communications → Jira); SQLite working DB |
| LLM Adapter | COMPLETE | Wraps endpoint.ChatClient; local SQLite semantic cache (cosine similarity at 0.95 threshold) |
| Scenario Overlays | COMPLETE | JSON overlay files in `testdata/scenarios/` loaded over seed data at startup |
| Vendor Profiles | COMPLETE | 6 YAML profiles: Acme Software, FinServ FinCore, Healthcare MedTech, Manufacturing FactoryOS, Retail RetailEdge, SaaS CloudOps |
| Exporter | COMPLETE | `cmd/export/` — SQLite DB → `testdata/seed/` JSON |
| Docker/CI | COMPLETE | `docker/Dockerfile`, Makefile, GCP Artifact Registry push, GitHub Actions build+push |
| Docker Dev Environment | COMPLETE | `docker-compose-dev.yml` + `.air.toml` with hot reload; port 8085 for standalone dev |
| Back to FalconMode Button | COMPLETE | `falcon_return` middleware: server-side cookie, URL validation, renders return button in layout |

### API Surface

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/services/oauth2/token` | Public | Username-password grant → Bearer token |
| `GET` | `/services/data/{version}/query` | Required | SOQL query (`?q=SELECT ...`) |
| `GET` | `/services/data/{version}/sobjects/{type}/{id}` | Required | Fetch record |
| `GET` | `/services/data/{version}/sobjects/{type}/describe` | Required | Object metadata |
| `POST` | `/services/data/{version}/sobjects/{type}` | Required | Create record |
| `PATCH` | `/services/data/{version}/sobjects/{type}/{id}` | Required | Update record |
| `DELETE` | `/services/data/{version}/sobjects/{type}/{id}` | Required | Delete record |
| `GET` | `/health` | Public | Health check |
| `POST` | `/admin/reset` | Bearer only | Reload seed data from disk |
| `GET` | `/` | Public | Login form (multi-user) or redirect to Case list |
| `POST` | `/login` | Public | Session cookie login |
| `GET` | `/lightning/o/Case/list` | Required | Lightning Case list UI |
| `GET` | `/lightning/r/Case/{id}/view` | Required | Lightning Case detail UI |
| `GET` | `/lightning/o/Account/list` | Required | Lightning Account list UI |
| `GET` | `/lightning/r/Account/{id}/view` | Required | Lightning Account detail UI |
| `GET` | `/static/` | Public | Embedded static assets |

API version is configurable (`API_VERSION` env, default `v66.0`). All versioned paths accept any `{version}` segment.

### Key Architecture Decisions

**ChatClient copied standalone (zero monorepo deps)** — `internal/endpoint/` is a self-contained multi-provider LLM client. Implements `ChatClient` interface with provider-specific clients (`client_zai.go`, `client_openai.go`, `client_anthropic.go`, `client_ollama.go`), `ModelRegistry`, streaming support, and embeddings. No falcon-backend imports whatsoever — stdlib transport only.

**LLM adapter decoupled with local SQLite cache** — `internal/llm/adapter.go` wraps `endpoint.ChatClient` and implements `generator.LLM`. Cache uses cosine similarity on Ollama `qwen3-embedding:0.6b` embeddings at 0.95 threshold, stored in `data/llm_cache.db` (SQLite). Replaces `internal/foundation/cache` from monorepo.

**`go_zai_client` absorbed as `internal/zai/`** — The Z.ai Go client was inlined directly into this repo rather than imported as an external dependency.

**All generators included for pipeline completeness; Falcon/Jira exporters excluded** — `internal/generator/` includes shared generators and Jira issue/comment generators (needed for the 4-phase pipeline). The Falcon LocalStore exporter was excluded — this repo only exports to Salesforce seed JSON format.

**Module path: `github.com/falconleon/mock-salesforce`** — Clean standalone module, no `falcondev` monorepo path.

**Auth middleware dual-path** — Bearer tokens are registered in an in-memory set (`mockValidTokens`) when issued. Session cookies use HMAC-SHA256 (`email|hex(hmac(email, secret))`), 8-hour expiry. Admin endpoints (`/admin/`) require Bearer only, not session cookies.

**Store interface polymorphism** — `store.Store` interface with `store.LoadableStore` extension. `MemoryStore` and `SQLiteStore` both implement `LoadableStore`. The `/admin/reset` handler checks for `LoadableStore` before attempting reload.

**falcon_return uses server-side cookie (not localStorage)** — Persistence across tabs and page navigations is handled via an HTTP cookie (`falcon_return`), set by the `FalconReturn` middleware when `?falcon_return=<encoded_url>` is present on any request. URL validated against allowed origins (`*.orb.local`, `localhost`, `127.0.0.1`) to prevent open redirects. The layout template renders a "Back to FalconMode" button when the cookie is present. FalconMode developer adds `?falcon_return=<encoded_url>` to all Salesforce mock links.

### SOQL Support Reference

| Feature | Support Level |
|---------|--------------|
| `SELECT field1, field2` | Full |
| `WHERE =, !=, <, <=, >, >=` | Full |
| `WHERE LIKE '%pattern%'` | Prefix/suffix/contains; exact fallback |
| `WHERE IN (...)` / `NOT IN` | Full |
| `AND` / `OR` / `NOT` | Full (logical tree) |
| `ORDER BY field ASC/DESC` | Full; multi-field; `NULLS FIRST`/`NULLS LAST` |
| `LIMIT N` | Full |
| `OFFSET N` | Full |
| Relationship fields (`Case.Account.Name`) | FK lookup via `relationshipMeta` map |
| Aggregate functions (`COUNT`, `SUM`) | Not implemented |
| `GROUP BY` | Not implemented |
| Subqueries | Not implemented |

**Supported FK relationships** (defined in `relationshipMeta` in `internal/soql/executor.go`):
- `Case` → `Account` (via `AccountId`), `Owner` (via `OwnerId`, resolves User), `CreatedBy` (via `CreatedById`, resolves User)
- `CaseComment` → `CreatedBy` (via `CreatedById`, resolves User)
- `FeedItem` → `CreatedBy` (via `CreatedById`, resolves User)

**Supported object types in seed data:** `Account`, `Contact`, `User`, `Case`, `EmailMessage`, `CaseComment`, `FeedItem`

### Data Generation Pipeline

4-phase pipeline in `internal/pipeline/`:

| Phase | Generators | Output entities |
|-------|-----------|----------------|
| 1 — Foundation | `accounts.go`, `contacts.go`, `users.go` | Accounts, Contacts, Users |
| 2 — Cases | `cases.go` | Cases linked to Account/Contact/Owner |
| 3 — Communications | `emails.go`, `comments.go`, `feed_items.go` | EmailMessages, CaseComments, FeedItems |
| 4 — Jira | `jira_issues.go`, `jira_comments.go` | JiraIssues, JiraComments (for escalated cases) |

Generators take a `Config` with LLM provider + model + counts; they call the `generator.LLM` interface which is satisfied by `internal/llm.Adapter`. All intermediate data is written to SQLite (`data/mock.db`).

**Entry points:**
- `cmd/acme/` — Acme Software scenario, fixed profile
- `cmd/industry/` — Generic industry runner, takes `--profile profiles/*.yaml`
- `cmd/generate/` — Direct pipeline runner
- `cmd/export/` — DB → `testdata/seed/` JSON files

### What's Done (Session 2 — Dev Environment + Back to FalconMode)

- Added Docker Compose dev environment: `docker-compose-dev.yml`, `.air.toml` with hot reload via `air` + Docker Compose Watch
- Port 8085 for standalone dev to avoid conflicts with FalconMode's `sf-mock` container; OrbStack HTTPS via `https://sf-mock.mock-salesforce.orb.local/`
- Added `falcon_return` middleware (`internal/server/middleware/falcon_return.go`): reads `?falcon_return=` query param, validates URL against allowed origins, sets server-side cookie
- Modified `auth.go` middleware, `router.go`, `salesforce.css`, `layout.html`, `login.html` to integrate the return button
- Added `docs/falcon-return-implementation-guide.md` for mock-Jira developer to implement the same pattern
- Added `docs/superpowers/specs/2026-04-03-back-to-falconmode-design.md`
- Updated `.env`: PORT=8085, INSTANCE_URL=localhost:8085, added ZAI_API_KEY and OPENROUTER_API_KEY
- Docker image rebuilt and pushed to GCP Artifact Registry; 363 tests passing (26 new tests for falcon_return middleware)

### What's Done (Session 1 — Extraction)

- Initialized repo from scratch as standalone Go module
- Ported Salesforce mock HTTP server from falcon-backend monorepo (`internal/server/`, `internal/handlers/`, `internal/soql/`, `internal/store/`, `internal/config/`)
- Copied `endpoint` ChatClient package as standalone (zero monorepo deps)
- Inlined Z.ai client as `internal/zai/`
- Added `internal/llm/` adapter with local SQLite semantic cache
- Added `internal/generator/`, `internal/pipeline/`, `internal/scenario/`, `internal/profile/`, `internal/exporter/`, `internal/db/`
- Created 6 vendor YAML profiles in `profiles/`
- Seeded `testdata/seed/` with Acme Software data (accounts, contacts, users, cases, emails, comments, feed items)
- Added `cmd/` binaries: `salesforce-mock`, `acme`, `industry`, `generate`, `generate_photos`, `export`, `pipeline`
- Added `docker/Dockerfile`, `Makefile`, `.env.example`, `config.yaml`
- Added GitHub Actions workflow for build + push to GCP Artifact Registry
- Added comprehensive docs: `docs/ARCHITECTURE.md`, `docs/CLI_REFERENCE.md`, `docs/PLAN.md`, `docs/TUTORIAL.md`, `docs/README.md`, `docs/mock-data-generation-prompt.md`
- 337 tests passing across 27 packages

### What's Queued / Next Steps

1. **Waiting: FalconMode developer** — needs to add `?falcon_return=<encoded_url>` to all Salesforce mock links so the "Back to FalconMode" button activates. Implementation guide at `docs/falcon-return-implementation-guide.md`. Mock-Jira developer also has their own copy.
2. **Iterate on LLM prompts** — improve realism of generated Salesforce data (see `docs/mock-data-generation-prompt.md` for prompt templates)
2. **Add SOQL aggregate functions** — `COUNT()`, `SUM()`, `AVG()`, `GROUP BY` for analytics queries from falcon-backend
3. **Expand SObject types** — `Opportunity`, `Lead`, `Task`, `Event`, `Contract` — currently only 7 types seeded
4. **Add Bulk API simulation** — `POST /services/async/{version}/jobs/ingest` for large data loads
5. **Add Chatter API endpoints** — `/services/data/{version}/chatter/feeds/...` for FeedItem queries
6. **Scenario overlay improvements** — currently scenarios are loaded over seed data at startup; add `POST /admin/scenario/{name}` for hot-swap
7. **Wire `internal/zai/` into endpoint package** — currently `client_zai.go` in endpoint may duplicate zai client logic
8. **Profile validation tests** — `profiles/validate_profiles_test.go` exists; extend for new profile fields
9. **Contact photo generation** — `cmd/generate_photos/` exists but integration with main pipeline unclear
10. **Push Docker image** — `make docker-push` target configured for GCP Artifact Registry; confirm registry credentials in CI

### Key Architecture Files

| File | Purpose |
|------|---------|
| `cmd/salesforce-mock/main.go` | Server entry point; flag parsing, store selection, graceful shutdown |
| `internal/server/router.go` | Route registration, middleware chain assembly |
| `internal/server/middleware/auth.go` | Auth middleware: Bearer token + HMAC session cookie dual-path |
| `internal/soql/executor.go` | SOQL execution: filter building, FK resolution, field projection |
| `internal/soql/parser.go` | SOQL lexer/parser → `SelectStatement` AST |
| `internal/store/memory.go` | In-memory store with type-keyed record maps |
| `internal/store/sqlite.go` | SQLite-backed store; lazy seed on empty DB |
| `internal/store/loader.go` | JSON seed file loader: filename → object type mapping |
| `internal/endpoint/chat_client.go` | Multi-provider ChatClient interface + factory |
| `internal/endpoint/client_zai.go` | Z.ai provider implementation (streaming + embeddings) |
| `internal/llm/adapter.go` | LLM adapter: wraps ChatClient, adds semantic cache |
| `internal/llm/cache.go` | SQLite semantic cache: cosine similarity on embeddings |
| `internal/pipeline/pipeline.go` | 4-phase generation orchestrator |
| `internal/generator/cases.go` | LLM-driven Case generator (largest, 14.4K) |
| `internal/scenario/acme.go` | Acme Software scenario definition |
| `internal/profile/profile.go` | YAML vendor profile loader |
| `profiles/acme_software.yaml` | Primary demo profile (18.8K, most complete) |
| `testdata/seed/cases.json` | Primary seed data (73.8K, ~50 cases) |
| `docker/Dockerfile` | Production image |
| `docker/Dockerfile.dev` | Dev image with `air` for hot reload |
| `docker-compose-dev.yml` | Dev Compose file with hot reload + Docker Compose Watch |
| `.air.toml` | Air hot-reload config for Go binary |
| `internal/server/middleware/falcon_return.go` | falcon_return cookie middleware; URL validation, cookie set/read |
| `docs/falcon-return-implementation-guide.md` | Guide for mock-Jira developer to implement the same pattern |
| `Makefile` | Build, test, run, docker targets |

### Configuration Reference

| Env Variable | Default | Flag Equivalent | Description |
|-------------|---------|-----------------|-------------|
| `PORT` | `8080` | `-port` | Server listen port |
| `LOG_LEVEL` | `info` | `-log-level` | `debug`, `info`, `warn`, `error` |
| `AUTH_ENABLED` | `true` | `-auth` | Enable OAuth token validation |
| `SEED_DATA_PATH` | `./testdata/seed` | `-seed` | Directory of seed JSON files |
| `MOCK_CLIENT_ID` | `mock-client-id` | — | OAuth2 client ID |
| `MOCK_CLIENT_SECRET` | `mock-client-secret` | — | OAuth2 client secret |
| `MOCK_USERNAME` | `demo@falcon.local` | — | Single-user OAuth username |
| `MOCK_PASSWORD` | `demo123` | — | Single-user OAuth password |
| `MOCK_USERS` | _(empty)_ | `-mock-users` | Multi-user: `email:pass,email:pass,...` |
| `SESSION_SECRET` | `sf-mock-dev-secret` | `-session-secret` | HMAC key for session cookies |
| `API_VERSION` | `v66.0` | — | Salesforce API version returned in responses |
| `INSTANCE_URL` | `http://localhost:8080` | — | OAuth response instance_url |
| `BASE_PATH` | _(empty)_ | `-base-path` | URL prefix for all template links (e.g., `/salesforce`) |
| `ZAI_API_KEY` | — | — | Z.ai API key for data generation |
| `OPENAI_API_KEY` | — | — | OpenAI API key for data generation |
| `ANTHROPIC_API_KEY` | — | — | Anthropic API key for data generation |
| `OLLAMA_HOST` | — | — | Ollama host for data generation |
| `FALCON_RETURN_ALLOWED_ORIGINS` | `*.orb.local,localhost,127.0.0.1` | — | Allowed origin patterns for `falcon_return` URL validation |
| `OPENROUTER_API_KEY` | — | — | OpenRouter API key for data generation |

**Docker image:** `us-west1-docker.pkg.dev/farsipractice/farsipractice/falcon_demo_sf_mock`

---

## Previous Sessions

**Session 2 (2026-04-03) — Docker dev environment + Back to FalconMode button:**
Added `docker-compose-dev.yml` + `.air.toml` for hot-reload dev (port 8085, OrbStack HTTPS). Implemented `falcon_return` middleware: reads `?falcon_return=` param, validates against allowed origins, stores as server-side cookie, renders "Back to FalconMode" button in layout. Updated auth middleware, router, CSS, templates. Wrote implementation guide for mock-Jira developer. Docker image rebuilt and pushed. 363 tests (+26 for falcon_return). 2 commits.

**Session 1 (2026-04-03) — Initial extraction from falcon-backend monorepo:**
Initialized standalone `github.com/falconleon/mock-salesforce` module. Ported Salesforce mock server, SOQL engine, and storage layer from falcon-backend. Added standalone `endpoint` ChatClient (zero monorepo deps), inlined Z.ai client as `internal/zai/`, added LLM adapter with local SQLite semantic cache. Full data generation pipeline: 4 phases, 8 generator types, 6 vendor YAML profiles. 7 `cmd/` binaries, Docker + CI, comprehensive docs. 337 tests, 7 commits.

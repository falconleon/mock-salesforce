# Mock Salesforce — Project State

**Last Updated:** 2026-05-04
**Phase:** OAuth conformance + PKCE/RTR shipped (PR #1); FalconMode design system applied (PR #2); fully deployable. Favicon fix complete but uncommitted on `main`.

---

## Current State Summary

- **Branch:** `main` (HEAD `a36dda6`)
- **Uncommitted changes (5 modified, ~10 untracked):**
  - Modified: `internal/server/middleware/auth.go`, `internal/server/router.go`, `internal/server/templates/layout.html`, `internal/server/templates/login.html`, `.claude/settings.json`
  - New file: `internal/server/static/favicon.svg`
  - Untracked runtime: `.beads/` lock/pid/port files, `.intent/`
- **Go source files:** 93 (excl. vendor, excl. test files)
- **Test files:** 56
- **Beads issues:** 0 total (tracker initialized but empty)
- **Health:** `go build`, `go vet`, `go test ./... -short` all clean; `docker build` green; container starts and loads full seed (12 accounts, 50 cases, 25 emails, 16 case comments, 15 feed items); `/health` returns 200.

---

## Architecture Health

| Session / Date | Area | Status | Notes |
|---|---|---|---|
| S5 · 2026-05-04 | Favicon route | ADDED | `/favicon.ico` public route + SVG; resolves console 401/404 noise |
| S5 · 2026-05-04 | Deploy verification | VERIFIED | go build/vet/test clean; docker build + run green; Playwright login + cases + accounts pass |
| S4 · ~2026-05-01 | FalconMode design system | COMPLETE | Brand tokens (`#1A3A6D` deep blue), a11y skip-links, ARIA landmarks, keyboard nav, contrast (PR #2) |
| S3 · ~2026-04-30 | OAuth `authorization_code` + PKCE | COMPLETE | `/authorize`, code-for-token exchange, PKCE `code_challenge`, refresh-token rotation, redirect_uri allowlist (PR #1) |
| S3 · ~2026-04-30 | OIDC discovery + token endpoints | COMPLETE | `/.well-known/openid-configuration`, `/introspect`, `/userinfo`, `/revoke` |
| S3 · ~2026-04-30 | SOQL Playground UI | COMPLETE | `/playground` page exercises same executor used by the API |
| S3 · ~2026-04-30 | Admin / Settings user CRUD | COMPLETE | `/admin/users` + `/settings/users` with token issuance |
| S2 · 2026-04-03 | Back to FalconMode button | COMPLETE | `falcon_return` middleware + server-side cookie + URL validation |
| S2 · 2026-04-03 | Docker dev environment | COMPLETE | `docker-compose-dev.yml` + `.air.toml` hot-reload; port 8085; OrbStack HTTPS |
| S1 · 2026-04-03 | Standalone Go module | COMPLETE | `github.com/falconleon/mock-salesforce` — extracted from falcon-backend monorepo |

---

## Recent Sessions

### Session 5 (2026-05-04) — Code review, deploy verification, favicon fix

**Code review + deploy:**
- `go build`, `go vet`, `go test ./... -short` all pass across the 93-file / 56-test-file codebase.
- `docker build -f docker/Dockerfile` succeeds; container starts cleanly and loads full seed: 12 accounts, 50 cases, 25 emails, 16 case comments, 15 feed items.
- `/health` returns 200; OAuth password grant + SOQL query verified via `curl`.
- Playwright smoke: login → home → `/lightning/o/Case/list` (50 cases rendered) → case detail (Emails(7)/Comments(4)/Feed(5)/Activities(2)/Files(0) tabs all populated) → `/lightning/o/Account/list` (12 accounts). Zero console errors or warnings.

**Favicon fix (uncommitted):**
Browser's automatic `/favicon.ico` request was blocked by auth middleware (401 when logged out, 404 when authenticated), polluting logs and Playwright output.

Changes made:
- `internal/server/static/favicon.svg` — 32×32 FalconMode cloud mark on brand `#1A3A6D` background; embedded via existing `//go:embed static/*`.
- `internal/server/router.go` — added `GET /favicon.ico` route serving the SVG with `image/svg+xml` content-type and 1-day cache (`Cache-Control: public, max-age=86400`).
- `internal/server/middleware/auth.go` — added `/favicon.ico` to `publicPaths` so the route is accessible unauthenticated.
- `internal/server/templates/layout.html` + `templates/login.html` — added `<link rel="icon" type="image/svg+xml" href=".../static/favicon.svg">` so modern browsers fetch via the already-public `/static/` route instead.
- Verified: build clean, all tests still pass, `curl -i /favicon.ico` returns 200 unauthenticated, Playwright reports 0 errors / 0 warnings on `/login` and `/home`.
- **Status: uncommitted on `main`.**

---

### Session 4 (~2026-05-01) — FalconMode design system + a11y (PR #2 `a36dda6`)

Full design system pass across all templates and CSS (Waves 9–11):

- **Brand color tokens** — `#1A3A6D` (deep blue primary), `#F5F7FA` (light surface), `#2E7D32` (success green), `#C62828` (error red), consistent throughout `salesforce.css`.
- **Skip-link** — `<a class="skip-link" href="#main-content">Skip to main content</a>` prepended to every page layout for keyboard users.
- **ARIA landmarks** — `<main id="main-content">`, `<nav aria-label="...">`, `<header role="banner">` applied across all Lightning UI templates.
- **Keyboard navigation** — interactive elements confirmed tab-reachable; focus styles made visible.
- **Contrast tightening** — text/background pairs adjusted to meet WCAG AA (4.5:1 minimum) — notably header text on brand backgrounds.
- No new Go files; changes were CSS + HTML template only.

---

### Session 3 (~2026-04-30) — OAuth conformance + PKCE/RTR mandate prep (PR #1 `79fe6e7`)

Full OAuth 2.0 authorization_code flow + PKCE implemented ahead of the 2026-05-11 mandate:

- **`GET /services/oauth2/authorize`** — renders consent screen; validates `client_id`, `redirect_uri` against `MOCK_REDIRECT_URIS` allowlist, `response_type=code`, optional PKCE `code_challenge`/`code_challenge_method=S256`.
- **`POST /services/oauth2/token` (authorization_code grant)** — exchanges code for `access_token` + `refresh_token`; verifies `code_verifier` when PKCE was used.
- **Refresh-token rotation (RTR)** — each use of `grant_type=refresh_token` issues a new `refresh_token` and invalidates the old one; replay of a rotated token returns `invalid_grant`.
- **`MOCK_REDIRECT_URIS`** env var — comma-separated allowlist of valid callback URLs (mirrors Salesforce Connected App "Callback URL" list). Defaults to `http://localhost:1717/OauthRedirect,http://localhost:8080/callback`. Set empty to disable (permissive mode with WARN log).
- **`MOCK_PUBLIC_BASE_URL`** — override base URL for OIDC discovery doc; ignores `X-Forwarded-*` headers when set. Defaults to request `Host`.
- **`GET /.well-known/openid-configuration`** — full OIDC discovery document (`issuer`, `authorization_endpoint`, `token_endpoint`, `userinfo_endpoint`, `jwks_uri`, `revocation_endpoint`, `introspection_endpoint`).
- **`POST /services/oauth2/introspect`** — RFC 7662 token introspection (`active`, `sub`, `exp`, `scope`).
- **`GET /services/oauth2/userinfo`** — OIDC userinfo (`sub`, `email`, `name`).
- **`POST /services/oauth2/revoke`** — token revocation (access + refresh).
- **SOQL Playground UI** — `/playground` page with query input, response display, exercising the same executor used by the REST API.
- **Admin/Settings user CRUD** — `/admin/users` (Bearer-only) and `/settings/users` (session) for creating/listing users and issuing tokens.
- **`BASE_URL` / `BASE_PATH` plumbing** — `instance_url` in OAuth token response now respects both env vars; `BASE_PATH` strips prefix from incoming request paths.
- All changes in PR #1 plus small follow-up commits: `6985237` (gitignore binary), `144e232` (BASE_URL), `3a8b638` (BASE_PATH in instance_url), `6a8c394` (strip BASE_PATH from requests), `1b8d9cf` (admin endpoints unauthenticated for demo controller).

---

### Session Blocks (consolidated)

**Sessions 1–2 (2026-04-03):** Initial extraction from falcon-backend monorepo + Docker dev environment.

Session 1 — Initialized standalone `github.com/falconleon/mock-salesforce` module. Ported Salesforce mock HTTP server, SOQL engine, and storage layer from falcon-backend. Added standalone `endpoint` ChatClient (zero monorepo deps, stdlib-only transport, supports Zai/OpenAI/Anthropic/Ollama). Inlined Z.ai client as `internal/zai/`. Added LLM adapter with local SQLite semantic cache (cosine similarity, 0.95 threshold). Full 4-phase data generation pipeline, 8 generator types, 6 vendor YAML profiles. 7 `cmd/` binaries, Docker + CI + GitHub Actions. 337 tests.

Session 2 — Docker Compose dev environment (`docker-compose-dev.yml` + `.air.toml`), port 8085, OrbStack HTTPS (`https://sf-mock.mock-salesforce.orb.local/`). `falcon_return` middleware: reads `?falcon_return=` query param, validates URL against allowed origins (`*.orb.local`, `localhost`, `127.0.0.1`), stores as server-side cookie, renders "Back to FalconMode" button in layout. Updated auth middleware, router, CSS, and templates. Wrote implementation guide at `docs/falcon-return-implementation-guide.md`. 363 tests (+26 for falcon_return middleware).

---

## Worktree Layout

| Path | Branch | Agent |
|---|---|---|
| `/Users/leon/dev/falcondev/mock-salesforce` | `main` | `coord_mock-salesforce` |
| `/Users/leon/dev/falcondev/mock-salesforce/.git/thrum-sync/a-sync` | `a-sync` | thrum sync (internal) |
| `/Users/leon/intent/workspaces/read-continuation-2/repo` | `ui-design-harmonization` | intent workspace |

---

## Open Epics / Active Work

No open epics or issues in beads (tracker initialized at `0e8a3fc`, currently empty).

---

## What's Queued / Next Steps

1. **Commit favicon fix** — 5 modified files + `internal/server/static/favicon.svg` are uncommitted on `main`. Commit message should reference the auth middleware public-path addition.

2. **falcon-backend integration** — now that the PKCE/RTR flow has shipped (2026-05-11 mandate), confirm falcon-backend is updated to drive `authorization_code` + PKCE against this mock. Ensure falcon-backend's OAuth callback URL is listed in `MOCK_REDIRECT_URIS`.

3. **SOQL aggregate functions** — `COUNT()`, `SUM()`, `AVG()`, `GROUP BY` not yet implemented (`internal/soql/executor.go`). Needed for analytics queries from falcon-backend.

4. **Expand SObject types** — currently 7 types seeded (`Account`, `Contact`, `User`, `Case`, `EmailMessage`, `CaseComment`, `FeedItem`). Add: `Opportunity`, `Lead`, `Task`, `Event`, `Contract`.

5. **Bulk API simulation** — `POST /services/async/{version}/jobs/ingest` for large data loads.

6. **Chatter API endpoints** — `/services/data/{version}/chatter/feeds/...` for FeedItem queries.

7. **LLM prompt iteration** — improve realism of generated data; see `docs/mock-data-generation-prompt.md` for templates.

8. **Scenario hot-swap** — `POST /admin/scenario/{name}` for hot-loading overlay scenarios without restart.

9. **Drop `dev-docs/Continuation_Prompt.md`** — this file is now superseded by `project_state.md`. Flag for user to delete or archive; do not delete without explicit approval.

---

## Key Architecture Files

| File | Purpose |
|---|---|
| `cmd/salesforce-mock/main.go` | Server entry point; flag parsing, store selection, graceful shutdown |
| `internal/server/router.go` | Route + middleware assembly; `GET /favicon.ico` added S5 |
| `internal/server/middleware/auth.go` | Bearer + session-cookie dual-path auth; `publicPaths` list |
| `internal/server/middleware/falcon_return.go` | Back-to-FalconMode cookie; URL validation against allowed origins |
| `internal/handlers/oauth.go` | OAuth token endpoint (password grant, authorization_code, refresh, RTR) |
| `internal/handlers/authorize.go` | `/services/oauth2/authorize` — PKCE consent screen + code issuance |
| `internal/handlers/authcodes.go` | In-memory authorization code store with PKCE verifier storage |
| `internal/handlers/discovery.go` | `/.well-known/openid-configuration`, `/introspect`, `/userinfo`, `/revoke` |
| `internal/handlers/admin_users.go` | Admin user CRUD + token issuance (`/admin/users`) |
| `internal/server/settings.go` | Settings page handler |
| `internal/server/settings_users.go` | `/settings/users` — session-authenticated user CRUD |
| `internal/server/playground.go` | `/playground` — SOQL query UI |
| `internal/server/templates/` | All HTML templates (layout, login, lightning views, playground, settings) |
| `internal/server/static/salesforce.css` | FalconMode design tokens + Lightning UI styles |
| `internal/server/static/favicon.svg` | 32×32 FalconMode cloud mark on `#1A3A6D` background (added S5) |
| `internal/soql/parser.go` | SOQL lexer/parser → `SelectStatement` AST |
| `internal/soql/executor.go` | SOQL execution: filter building, FK resolution, field projection |
| `internal/store/memory.go` | In-memory store (default) |
| `internal/store/sqlite.go` | SQLite-backed store; lazy seed load on empty DB |
| `internal/store/loader.go` | JSON seed file loader: filename → object type mapping |
| `internal/endpoint/chat_client.go` | Multi-provider ChatClient interface + factory |
| `internal/llm/adapter.go` | LLM adapter: wraps ChatClient, adds semantic cache |
| `internal/llm/cache.go` | SQLite semantic cache (cosine similarity on embeddings, 0.95 threshold) |
| `internal/pipeline/pipeline.go` | 4-phase generation orchestrator |
| `profiles/*.yaml` | 6 vendor YAML profiles (Acme Software, FinServ, Healthcare, etc.) |
| `testdata/seed/*.json` | Default seed data (12 accounts, 50 cases, etc.) |
| `docker/Dockerfile` | Production image |
| `docker-compose-dev.yml` | Dev Compose with hot-reload (port 8085, OrbStack HTTPS) |
| `.air.toml` | Air hot-reload config |
| `Makefile` | Build, test, run, docker targets |

---

## Configuration Reference

Non-obvious env vars added or changed in Sessions 3-5; see README for full list including provider API keys and `PORT`/`LOG_LEVEL`.

| Variable | Default | Description |
|---|---|---|
| `MOCK_USERS` | _(empty)_ | Multi-user list: `email:pass,email:pass,...` — enables login form at `GET /` |
| `MOCK_REDIRECT_URIS` | `http://localhost:1717/OauthRedirect,http://localhost:8080/callback` | Allowlist for `authorization_code` flow `redirect_uri`. Set empty to disable (permissive WARN). |
| `MOCK_PUBLIC_BASE_URL` | _(empty)_ | Override base URL in OIDC discovery doc; ignores `X-Forwarded-*` when set. |
| `BASE_PATH` | _(empty)_ | URL prefix for all template links; stripped from incoming request paths (e.g., `/salesforce`). |
| `SESSION_SECRET` | `sf-mock-dev-secret` | HMAC key for session cookies (8-hour expiry). |
| `AUTH_ENABLED` | `true` | Disable to allow unauthenticated API access (local debugging only). |
| `INSTANCE_URL` | `http://localhost:8080` | Returned in OAuth `instance_url` field; honours `BASE_PATH`. |
| `FALCON_RETURN_ALLOWED_ORIGINS` | `*.orb.local,localhost,127.0.0.1` | Allowed origin patterns for `falcon_return` URL validation (open-redirect guard). |

---

## Notes

The older `dev-docs/Continuation_Prompt.md` (Session 2, 2026-04-03) is now superseded by this file. Sessions 3–5 were added by reconciling the git log against that document; exact dates for Sessions 3–4 are approximated (`~2026-04-30` / `~2026-05-01`) because the commits for PR #1 and PR #2 do not carry authorship timestamps in the oneline log — use `git log --format="%H %ai %s"` if exact dates are needed.

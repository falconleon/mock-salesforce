# mock-salesforce

A self-contained Salesforce REST API mock server for development, testing, and demos. Implements OAuth2 username-password grant, SOQL query execution, SObject CRUD, and a Salesforce Lightning-style UI — all backed by JSON seed files or an optional SQLite database.

## Features

- **SOQL engine** — parses and executes SELECT queries with WHERE, ORDER BY, LIMIT, OFFSET, IN, LIKE, and relationship field traversal (e.g., `Owner.Name`)
- **OAuth2** — username-password grant flow returning signed Bearer tokens; supports multiple users via `MOCK_USERS`
- **SObject CRUD** — GET, POST, PATCH, DELETE on any object type loaded into the store
- **Lightning UI** — Case list and detail views, Account list and detail views at `/lightning/...` paths
- **Seed data** — loads JSON files from `testdata/seed/` on startup; reloadable via `POST /admin/reset`
- **SQLite persistence** — optional `-db-path` flag for durable storage across restarts
- **Data generation pipeline** — LLM-driven pipeline to generate realistic Salesforce data

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/services/oauth2/token` | OAuth2 token (username-password grant) |
| `GET` | `/services/data/{version}/query` | SOQL query (`?q=SELECT ...`) |
| `GET` | `/services/data/{version}/sobjects/{type}/{id}` | Fetch a record |
| `GET` | `/services/data/{version}/sobjects/{type}/describe` | Object metadata |
| `POST` | `/services/data/{version}/sobjects/{type}` | Create a record |
| `PATCH` | `/services/data/{version}/sobjects/{type}/{id}` | Update a record |
| `DELETE` | `/services/data/{version}/sobjects/{type}/{id}` | Delete a record |
| `GET` | `/health` | Health check |
| `POST` | `/admin/reset` | Reload seed data |
| `GET` | `/lightning/o/Case/list` | Case list UI |
| `GET` | `/lightning/r/Case/{id}/view` | Case detail UI |
| `GET` | `/lightning/o/Account/list` | Account list UI |
| `GET` | `/lightning/r/Account/{id}/view` | Account detail UI |

## Authentication

The server supports two authentication methods:

**OAuth2 Bearer token** — use the username-password grant:

```bash
curl -X POST http://localhost:8080/services/oauth2/token \
  -d "grant_type=password" \
  -d "client_id=mock-client-id" \
  -d "client_secret=mock-client-secret" \
  -d "username=demo@falcon.local" \
  -d "password=demo123"
```

Returns `{"access_token": "...", "instance_url": "...", "token_type": "Bearer"}`. Include the token in subsequent requests:

```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/services/data/v66.0/query?q=SELECT+Id,Subject+FROM+Case"
```

**Session cookies** — when `MOCK_USERS` is set, the server renders a login form at `GET /`. Successful login sets an HMAC-signed session cookie valid for subsequent UI and API requests.

## SOQL Support

The built-in SOQL engine handles:

| Feature | Example |
|---------|---------|
| Field selection | `SELECT Id, Subject, Status FROM Case` |
| WHERE with `=`, `!=`, `<`, `<=`, `>`, `>=` | `WHERE Status = 'Open'` |
| `LIKE` pattern matching | `WHERE Subject LIKE '%login%'` |
| `IN` / `NOT IN` | `WHERE Status IN ('Open', 'Working')` |
| `AND` / `OR` / `NOT` | `WHERE Status = 'Open' AND Priority = 'High'` |
| `ORDER BY` with `ASC`/`DESC`, `NULLS FIRST`/`LAST` | `ORDER BY CreatedDate DESC` |
| `LIMIT` and `OFFSET` | `LIMIT 10 OFFSET 20` |
| Relationship field traversal | `SELECT Id, Owner.Name FROM Case` |

Relationship traversal supports: `Case.Account`, `Case.Owner`, `Case.CreatedBy`, `CaseComment.CreatedBy`, `FeedItem.CreatedBy`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `AUTH_ENABLED` | `true` | Enable OAuth token validation |
| `SEED_DATA_PATH` | `./testdata/seed` | Directory containing seed JSON files |
| `MOCK_CLIENT_ID` | `mock-client-id` | OAuth client ID |
| `MOCK_CLIENT_SECRET` | `mock-client-secret` | OAuth client secret |
| `MOCK_USERNAME` | `demo@falcon.local` | Single-user OAuth username |
| `MOCK_PASSWORD` | `demo123` | Single-user OAuth password |
| `MOCK_USERS` | _(empty)_ | Multi-user list: `email:pass,email:pass,...` |
| `SESSION_SECRET` | `sf-mock-dev-secret` | HMAC key for session cookies |
| `API_VERSION` | `v66.0` | Salesforce API version |
| `INSTANCE_URL` | `http://localhost:8080` | Returned in OAuth response |
| `BASE_PATH` | _(empty)_ | URL prefix for all template links |

## Data Generation

The repo includes an LLM-driven pipeline that generates realistic Salesforce seed data.

### Quick start

```bash
cp .env.example .env
# Edit .env to set ZAI_API_KEY or OPENAI_API_KEY

# Generate data using the Acme Software profile
go run ./cmd/acme/ --reset --full

# Export to testdata/seed/
go run ./cmd/export/

# Or run a custom industry profile
go run ./cmd/industry/ --profile profiles/acme_software.yaml --export
```

### Generated entities

- **Accounts** — companies with industry, billing address, annual revenue
- **Contacts** — contacts linked to accounts
- **Users** — support team members with roles (Manager, L1/L2/L3 Support)
- **Cases** — support cases with status, priority, product, origin
- **Email messages** — threaded email conversations per case
- **Case comments** — internal notes and public replies
- **Feed items** — Chatter-style activity posts
- **Jira issues/comments** — linked engineering issues for escalated cases

### Pipeline phases

| Phase | What it generates |
|-------|------------------|
| 1 | Foundation: accounts, contacts, users |
| 2 | Cases linked to accounts/contacts/users |
| 3 | Communications: emails, comments, feed items |
| 4 | Jira issues and comments for escalated cases |

### LLM providers

| Provider | Env var | Default model |
|----------|---------|---------------|
| Z.ai | `ZAI_API_KEY` | `GLM-4.7` |
| OpenAI | `OPENAI_API_KEY` | `gpt-4o-mini` |
| Anthropic | `ANTHROPIC_API_KEY` | `claude-3-haiku-20240307` |
| Ollama | `OLLAMA_HOST` | `llama3.2` |

## Docker

```bash
# Build image
docker build -f docker/Dockerfile -t salesforce-mock .

# Run
docker run -p 8080:8080 \
  -e AUTH_ENABLED=true \
  -e MOCK_USERS="analyst@falcon.local:analyst-pass,operator@falcon.local:operator-pass" \
  -e SESSION_SECRET=change-me \
  salesforce-mock
```

Using Make:

```bash
make docker-build
make docker-push   # pushes to GCP Artifact Registry
```

## Development

```bash
# Run tests
make test

# Build
make build

# Run locally
make run

# Start with debug logging
go run ./cmd/salesforce-mock/ -port 8080 -log-level debug -auth=false
```

### Seed data format

Seed files are JSON arrays placed in `testdata/seed/`. The file name determines the object type:

| File | Object type |
|------|-------------|
| `accounts.json` | `Account` |
| `contacts.json` | `Contact` |
| `users.json` | `User` |
| `cases.json` | `Case` |
| `email_messages.json` | `EmailMessage` |
| `case_comments.json` | `CaseComment` |
| `feed_items.json` | `FeedItem` |

All records use Salesforce field naming conventions (`Id`, `CreatedDate`, `OwnerId`, etc.).

### Project structure

```
cmd/
  salesforce-mock/    # Main server binary
  acme/               # Acme Software scenario generator
  industry/           # Generic industry scenario generator
  generate/           # Pipeline-based generator
  generate_photos/    # Contact photo generator
  export/             # Export DB to seed JSON files
  pipeline/           # Pipeline runner (WIP)
internal/
  config/             # Server configuration
  handlers/           # HTTP handlers (oauth, query, sobject, health)
  server/             # Router, middleware, UI handlers
  soql/               # SOQL parser and executor
  store/              # In-memory and SQLite stores
  db/                 # SQLite schema for data generation
  generator/          # LLM-driven entity generators
  pipeline/           # Orchestrates generation phases
  scenario/           # Scenario definitions (Acme, industry)
  profile/            # Vendor profile loader (YAML)
  exporter/           # DB to JSON seed exporter
  llm/                # LLM adapter (decoupled from monorepo)
  endpoint/           # Multi-provider LLM chat client
  zai/                # Z.ai Go client
pkg/
  models/             # Shared Salesforce data models
profiles/             # YAML vendor profiles
testdata/seed/        # Default seed data
```

# Mock Data Generation - Architecture Plan

## Overview

Central mock data generation system that populates a SQLite database with realistic
customer support interaction data, then exports it into the format required by each
mock API (Salesforce, JIRA, future integrations).

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                  mock_data_generation/                            │
│                                                                  │
│  ┌─────────┐     ┌────────────┐     ┌──────────┐               │
│  │  z.ai   │────▶│ Generators │────▶│  SQLite  │               │
│  │ GLM4.7  │     │            │     │    DB    │               │
│  │(endpoint│     │ accounts   │     │          │               │
│  │ module) │     │ contacts   │     │ data/    │               │
│  └─────────┘     │ users      │     │ mock.db  │               │
│                  │ cases      │     └────┬─────┘               │
│                  │ emails     │          │                       │
│                  │ comments   │          │                       │
│                  │ feed_items │     ┌────▼─────┐               │
│                  │ jira_*     │     │Exporters │               │
│                  └────────────┘     │          │               │
│                                     │salesforce│──▶ SF seed/   │
│                                     │jira      │──▶ JIRA seed/ │
│                                     │(future)  │──▶ ...        │
│                                     └──────────┘               │
└──────────────────────────────────────────────────────────────────┘
```

## Data Flow

1. **Generate**: LLM (z.ai GLM4.7 via endpoint module) produces realistic content
2. **Store**: Generators write structured records into SQLite
3. **Export**: Exporters read from SQLite and write JSON in each mock API's format
4. **Consume**: Mock APIs load JSON seed files at startup (no changes to mock APIs)

## Key Design Decisions

### SQLite as Canonical Store

- Single source of truth for all mock data
- Queryable for validation and debugging
- Supports incremental generation (add data without regenerating everything)
- Committed to the repo (not sensitive data)
- Efficient storage and fast reads

### Generators Don't Build Their Own LLM Client

The z.ai GLM4.7 integration is provided by the existing endpoint module.
Generators receive an interface for LLM calls, keeping them decoupled from
the specific LLM provider.

### Exporters Are Format Adapters

Each exporter reads from SQLite and produces JSON matching the target mock API's
schema. Adding a new mock API = writing a new exporter. No changes to generators
or the database.

### Data Lives in the Repo

Generated data is committed alongside the code. The SQLite DB file and exported
JSON files are not sensitive (fictional companies, names, interactions).

## Database Schema

### Core Tables

```sql
-- Companies / Organizations
CREATE TABLE accounts (
    id              TEXT PRIMARY KEY,   -- Salesforce-style 18-char ID
    name            TEXT NOT NULL,
    industry        TEXT NOT NULL,
    type            TEXT NOT NULL,      -- Enterprise, Mid-Market, SMB
    website         TEXT,
    phone           TEXT,
    billing_city    TEXT,
    billing_state   TEXT,
    annual_revenue  REAL,
    num_employees   INTEGER,
    created_at      TEXT NOT NULL       -- ISO 8601
);

-- Customer contacts
CREATE TABLE contacts (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL,
    phone           TEXT,
    title           TEXT,
    department      TEXT,
    created_at      TEXT NOT NULL
);

-- Support agents / internal users
CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL,
    username        TEXT NOT NULL,
    title           TEXT,
    department      TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1,
    manager_id      TEXT REFERENCES users(id),
    user_role       TEXT,              -- L1, L2, L3, Manager, TAM
    created_at      TEXT NOT NULL
);

-- Support cases / tickets
CREATE TABLE cases (
    id              TEXT PRIMARY KEY,
    case_number     TEXT NOT NULL UNIQUE,
    subject         TEXT NOT NULL,
    description     TEXT NOT NULL,
    status          TEXT NOT NULL,      -- New, In Progress, Escalated, Pending Customer, Closed
    priority        TEXT NOT NULL,      -- P0, P1, P2, P3
    product         TEXT,
    case_type       TEXT,              -- Problem, Question, Feature Request, Incident
    origin          TEXT,              -- Email, Phone, Web, Chat
    reason          TEXT,
    owner_id        TEXT NOT NULL REFERENCES users(id),
    contact_id      TEXT NOT NULL REFERENCES contacts(id),
    account_id      TEXT NOT NULL REFERENCES accounts(id),
    created_at      TEXT NOT NULL,
    closed_at       TEXT,
    is_closed       INTEGER NOT NULL DEFAULT 0,
    is_escalated    INTEGER NOT NULL DEFAULT 0,
    -- Optional link to JIRA
    jira_issue_key  TEXT
);

-- Email messages on cases
CREATE TABLE email_messages (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL REFERENCES cases(id),
    subject         TEXT NOT NULL,
    text_body       TEXT NOT NULL,
    html_body       TEXT,
    from_address    TEXT NOT NULL,
    from_name       TEXT NOT NULL,
    to_address      TEXT NOT NULL,
    cc_address      TEXT,
    bcc_address     TEXT,
    message_date    TEXT NOT NULL,
    status          TEXT NOT NULL,      -- New, Sent, Draft, Read, Replied
    incoming        INTEGER NOT NULL,   -- 1 = from customer, 0 = from agent
    has_attachment   INTEGER NOT NULL DEFAULT 0,
    headers         TEXT,
    sequence_num    INTEGER NOT NULL    -- Order within the case thread
);

-- Internal case comments
CREATE TABLE case_comments (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL REFERENCES cases(id),
    comment_body    TEXT NOT NULL,
    created_by_id   TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL,
    is_published    INTEGER NOT NULL DEFAULT 0
);

-- Activity feed items
CREATE TABLE feed_items (
    id              TEXT PRIMARY KEY,
    case_id         TEXT NOT NULL REFERENCES cases(id),
    body            TEXT NOT NULL,
    type            TEXT NOT NULL,      -- TrackedChange, TextPost, ContentPost
    created_by_id   TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL
);

-- JIRA issues (engineering escalations)
CREATE TABLE jira_issues (
    id              TEXT PRIMARY KEY,
    key             TEXT NOT NULL UNIQUE,
    project_key     TEXT NOT NULL,
    summary         TEXT NOT NULL,
    description_adf TEXT NOT NULL,      -- JSON ADF document
    issue_type      TEXT NOT NULL,      -- Bug, Task, Story
    status          TEXT NOT NULL,
    priority        TEXT NOT NULL,
    assignee_id     TEXT,
    reporter_id     TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    labels          TEXT,              -- JSON array
    -- Link back to Salesforce case
    sf_case_id      TEXT REFERENCES cases(id)
);

-- JIRA comments
CREATE TABLE jira_comments (
    id              TEXT PRIMARY KEY,
    issue_id        TEXT NOT NULL REFERENCES jira_issues(id),
    author_id       TEXT NOT NULL,
    body_adf        TEXT NOT NULL,      -- JSON ADF document
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

-- JIRA users (separate from SF users)
CREATE TABLE jira_users (
    account_id      TEXT PRIMARY KEY,   -- 24-char hex
    display_name    TEXT NOT NULL,
    email           TEXT,
    account_type    TEXT NOT NULL DEFAULT 'atlassian',
    active          INTEGER NOT NULL DEFAULT 1,
    -- Optional mapping to SF user
    sf_user_id      TEXT REFERENCES users(id)
);
```

## Generation Pipeline

### Phase 1: Foundation
1. Generate accounts (25 diverse companies)
2. Generate contacts (75, ~3 per account)
3. Generate users (15 support agents with hierarchy)
4. Generate JIRA users (mapped from SF users + external)

### Phase 2: Cases
1. Generate 200 cases distributed by scenario template:
   - 30% quick resolution (1-3 emails)
   - 40% standard issues (5-8 emails)
   - 20% complex escalations (10-20 emails, JIRA link)
   - 10% long-running (15+ emails)
2. Distribute across accounts, products, priorities, statuses

### Phase 3: Communications
1. For each case, generate email thread (chronological)
2. Generate internal comments at appropriate points
3. Generate feed items for status changes and actions

### Phase 4: JIRA Escalations
1. For ~40 escalated cases, generate linked JIRA issues
2. Generate JIRA comments with technical discussion
3. Ensure bidirectional references (case ↔ issue)

### Phase 5: Validation & Export
1. Validate referential integrity across all tables
2. Check temporal consistency (dates make sense)
3. Export to Salesforce JSON format
4. Export to JIRA JSON format

## CLI Usage

```bash
# Generate all data (phases 1-4)
go run ./cmd/generate --config config.yaml

# Generate specific phase
go run ./cmd/generate --phase 1 --config config.yaml

# Export to Salesforce format
go run ./cmd/export --format salesforce --out ../salesforce/mock_api/testdata/seed/

# Export to JIRA format
go run ./cmd/export --format jira --out ../jira/mock_api/testdata/seed/

# Export all formats
go run ./cmd/export --all
```

## Configuration (config.yaml)

```yaml
database:
  path: data/mock.db

volumes:
  accounts: 25
  contacts_per_account: 3
  users: 15
  cases: 200
  emails_per_case_avg: 5
  comments_per_case_avg: 3
  feed_items_per_case_avg: 2
  jira_escalation_pct: 20
  jira_comments_per_issue_avg: 3

date_range:
  start: "2024-01-01T00:00:00Z"
  end: "2025-01-01T00:00:00Z"

scenarios:
  quick_resolution_pct: 30
  standard_issue_pct: 40
  complex_escalation_pct: 20
  long_running_pct: 10

case_status_distribution:
  new: 10
  in_progress: 30
  escalated: 10
  pending_customer: 15
  closed: 35

export:
  salesforce: ../salesforce/mock_api/testdata/seed/
  jira: ../jira/mock_api/testdata/seed/
```

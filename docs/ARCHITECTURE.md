# Mock Data Generation Architecture

System design and data flow documentation.

## Overview

The mock data generation system creates realistic customer support interaction data
for testing and development. It uses LLM-generated content stored in SQLite, with
exporters that convert to format-specific JSON for each mock API.

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     mock_data_generation/                                    │
│                                                                             │
│  ┌──────────────┐     ┌──────────────────┐     ┌──────────────┐            │
│  │   Profiles   │     │    Scenarios     │     │  Generators  │            │
│  │              │────▶│                  │────▶│              │            │
│  │ YAML configs │     │ acme, industry   │     │ accounts     │            │
│  │ per industry │     │                  │     │ contacts     │            │
│  └──────────────┘     └──────────────────┘     │ users        │            │
│                              │                  │ cases        │            │
│                              │                  │ emails       │            │
│                              ▼                  │ comments     │            │
│  ┌──────────────┐     ┌──────────────────┐     │ jira_*       │            │
│  │     LLM      │────▶│    Pipeline      │────▶└──────┬───────┘            │
│  │   Adapter    │     │   (4 phases)     │            │                     │
│  │              │     │                  │            ▼                     │
│  │ z.ai/ollama  │     │ 1. Foundation    │     ┌──────────────┐            │
│  │ openai       │     │ 2. Cases         │     │    SQLite    │            │
│  └──────────────┘     │ 3. Communications│     │   Database   │            │
│                       │ 4. JIRA          │     │              │            │
│                       └──────────────────┘     │ data/mock.db │            │
│                                                └──────┬───────┘            │
│                                                       │                     │
│                       ┌───────────────────────────────┼──────────────┐     │
│                       │                               ▼              │     │
│                       │         ┌──────────────────────────────────┐ │     │
│                       │         │           Exporters              │ │     │
│                       │         │                                  │ │     │
│                       │         │  ┌────────────┐ ┌────────────┐  │ │     │
│                       │         │  │ Salesforce │ │    JIRA    │  │ │     │
│                       │         │  │  Exporter  │ │  Exporter  │  │ │     │
│                       │         │  └─────┬──────┘ └─────┬──────┘  │ │     │
│                       │         │        │              │         │ │     │
│                       │         │  ┌─────▼──────────────▼──────┐  │ │     │
│                       │         │  │      Falcon Exporter      │  │ │     │
│                       │         │  │    (4-tier LocalStore)    │  │ │     │
│                       │         │  └────────────┬──────────────┘  │ │     │
│                       │         └───────────────┼─────────────────┘ │     │
│                       │                         │                   │     │
│                       │                         ▼                   │     │
│                       │    ┌────────────────────────────────────┐   │     │
│                       │    │           JSON Seed Files          │   │     │
│                       │    │                                    │   │     │
│                       │    │  salesforce/  jira/  falcon/       │   │     │
│                       │    └────────────────────────────────────┘   │     │
│                       └─────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/integration/mock_data_generation/
├── cmd/
│   ├── acme/          # Acme Software scenario CLI
│   ├── industry/      # Industry-specific scenario CLI
│   └── export/        # Export CLI
├── internal/
│   ├── config/        # Environment loading
│   ├── db/            # SQLite schema, models, store
│   ├── exporter/      # Format-specific exporters
│   │   ├── salesforce.go
│   │   ├── jira.go
│   │   ├── falcon.go
│   │   └── scenarios.go
│   ├── generator/     # Per-entity generators
│   │   ├── accounts.go
│   │   ├── contacts.go
│   │   ├── users.go
│   │   ├── cases.go
│   │   ├── emails.go
│   │   ├── comments.go
│   │   ├── feed_items.go
│   │   ├── jira_issues.go
│   │   └── jira_comments.go
│   ├── llm/           # LLM adapter (z.ai, ollama, openai)
│   ├── pipeline/      # Phased generation orchestration
│   ├── profile/       # Vendor profile loader
│   └── scenario/      # Scenario implementations
├── profiles/          # YAML vendor profiles
│   ├── acme_software.yaml
│   ├── healthcare_medtech.yaml
│   ├── finserv_fincore.yaml
│   ├── saas_cloudops.yaml
│   ├── retail_retailedge.yaml
│   └── manufacturing_factoryos.yaml
├── data/              # Generated SQLite databases
└── docs/              # Documentation
```

## Data Flow

### 1. Profile Loading

Vendor profiles (YAML) define:
- Company information (name, industry, size)
- Products and features
- Customer segments (Enterprise, Mid-Market, SMB)
- Support team structure (L1, L2, L3, Manager)
- SLAs and case type distributions
- Communication patterns and JIRA escalation rules

### 2. Scenario Execution

Scenarios use profiles to drive realistic data generation:

1. **Parse profile** → Extract customer segments, products, team structure
2. **Generate entities** → Accounts, contacts, users based on profile
3. **Generate interactions** → Cases, emails, comments based on distributions
4. **Generate escalations** → JIRA issues for high-priority cases

### 3. LLM Content Generation

The LLM adapter provides content for:
- Company names and descriptions
- Contact names and titles
- Case subjects and descriptions


| Phase | Name | Tables Created | Dependencies |
|-------|------|----------------|--------------|
| 1 | Foundation | accounts, contacts, users | None |
| 2 | Cases | cases | accounts, contacts, users |
| 3 | Communications | email_messages, case_comments, feed_items | cases |
| 4 | JIRA Escalations | jira_users, jira_issues, jira_comments | cases (high priority) |

---

## Database Schema

### Entity Tables

```sql
-- Core entities
accounts (id, sf_id, name, industry, type, annual_revenue, employee_count, ...)
contacts (id, sf_id, account_id, first_name, last_name, email, title, ...)
users    (id, sf_id, username, email, role, tier, ...)

-- Salesforce interactions
cases          (id, sf_id, case_number, account_id, contact_id, owner_id, subject, description, status, priority, ...)
email_messages (id, sf_id, case_id, sequence_num, subject, text_body, from_name, from_address, ...)
case_comments  (id, sf_id, case_id, author_id, comment_body, is_public, ...)
feed_items     (id, sf_id, case_id, author_id, body, ...)

-- JIRA escalations
jira_users    (id, jira_key, email, display_name, ...)
jira_issues   (id, jira_key, case_id, project_key, issue_type, summary, description, status, priority, ...)
jira_comments (id, jira_id, issue_id, author_id, body, ...)
```

### Entity Relationships

```
┌──────────┐       ┌──────────┐       ┌──────────┐
│ accounts │◄──────│ contacts │       │  users   │
└────┬─────┘       └────┬─────┘       └────┬─────┘
     │                  │                  │
     │     ┌────────────┴──────────────────┤
     │     │                               │
     ▼     ▼                               ▼
┌────────────────────────────────────────────────┐
│                    cases                        │
│  account_id, contact_id, owner_id → FK refs    │
└─────────────────────┬──────────────────────────┘
                      │
    ┌─────────────────┼─────────────────┬─────────────────┐
    │                 │                 │                 │
    ▼                 ▼                 ▼                 ▼
┌────────────┐  ┌────────────┐  ┌────────────┐   ┌─────────────┐
│email_msgs  │  │case_comment│  │ feed_items │   │ jira_issues │
└────────────┘  └────────────┘  └────────────┘   └──────┬──────┘
                                                        │
                                                        ▼
                                                 ┌─────────────┐
                                                 │jira_comments│
                                                 └─────────────┘
```

### Foreign Key Constraints

| Table | Column | References |
|-------|--------|------------|
| contacts | account_id | accounts.id |
| cases | account_id | accounts.id |
| cases | contact_id | contacts.id |
| cases | owner_id | users.id |
| email_messages | case_id | cases.id |
| case_comments | case_id | cases.id |
| case_comments | author_id | users.id |
| feed_items | case_id | cases.id |
| feed_items | author_id | users.id |
| jira_issues | case_id | cases.id |
| jira_comments | issue_id | jira_issues.id |
| jira_comments | author_id | jira_users.id |

---

## Deterministic Generation (Faker Seeds)

To ensure reproducible data, generators use seeded random number generators:

```go
// Each generator seeds based on entity type and index
seed := hash(entityType + strconv.Itoa(index))
rng := rand.New(rand.NewSource(seed))
faker := gofakeit.NewWithSeed(seed)
```

This ensures:
- Same input profile → same output data
- Regenerating doesn't change existing records
- Tests can rely on consistent data

---

## Exporter Architecture

### Common Interface

All exporters implement:

```go
type Exporter interface {
    Export(db *Store, outputDir string) error
}
```

### Salesforce Exporter

Exports 7 entity types to individual JSON files matching SF API format:

- Transforms internal IDs to SF-format 15/18-char IDs
- Converts timestamps to SF datetime format
- Includes relationship fields (AccountId, ContactId, etc.)

### JIRA Exporter

Exports 3 entity types to JIRA REST API format:

- Converts to JIRA key format (PROJECT-123)
- Maps status/priority to JIRA values
- Includes Atlassian Document Format for descriptions

### Falcon Exporter

Exports to 4-tier LocalStore seed format:

| Seed File | Tier | Purpose |
|-----------|------|---------|
| `processing_seed.json` | Processing | integration_cache, llm_results |
| `orchestration_seed.json` | Orchestration | workflows, jobs, executions |
| `management_seed.json` | Management | users, roles, audit_logs |
| `data_seed.json` | Data | workflow_state |

**Scenario-based scaling**: Uses scenario presets to control data volume:

```go
scenarios := map[string]ScenarioConfig{
    "fresh_environment":    {Accounts: 5, Cases: 15},
    "busy_day":             {Accounts: 50, Cases: 500},
    "sync_stress":          {Accounts: 100, Cases: 1000},
    "historical_12_months": {Accounts: 500, Cases: 10000},
}
```

---

## LLM Adapter

The LLM adapter abstracts provider differences:

```go
type LLMAdapter interface {
    Generate(prompt string) (string, error)
}

// Provider implementations
type ZaiAdapter struct { ... }
type OllamaAdapter struct { ... }
type OpenAIAdapter struct { ... }
```

### Fallback Content

When LLM fails (rate limits, network issues, malformed JSON):

1. Log the error
2. Return deterministic fallback content
3. Continue generation without interruption

Fallback content uses templates + faker for realistic-looking data.

---

## Configuration

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `ZAI_API_KEY` | Z.ai API authentication | Required for zai provider |
| `OPENAI_API_KEY` | OpenAI authentication | Required for openai provider |
| `OLLAMA_HOST` | Ollama server address | http://localhost:11434 |

### Profile Configuration

Profiles control generation parameters:

```yaml
company:
  name: "Acme Software Inc."
  industry: "Enterprise SaaS"

customer_segments:
  - name: "Enterprise"
    accounts: 3
    contacts_per_account: 5
  - name: "SMB"
    accounts: 5
    contacts_per_account: 2

support_team:
  tiers:
    - name: "L1"
      count: 8
    - name: "L2"
      count: 4
    - name: "L3"
      count: 2
    - name: "Manager"
      count: 1

case_distribution:
  priority:
    high: 0.15
    medium: 0.50
    low: 0.35
```

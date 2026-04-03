# CLI Reference

Complete documentation for all mock data generation commands.

## cmd/acme

Generate data for the Acme Software demo scenario.

### Usage

```bash
go run ./cmd/acme [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--profile` | string | `profiles/acme_software.yaml` | Path to vendor profile YAML |
| `--db` | string | `data/mock.db` | Path to SQLite database |
| `--reset` | bool | `false` | Reset database before generating (drops all tables) |
| `--provider` | string | `zai` | LLM provider: `zai`, `ollama`, `openai` |
| `--model` | string | `GLM-4.7` | Model name (provider-specific) |
| `--interactions` | bool | `false` | Generate interactions only (cases, emails, comments, JIRA) |
| `--full` | bool | `false` | Generate entities AND interactions (complete dataset) |

### Examples

```bash
# Generate complete dataset from scratch
go run ./cmd/acme --reset --full

# Generate entities only (accounts, contacts, users)
go run ./cmd/acme --reset

# Generate interactions for existing entities
go run ./cmd/acme --interactions

# Use Ollama instead of Z.ai
go run ./cmd/acme --reset --provider ollama --model llama3.1

# Use OpenAI
go run ./cmd/acme --reset --provider openai --model gpt-4o

# Use a different profile
go run ./cmd/acme --profile profiles/healthcare_medtech.yaml --reset
```

### Generation Phases

When generating a full dataset, the command runs these phases:

1. **Entities**: Accounts, Contacts, Users (based on profile's customer segments)
2. **Cases**: Support cases distributed by priority and status
3. **Communications**: Email threads, case comments, feed items
4. **JIRA**: Escalated issues linked to Salesforce cases

---

## cmd/industry

Generate data for any industry-specific scenario.

### Usage

```bash
go run ./cmd/industry --profile <path> [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--profile` | string | **required** | Path to vendor profile YAML |
| `--db` | string | `data/{industry}.db` | Path to SQLite database |
| `--reset` | bool | `false` | Reset database before generating |
| `--provider` | string | `zai` | LLM provider: `zai`, `ollama`, `openai` |
| `--model` | string | `GLM-4.7` | Model name |
| `--accounts` | int | `0` | Limit accounts per segment (0 = use profile) |
| `--cases` | int | `0` | Limit total cases (0 = default 200) |
| `--export` | bool | `false` | Export to JSON after generation |
| `--out` | string | `output/{industry}/` | Output directory for export |

### Available Profiles

| Profile File | Industry | Company Name |
|--------------|----------|--------------|
| `acme_software.yaml` | Enterprise SaaS | Acme Software Inc. |
| `healthcare_medtech.yaml` | Healthcare IT | MedTech Solutions |
| `finserv_fincore.yaml` | Financial Services | FinCore Technologies |
| `saas_cloudops.yaml` | Cloud/SaaS | CloudOps Platform |
| `retail_retailedge.yaml` | Retail/E-commerce | RetailEdge Systems |
| `manufacturing_factoryos.yaml` | Manufacturing | FactoryOS Inc. |

### Examples

```bash
# Generate healthcare dataset
go run ./cmd/industry --profile profiles/healthcare_medtech.yaml --reset

# Generate with custom limits (faster)
go run ./cmd/industry --profile profiles/saas_cloudops.yaml --reset --accounts 2 --cases 20

# Generate and export
go run ./cmd/industry --profile profiles/finserv_fincore.yaml --reset --export

# Custom output directory
go run ./cmd/industry --profile profiles/retail_retailedge.yaml --reset --export --out ./my-export/

# Specify database location
go run ./cmd/industry --profile profiles/manufacturing_factoryos.yaml --db ./custom/factory.db --reset
```

---

## cmd/export

Export SQLite data to JSON formats for mock APIs.

### Usage

```bash
go run ./cmd/export [flags]
```



#### Salesforce (`--format salesforce`)

Exports data in Salesforce mock API seed format.

**Output Files:**

| File | Description |
|------|-------------|
| `accounts.json` | Customer account records |
| `contacts.json` | Contact records with account links |
| `users.json` | Support agent user records |
| `cases.json` | Support case records |
| `email_messages.json` | Email thread messages |
| `case_comments.json` | Internal case comments |
| `feed_items.json` | Activity feed items |

#### JIRA (`--format jira`)

Exports data in JIRA mock API seed format.

**Output Files:**

| File | Description |
|------|-------------|
| `issues.json` | JIRA issue records |
| `comments.json` | Issue comments |
| `users.json` | JIRA user records |

#### Falcon (`--format falcon`)

Exports data in Falcon 4-tier LocalStore seed format for the Data Tier team.

**Output Files:**

| File | Tier | Tables |
|------|------|--------|
| `processing_seed.json` | Processing | integration_cache, llm_results, processing_results |
| `orchestration_seed.json` | Orchestration | workflow_definitions, jobs, executions |
| `management_seed.json` | Management | users, roles, user_roles, audit_logs |
| `data_seed.json` | Data | workflow_state |

### Falcon Scenarios

Use `--list-scenarios` to see available presets:

```bash
go run ./cmd/export --list-scenarios
```

| Scenario | Description | Scale |
|----------|-------------|-------|
| `fresh_environment` | New tenant with initial data | 5 accounts, 15 cases |
| `busy_day` | Typical production day | 50 accounts, 500 cases |
| `sync_stress` | Stress testing sync engine | 100 accounts, 1000 cases |
| `historical_12_months` | 12 months of historical data | 500 accounts, 10000 cases |

**Scenario Details:**

```
fresh_environment
    Entities: 5 accounts, 10 contacts, 3 users, 15 cases

busy_day
    Entities: 50 accounts, 200 contacts, 25 users, 500 cases

sync_stress
    Entities: 100 accounts, 400 contacts, 50 users, 1000 cases

historical_12_months
    Entities: 500 accounts, 2000 contacts, 100 users, 10000 cases
```

### Examples

```bash
# Export to Salesforce format
go run ./cmd/export --format salesforce --out ./export/sf/

# Export to JIRA format
go run ./cmd/export --format jira --out ./export/jira/

# Export Falcon with scenario
go run ./cmd/export --format falcon --scenario busy_day --out ./export/falcon/

# Export all formats to default paths
go run ./cmd/export --all

# Export from a different database
go run ./cmd/export --format salesforce --db data/healthcare_medtech.db --out ./export/healthcare/

# List available scenarios
go run ./cmd/export --list-scenarios
```

### Default Paths (with `--all`)

When using `--all`, exports go to:

- Salesforce: `../salesforce/mock_api/testdata/seed/`
- JIRA: `../jira/mock_api/testdata/seed/`
- Falcon: `./seeds/falcon/`

---

## Environment Variables

### LLM API Keys

| Variable | Provider | Required |
|----------|----------|----------|
| `ZAI_API_KEY` | Z.ai (default) | Yes, when using `--provider zai` |
| `OPENAI_API_KEY` | OpenAI | Yes, when using `--provider openai` |
| `ANTHROPIC_API_KEY` | Anthropic | Yes, when using `--provider anthropic` |

Ollama does not require an API key (runs locally).

### Configuration Location

API keys should be set in `internal/integration/.env`:

```bash
ZAI_API_KEY=your-zai-key
OPENAI_API_KEY=your-openai-key
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (database, LLM, or configuration issue) |

---

## Common Patterns

### Full Regeneration

```bash
rm -f data/mock.db
go run ./cmd/acme --reset --full
go run ./cmd/export --all
```

### Quick Test Dataset

```bash
go run ./cmd/industry --profile profiles/acme_software.yaml --reset --accounts 1 --cases 5
```

### Multiple Industries

```bash
for p in profiles/*.yaml; do
  go run ./cmd/industry --profile "$p" --reset --export
done
```


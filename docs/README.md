# Mock Data Generation System

Generate realistic customer support interaction data for testing and development.

## What It Does

This system generates mock data for customer support scenarios including:

- **Customer accounts** with industry-specific company profiles
- **Contact records** linked to accounts
- **Support team users** with role hierarchy (L1, L2, L3, Manager)
- **Support cases** with realistic subjects and descriptions
- **Email threads** between customers and support agents
- **Case comments** and activity feed items
- **JIRA issues** for escalated engineering cases

Data is stored in SQLite and can be exported to multiple formats:

- **Salesforce JSON** - For SF mock API seed files
- **JIRA JSON** - For JIRA mock API seed files
- **Falcon** - 4-tier LocalStore format for Data Tier

## Quick Start

```bash
# Navigate to the module
cd internal/integration/mock_data_generation

# Generate the complete Acme Software dataset
go run ./cmd/acme --reset --full

# Verify the data
sqlite3 data/mock.db "SELECT COUNT(*) FROM accounts; SELECT COUNT(*) FROM cases;"

# Export to all formats
go run ./cmd/export --all
```

## Prerequisites

- Go 1.21+
- Z.ai API key (set `ZAI_API_KEY` in `internal/integration/.env`)
- Or Ollama for local LLM inference

## Available Commands

| Command | Purpose |
|---------|---------|
| `cmd/acme` | Generate Acme Software demo dataset |
| `cmd/industry` | Generate industry-specific datasets |
| `cmd/export` | Export SQLite data to JSON formats |

## Available Profiles

| Profile | Industry | Description |
|---------|----------|-------------|
| `acme_software.yaml` | Enterprise SaaS | B2B project management company |
| `healthcare_medtech.yaml` | Healthcare IT | Medical technology vendor |
| `finserv_fincore.yaml` | Financial Services | Banking software provider |
| `saas_cloudops.yaml` | Cloud/SaaS | DevOps and cloud platform |
| `retail_retailedge.yaml` | Retail/E-commerce | Retail technology solutions |
| `manufacturing_factoryos.yaml` | Manufacturing | Industrial software |

## Documentation

- [**TUTORIAL.md**](TUTORIAL.md) - Step-by-step walkthrough with copy-paste commands
- [**CLI_REFERENCE.md**](CLI_REFERENCE.md) - Complete CLI documentation with all flags
- [**ARCHITECTURE.md**](ARCHITECTURE.md) - System design and data flow

## Database Location

Generated data is stored in SQLite databases:

- `data/mock.db` - Default Acme Software dataset
- `data/{industry}.db` - Industry-specific datasets (e.g., `healthcare_medtech.db`)

## Canonical Data Location

After generation, copy the database to the canonical location for use by other services:

```bash
cp data/mock.db ../../../mock_data/mock.db
```


# Mock Data Generation Tutorial

This tutorial walks you through generating a complete mock dataset from scratch.

## Prerequisites

### 1. Go Installation

Ensure you have Go 1.21 or later:

```bash
go version
# Should show: go version go1.21.x or higher
```

### 2. API Key Configuration

The generator uses Z.ai GLM-4.7 by default. Verify your API key is configured:

```bash
# Check for ZAI_API_KEY in the repo's .env file
cat internal/integration/.env | grep ZAI_API_KEY

# Expected output:
# ZAI_API_KEY=your-api-key-here
```

If not set, add it to `internal/integration/.env`:

```bash
echo 'ZAI_API_KEY=your-api-key-here' >> internal/integration/.env
```

**Alternative: Use Ollama for Local LLM**

If you prefer local inference:

```bash
# Start Ollama with a compatible model
ollama run llama3.1

# Use --provider flag when running commands
go run ./cmd/acme --reset --provider ollama --model llama3.1
```

---

## Part 1: Generate Acme Software Dataset

Navigate to the mock data generation module:

```bash
cd internal/integration/mock_data_generation
```

### Option A: Full Dataset (Entities + Interactions)

Generate everything in one command (~5-10 minutes with LLM calls):

```bash
go run ./cmd/acme --reset --full
```

### Option B: Incremental Generation

Generate in stages for faster iteration:

```bash
# Step 1: Generate entities only (accounts, contacts, users) - ~2 min
go run ./cmd/acme --reset

# Step 2: Generate interactions (cases, emails, comments, JIRA) - ~5 min
go run ./cmd/acme --interactions
```

### Expected Output

```
📊 Accounts by Segment:
  Enterprise | Finance: 1
  Enterprise | Healthcare: 1
  Mid-Market | Technology: 2
  Mid-Market | Manufacturing: 2
  SMB | Startup: 2
  SMB | Consulting: 2

👥 Contacts per Account:
  [Enterprise] GlobalHealth Systems: 5 contacts
  [Mid-Market] TechFlow Solutions: 3 contacts
  ...

🎧 Support Team:
  L1: 8
  L2: 4
  L3: 2
  Manager: 1

📝 Interactions:
  Cases: 200
  Email Messages: 850
  Case Comments: 350
  Feed Items: 400
  JIRA Issues: 40
  JIRA Comments: 120
```

---

## Part 2: Verify Generated Data

### Quick Counts

```bash
sqlite3 data/mock.db ".tables"
# Output: accounts  case_comments  cases  contacts  email_messages
#         feed_items  jira_comments  jira_issues  jira_users  users
```

```bash
sqlite3 data/mock.db "
SELECT 'accounts' as table_name, COUNT(*) as count FROM accounts
UNION SELECT 'contacts', COUNT(*) FROM contacts
UNION SELECT 'users', COUNT(*) FROM users
UNION SELECT 'cases', COUNT(*) FROM cases
UNION SELECT 'email_messages', COUNT(*) FROM email_messages
UNION SELECT 'case_comments', COUNT(*) FROM case_comments
UNION SELECT 'jira_issues', COUNT(*) FROM jira_issues;
"
```

### Sample Data

```bash
# View sample accounts
sqlite3 -header -column data/mock.db "SELECT name, industry, type FROM accounts LIMIT 5;"

# View sample cases
sqlite3 -header -column data/mock.db "SELECT case_number, subject, status, priority FROM cases LIMIT 5;"

# View email thread for a case
sqlite3 -header -column data/mock.db "
SELECT e.sequence_num, e.from_name, substr(e.subject, 1, 50) as subject
FROM email_messages e
JOIN cases c ON e.case_id = c.id
WHERE c.case_number = '00001001'
ORDER BY e.sequence_num;
"
```

---

## Part 3: Generate Industry-Specific Data

### Generate Healthcare Dataset

```bash
go run ./cmd/industry --profile profiles/healthcare_medtech.yaml --reset
```

### Generate Financial Services Dataset

```bash
go run ./cmd/industry --profile profiles/finserv_fincore.yaml --reset
```

### Generate with Custom Limits

```bash
# Smaller dataset for quick testing
go run ./cmd/industry --profile profiles/saas_cloudops.yaml --reset --accounts 2 --cases 20
```

### Generate with Auto-Export

```bash
# Generate and immediately export to JSON
go run ./cmd/industry --profile profiles/retail_retailedge.yaml --reset --export
# Output goes to: output/retail_retailedge/salesforce/ and output/retail_retailedge/jira/
```

---

## Part 4: Export Data

### Export to Salesforce Format

```bash
go run ./cmd/export --format salesforce --out ./export/salesforce/
```

Creates these files:

- `accounts.json`
- `contacts.json`
- `users.json`
- `cases.json`
- `email_messages.json`
- `case_comments.json`
- `feed_items.json`

### Export to JIRA Format

```bash
go run ./cmd/export --format jira --out ./export/jira/
```

Creates these files:

- `issues.json`
- `comments.json`
- `users.json`

### Export to Falcon (Data Tier) Format

```bash
# List available scenarios
go run ./cmd/export --list-scenarios

# Export with a scenario
go run ./cmd/export --format falcon --scenario busy_day --out ./export/falcon/
```

Creates tier-specific seed files:

- `processing_seed.json` - integration_cache, llm_results, processing_results
- `orchestration_seed.json` - workflow_definitions, jobs, executions
- `management_seed.json` - users, roles, user_roles, audit_logs
- `data_seed.json` - workflow_state

### Export All Formats

```bash
go run ./cmd/export --all
# Exports to default paths:
#   ../salesforce/mock_api/testdata/seed/
#   ../jira/mock_api/testdata/seed/
#   ./seeds/falcon/
```

---

## Part 5: Promote to Canonical Location

Copy the generated database to the shared mock_data location:

```bash
cp data/mock.db ../../../mock_data/mock.db
```

---

## Common Workflows

### Fresh Start

```bash
# Delete existing database and regenerate
rm -f data/mock.db
go run ./cmd/acme --reset --full
```

### Add More Interactions to Existing Entities

```bash
# Keep existing accounts/contacts/users, add new cases/emails
go run ./cmd/acme --interactions
```

### Switch LLM Provider

```bash
# Use OpenAI instead of Z.ai
export OPENAI_API_KEY=your-key
go run ./cmd/acme --reset --provider openai --model gpt-4o

# Use local Ollama
go run ./cmd/acme --reset --provider ollama --model llama3.1
```

### Generate Multiple Industries

```bash
# Generate all industry profiles
for profile in profiles/*.yaml; do
  go run ./cmd/industry --profile "$profile" --reset --accounts 2 --cases 20
done
```

---

## Troubleshooting

### "ZAI_API_KEY not set"

Ensure the key is in `internal/integration/.env`:

```bash
echo 'ZAI_API_KEY=your-key' >> ../../.env
```

### "Failed to parse LLM response"

The LLM sometimes returns malformed JSON. The generator will retry or use fallback content. Check logs for details.

### Database Locked

Ensure no other process has the database open:

```bash
lsof data/mock.db
```

### Export Directory Not Found

Create the output directory before exporting:

```bash
mkdir -p ./export/salesforce
go run ./cmd/export --format salesforce --out ./export/salesforce/
```

# Mock Data Generation Agent Prompt

## Context: What Has Been Built

We have built **mock APIs** for **Salesforce** and **JIRA** to support demo presentations of our customer support analysis platform. These mock APIs simulate real ticketing systems with realistic data.

### Architecture Overview

```
internal/integration/
├── salesforce/mock_api/     # Salesforce Service Cloud mock
│   ├── cmd/salesforce-mock/ # Standalone server
│   ├── internal/            # Handlers, SOQL parser, store
│   └── testdata/seed/       # JSON seed data files
│
└── jira/mock_api/           # JIRA Cloud v3 mock
    ├── cmd/jira-mock/       # Standalone server
    ├── internal/            # Handlers, JQL parser, store
    └── testdata/seed/       # JSON seed data files
```

### Current Implementation Status

| Component | Status |
|-----------|--------|
| Salesforce OAuth | ✅ Complete |
| Salesforce SOQL Query | ✅ Complete |
| Salesforce SObject CRUD | ✅ Complete |
| Salesforce Describe | ✅ Complete |
| JIRA Basic Auth | ✅ Complete |
| JIRA JQL Search | ✅ Complete |
| JIRA Issue CRUD | ✅ Complete |
| JIRA Transitions | ✅ Complete |

### Current Seed Data (Limited)

| Object | Salesforce | JIRA |
|--------|------------|------|
| Companies/Projects | 10 accounts | 3 projects |
| Users | 8 agents | 10 users |
| Contacts | 10 contacts | N/A |
| Cases/Issues | 8 cases | 10 issues |
| Emails/Comments | 12 emails | 10 comments |
| Comments | 14 comments | N/A |
| Feed Items | 15 items | N/A |

---

## Your Mission

Generate **comprehensive, realistic mock data** representing the full spectrum of customer-company interactions for a B2B SaaS company providing enterprise software (analytics, security, integrations).

### Target Data Volumes

| Object | Target Count | Notes |
|--------|--------------|-------|
| Accounts (Companies) | 25 | Mix of industries, sizes, tiers |
| Contacts | 75 | 3 per account average |
| Users (Support Agents) | 15 | Tiered: L1, L2, L3, Managers |
| Cases | 200 | Full lifecycle distribution |
| Email Messages | 1,000 | 5 per case average |
| Case Comments | 600 | 3 per case average (internal notes) |
| Feed Items | 400 | 2 per case average (activity log) |
| JIRA Issues | 150 | Engineering escalations |
| JIRA Comments | 450 | 3 per issue average |

---

## Data Schemas

### Salesforce Objects

#### Account (Company)
```json
{
  "Id": "0013t00002AbCdEAAV",      // 18-char, prefix 001
  "Name": "Acme Corporation",
  "Industry": "Technology",         // Technology, Healthcare, Finance, Retail, Education, Manufacturing
  "Type": "Enterprise",             // Enterprise, Mid-Market, SMB
  "Website": "https://acme.example.com",
  "Phone": "555-0100",
  "BillingCity": "San Francisco",
  "BillingState": "CA",
  "AnnualRevenue": 50000000,
  "NumberOfEmployees": 500,
  "CreatedDate": "2023-01-15T10:00:00Z"
}
```

#### Contact (Customer Person)
```json
{
  "Id": "0033t00002CdEfGAAV",       // prefix 003
  "AccountId": "0013t00002AbCdEAAV",
  "FirstName": "John",
  "LastName": "Smith",
  "Email": "john.smith@acme.example.com",
  "Phone": "555-0101",
  "Title": "IT Director",           // CTO, IT Director, System Admin, Project Manager, etc.
  "Department": "Information Technology",
  "CreatedDate": "2023-01-16T09:00:00Z"
}
```

#### User (Support Agent)
```json
{
  "Id": "0053t00000XyZAbAAV",       // prefix 005
  "Name": "Maria Garcia",
  "FirstName": "Maria",
  "LastName": "Garcia",
  "Email": "maria.garcia@falcon.local",
  "Username": "mgarcia@falcon.local",
  "Title": "Senior Support Engineer",
  "Department": "Customer Support",
  "IsActive": true,
  "ManagerId": "0053t00000MgrIdAAV",
  "UserRole": "L2 Support",         // L1 Support, L2 Support, L3 Support, Support Manager, TAM
  "CreatedDate": "2022-06-01T08:00:00Z"
}
```

#### Case (Support Ticket)
```json
{
  "Id": "5003t00002AbCdEAAV",       // prefix 500
  "CaseNumber": "00123456",
  "Subject": "Unable to access dashboard after update",
  "Description": "Detailed description with context, steps to reproduce, expected vs actual...",
  "Status": "In Progress",          // New, In Progress, Escalated, Pending Customer, Closed
  "Priority": "P1",                 // P0 (Critical), P1 (High), P2 (Medium), P3 (Low)
  "Product__c": "Workspace ONE",    // Product names
  "Type": "Problem",                // Problem, Question, Feature Request, Incident
  "Origin": "Email",                // Email, Phone, Web, Chat
  "Reason": "User did not attend training",
  "OwnerId": "0053t00000XyZAbAAV",
  "ContactId": "0033t00002CdEfGAAV",
  "AccountId": "0013t00002AbCdEAAV",
  "CreatedDate": "2024-01-20T08:30:00Z",
  "ClosedDate": null,
  "ContactEmail": "john.smith@acme.example.com",
  "ContactPhone": "555-0101",
  "IsClosed": false,
  "IsEscalated": true
}
```

#### EmailMessage (Email Thread)
```json
{
  "Id": "02s3t00000AbCdEAAV",       // prefix 02s
  "ParentId": "5003t00002AbCdEAAV", // Case ID
  "Subject": "Re: Unable to access dashboard after update",
  "TextBody": "Full email text with proper formatting, signature, etc.",
  "HtmlBody": "<html><body>...</body></html>",
  "FromAddress": "john.smith@acme.example.com",
  "FromName": "John Smith",
  "ToAddress": "support@falcon.example.com",
  "CcAddress": "manager@acme.example.com",
  "BccAddress": null,
  "MessageDate": "2024-01-20T08:30:00Z",
  "Status": "New",                  // New (incoming), Sent (outgoing), Draft, Read, Replied
  "Incoming": true,                 // true = from customer, false = from agent
  "HasAttachment": false,
  "Headers": "Message-ID: <...>\nIn-Reply-To: <...>"
}
```

#### CaseComment (Internal Note)
```json
{
  "Id": "00a3t00001AbCdEAAV",       // prefix 00a
  "ParentId": "5003t00002AbCdEAAV", // Case ID
  "CommentBody": "Internal note about case progress, findings, next steps...",
  "CreatedById": "0053t00000XyZAbAAV",
  "CreatedDate": "2024-01-20T09:30:00Z",
  "IsPublished": false              // false = internal, true = visible to customer
}
```

#### FeedItem (Activity Feed)
```json
{
  "Id": "0D53t00000AbCdEAAV",       // prefix 0D5
  "ParentId": "5003t00002AbCdEAAV", // Case ID
  "Body": "Case priority changed from P2 to P1",
  "Type": "TrackedChange",          // TrackedChange, TextPost, ContentPost
  "CreatedById": "0053t00000XyZAbAAV",
  "CreatedDate": "2024-01-20T09:00:00Z"
}
```

### JIRA Objects

#### Issue
```json
{
  "id": "10001",
  "key": "SUPPORT-1",
  "self": "https://mock.atlassian.net/rest/api/3/issue/10001",
  "fields": {
    "summary": "Short issue title",
    "description": {
      "version": 1,
      "type": "doc",
      "content": [
        {
          "type": "paragraph",
          "content": [{"type": "text", "text": "Description text..."}]
        }
      ]
    },
    "issuetype": {"id": "10001", "name": "Bug", "subtask": false},
    "project": {"id": "10000", "key": "SUPPORT", "name": "Customer Support"},
    "status": {"id": "10001", "name": "In Progress", "statusCategory": {"key": "indeterminate"}},
    "priority": {"id": "2", "name": "High"},
    "assignee": {"accountId": "5b10ac8d82e05b22cc7d4ef5", "displayName": "Sarah Chen"},
    "reporter": {"accountId": "customer001", "displayName": "John Customer"},
    "created": "2024-01-25T10:30:00.000+0000",
    "updated": "2024-01-27T14:15:00.000+0000",
    "labels": ["authentication", "critical-path"]
  }
}
```

#### Comment (ADF format)
```json
{
  "id": "10100",
  "self": "https://mock.atlassian.net/rest/api/3/issue/10001/comment/10100",
  "issueId": "10001",
  "author": {"accountId": "5b10ac8d82e05b22cc7d4ef5", "displayName": "Sarah Chen"},
  "body": {
    "version": 1,
    "type": "doc",
    "content": [
      {
        "type": "paragraph",
        "content": [{"type": "text", "text": "Comment content..."}]
      }
    ]
  },
  "created": "2024-01-25T11:00:00.000+0000",
  "updated": "2024-01-25T11:00:00.000+0000"
}
```

---

## Interaction Scenarios to Generate

Create diverse, realistic scenarios covering:

### 1. Support Case Lifecycles

#### Quick Resolutions (30% of cases)
- Simple how-to questions
- Password resets
- Configuration guidance
- 1-3 emails, resolved in hours

#### Standard Issues (40% of cases)
- Bug reports with reproduction steps
- Integration problems
- Performance issues
- 5-8 emails, resolved in 1-3 days

#### Complex Escalations (20% of cases)
- P0/P1 production outages
- Security incidents
- Multi-stakeholder coordination
- 10-20 emails, 5+ comments, escalated to engineering
- Creates linked JIRA issues

#### Long-Running Issues (10% of cases)
- Feature requests with back-and-forth
- Compliance/audit requests
- Data migration projects
- 15+ emails over weeks/months

### 2. Customer Emotional Journeys

Generate realistic emotional arcs:

- **Frustrated → Satisfied**: Customer starts angry, agent de-escalates, problem solved
- **Confused → Educated**: Customer doesn't understand, agent provides clear guidance
- **Urgent → Relieved**: Time-sensitive issue, agent prioritizes and delivers
- **Skeptical → Trusting**: Customer doubts solution, agent proves it works
- **Escalating → Executive Resolution**: Customer threatens to leave, manager intervenes

### 3. Industry-Specific Scenarios

**Healthcare (HIPAA)**
- PHI data handling questions
- Integration with Epic/Cerner
- Audit trail requirements

**Finance (SOC2/PCI)**
- Security certification questions
- Encryption requirements
- Access control issues

**Technology**
- API integration problems
- SSO/SAML configuration
- Performance optimization

**Retail**
- Seasonal scaling concerns
- Inventory sync issues
- Multi-location deployments

**Education**
- Academic calendar considerations
- Student data privacy
- Budget constraints

### 4. Product Areas

Generate issues across these product lines:
- **Analytics Platform** - Dashboards, reports, data visualization
- **Workspace ONE** - Enterprise mobility, device management
- **Identity Management** - SSO, MFA, user provisioning
- **Healthcare Integration** - HL7, FHIR, EHR connections
- **API Platform** - Rate limits, authentication, webhooks
- **Professional Services** - Migrations, implementations, training

### 5. Communication Patterns

**Email Styles:**
- Formal enterprise (C-level, legal)
- Technical detailed (IT admins, developers)
- Frustrated urgent (outage situations)
- Appreciative positive (resolved issues)
- Terse busy (executives)

**Agent Responses:**
- Professional acknowledgment
- Technical troubleshooting
- Escalation communication
- Resolution confirmation
- Follow-up satisfaction check

---

## Generation Guidelines

### ID Format Rules

```
Account:      001 + 15 alphanumeric + AAV
Contact:      003 + 15 alphanumeric + AAV
User:         005 + 15 alphanumeric + AAV
Case:         500 + 15 alphanumeric + AAV
EmailMessage: 02s + 15 alphanumeric + AAV
CaseComment:  00a + 15 alphanumeric + AAV
FeedItem:     0D5 + 15 alphanumeric + AAV
```

### Temporal Consistency

- Account created before its contacts
- Contacts created before their cases
- Cases have chronologically ordered emails
- Email threads have proper In-Reply-To headers
- Comments timestamped after case creation
- Feed items reflect status changes at correct times

### Relationship Integrity

- Every case links to valid Contact, Account, Owner (User)
- Every email's ParentId references valid Case
- Every comment's CreatedById references valid User
- JIRA issues link to Salesforce cases via custom field or labels

### Realistic Content

**Email Bodies Should Include:**
- Proper salutations and signatures
- Company email footers
- Technical details when relevant
- Emotional tone appropriate to situation
- Typos/informality in customer emails (realistic)

**Case Descriptions Should Include:**
- Problem statement
- Environment details
- Steps to reproduce (for bugs)
- Business impact statement
- Timeline/urgency context

---

## Output Format

Generate data as JSON files matching existing structure:

```
testdata/seed/
├── accounts.json      # 25 accounts
├── contacts.json      # 75 contacts
├── users.json         # 15 users
├── cases.json         # 200 cases
├── email_messages.json # 1000 emails
├── case_comments.json # 600 comments
├── feed_items.json    # 400 feed items
```

For JIRA:
```
testdata/seed/
├── projects.json      # 3 projects (keep existing)
├── users.json         # 10 users (extend)
├── issues.json        # 150 issues
├── comments.json      # 450 comments
├── statuses.json      # Keep existing
├── priorities.json    # Keep existing
├── transitions.json   # Keep existing
```

---

## Execution Plan

### Phase 1: Foundation Data
1. Generate 25 diverse accounts across industries
2. Generate 75 contacts (3 per account)
3. Generate 15 support users with hierarchy

### Phase 2: Case Scenarios
1. Define 10-15 scenario templates (quick fix, escalation, etc.)
2. Generate 200 cases distributed across scenarios
3. Ensure proper status/priority distribution

### Phase 3: Communication Content
1. Generate email threads for each case
2. Generate internal comments
3. Generate feed items for status changes

### Phase 4: JIRA Escalations
1. Identify 50 cases that escalate to engineering
2. Generate corresponding JIRA issues
3. Generate JIRA comments with technical discussion

### Phase 5: Validation
1. Verify referential integrity
2. Check temporal consistency
3. Validate JSON structure matches schemas

---

## Using z.ai GLM4.7 Effectively

Since you have unlimited tokens, use this approach:

1. **Batch Generation**: Generate objects in batches (e.g., 10 accounts at a time)
2. **Context Chaining**: Pass generated IDs to subsequent generations for relationships
3. **Template Variation**: Create base templates, then generate variations
4. **Review Cycles**: Generate, review sample, adjust prompts, regenerate

### Example Prompt for Account Generation:

```
Generate 5 B2B software company accounts as JSON array.
Industries: Technology, Healthcare, Finance
Include realistic company names, addresses, employee counts.
Revenue range: $10M - $500M
Format must match this schema exactly: [schema]
```

### Example Prompt for Case Thread:

```
Generate a complete support case scenario:
- Company: [account details]
- Contact: [contact details]
- Agent: [user details]
- Scenario: P1 production outage, customer frustrated, resolved in 6 hours
- Include: 8 emails (4 from customer, 4 from agent), 3 internal comments, 5 feed items

Emails should show:
1. Initial urgent report
2. Agent acknowledgment and questions
3. Customer provides logs
4. Agent identifies issue
5. Customer asks for ETA
6. Agent provides workaround
7. Customer confirms workaround helps
8. Agent confirms permanent fix deployed

Make emails realistic with signatures, emotional progression, technical details.
```

---

## Success Criteria

The generated data should:

1. **Look Real**: Someone unfamiliar should believe it's anonymized production data
2. **Tell Stories**: Each case should have a coherent narrative arc
3. **Show Diversity**: Represent multiple industries, urgencies, emotions, outcomes
4. **Be Consistent**: All IDs, dates, and relationships are valid
5. **Support Demos**: Enable showcasing the platform's analysis capabilities

---

## Questions to Consider

Before generating, decide:

1. What date range should cases span? (Suggest: Jan 2024 - Jan 2025)
2. What percentage of cases should be closed vs open?
3. Should we include any "bad" outcomes (unresolved, churned customer)?
4. Do you want real-ish company names or clearly fictional ones?
5. Should agents have specializations (by product, by tier)?

---

Ready to begin? Start with Phase 1: Foundation Data (Accounts, Contacts, Users).

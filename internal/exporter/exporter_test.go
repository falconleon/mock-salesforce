package exporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/db"
)

// testSetup creates a test store with sample data and returns cleanup func.
func testSetup(t *testing.T) (*db.Store, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "exporter-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	store, err := db.Open(tmpFile.Name(), logger)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("open store: %v", err)
	}

	if err := store.Init(); err != nil {
		store.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("init store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}

	return store, cleanup
}

// insertTestData populates the store with sample data for export tests.
func insertTestData(t *testing.T, store *db.Store) {
	t.Helper()

	// Account
	if err := store.InsertAccount(&db.Account{
		ID: "acct-001", Name: "Acme Corp", Industry: "Technology", Type: "Customer",
		Website: "https://acme.example.com", Phone: "555-1234",
		BillingCity: "San Francisco", BillingState: "CA",
		AnnualRevenue: 1000000.00, NumEmployees: 50,
		CreatedAt: "2024-01-15T10:00:00Z",
	}); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	// Contact
	if err := store.InsertContact(&db.Contact{
		ID: "cont-001", AccountID: "acct-001", FirstName: "Jane", LastName: "Doe",
		Email: "jane.doe@acme.example.com", Phone: "555-5678",
		Title: "VP Engineering", Department: "Engineering",
		CreatedAt: "2024-01-15T10:00:00Z",
	}); err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	// User
	if err := store.InsertUser(&db.User{
		ID: "user-001", FirstName: "John", LastName: "Agent",
		Email: "john.agent@support.example.com", Username: "jagent",
		Title: "Support Engineer", Department: "Support",
		IsActive: true, UserRole: "Standard",
		CreatedAt: "2024-01-15T10:00:00Z",
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Case
	if err := store.InsertCase(&db.Case{
		ID: "case-001", CaseNumber: "00001234", Subject: "Login Issue",
		Description: "Customer cannot login to the portal.",
		Status: "Open", Priority: "High", Product: "Widget Pro",
		CaseType: "Problem", Origin: "Email", Reason: "User Error",
		OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001",
		CreatedAt: "2024-01-15T10:00:00Z", IsEscalated: true,
		JiraIssueKey: "SUPPORT-123",
	}); err != nil {
		t.Fatalf("insert case: %v", err)
	}

	// Email
	if err := store.InsertEmail(&db.Email{
		ID: "email-001", CaseID: "case-001", Subject: "RE: Login Issue",
		TextBody: "Thank you for contacting us.", HTMLBody: "<p>Thank you.</p>",
		FromAddress: "support@example.com", FromName: "Support Team",
		ToAddress: "jane.doe@acme.example.com",
		MessageDate: "2024-01-15T11:00:00Z", Status: "Sent",
		Incoming: false, HasAttachment: false, SequenceNum: 1,
	}); err != nil {
		t.Fatalf("insert email: %v", err)
	}

	// Comment
	if err := store.InsertComment(&db.Comment{
		ID: "comment-001", CaseID: "case-001",
		CommentBody: "Customer confirmed issue is resolved.",
		CreatedByID: "user-001", CreatedAt: "2024-01-16T10:00:00Z",
		IsPublished: true,
	}); err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	// FeedItem
	if err := store.InsertFeedItem(&db.FeedItem{
		ID: "feed-001", CaseID: "case-001",
		Body: "Case escalated to tier 2.", Type: "TextPost",
		CreatedByID: "user-001", CreatedAt: "2024-01-15T12:00:00Z",
	}); err != nil {
		t.Fatalf("insert feed item: %v", err)
	}

	// Jira User
	if err := store.InsertJiraUser(&db.JiraUser{
		AccountID: "jira-001", DisplayName: "John Developer",
		Email: "john.dev@example.com", AccountType: "atlassian",
		Active: true, SFUserID: "user-001",
	}); err != nil {
		t.Fatalf("insert jira user: %v", err)
	}

	// Jira Issue
	if err := store.InsertJiraIssue(&db.JiraIssue{
		ID: "issue-001", Key: "SUPPORT-123", ProjectKey: "SUPPORT",
		Summary: "Customer login issue",
		DescriptionADF: `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Login issue reported"}]}]}`,
		IssueType: "Bug", Status: "In Progress", Priority: "High",
		AssigneeID: "jira-001", ReporterID: "jira-001",
		CreatedAt: "2024-01-15T10:00:00Z", UpdatedAt: "2024-01-16T10:00:00Z",
		Labels: `["customer-facing"]`, SFCaseID: "case-001",
	}); err != nil {
		t.Fatalf("insert jira issue: %v", err)
	}

	// Jira Comment
	if err := store.InsertJiraComment(&db.JiraComment{
		ID: "jc-001", IssueID: "issue-001", AuthorID: "jira-001",
		BodyADF:   `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Working on fix"}]}]}`,
		CreatedAt: "2024-01-15T14:00:00Z", UpdatedAt: "2024-01-15T14:00:00Z",
	}); err != nil {
		t.Fatalf("insert jira comment: %v", err)
	}
}

// --- Salesforce Exporter Tests ---

func TestSalesforceExport(t *testing.T) {
	store, cleanup := testSetup(t)
	defer cleanup()

	insertTestData(t, store)

	// Create temp output directory
	outDir, err := os.MkdirTemp("", "sf-export-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	exporter := NewSalesforceExporter(store.DB(), logger)

	if err := exporter.Export(outDir); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Verify expected files exist
	expectedFiles := []string{
		"accounts.json",
		"contacts.json",
		"users.json",
		"cases.json",
		"email_messages.json",
		"case_comments.json",
		"feed_items.json",
	}

	for _, fname := range expectedFiles {
		path := filepath.Join(outDir, fname)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s not found", fname)
		}
	}
}

func TestSalesforceExportAccountsFormat(t *testing.T) {
	store, cleanup := testSetup(t)
	defer cleanup()

	insertTestData(t, store)

	outDir, err := os.MkdirTemp("", "sf-export-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	exporter := NewSalesforceExporter(store.DB(), logger)

	if err := exporter.Export(outDir); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Read and validate accounts.json
	data, err := os.ReadFile(filepath.Join(outDir, "accounts.json"))
	if err != nil {
		t.Fatalf("read accounts.json: %v", err)
	}

	var accounts []map[string]interface{}
	if err := json.Unmarshal(data, &accounts); err != nil {
		t.Fatalf("parse accounts.json: %v", err)
	}

	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}

	acct := accounts[0]
	// Verify Salesforce field names
	requiredFields := []string{"Id", "Name", "Industry", "Type", "CreatedDate"}
	for _, field := range requiredFields {
		if _, ok := acct[field]; !ok {
			t.Errorf("account missing required field: %s", field)
		}
	}

	if acct["Id"] != "acct-001" {
		t.Errorf("account Id mismatch: got %v", acct["Id"])
	}
	if acct["Name"] != "Acme Corp" {
		t.Errorf("account Name mismatch: got %v", acct["Name"])
	}
}

func TestSalesforceExportCasesFormat(t *testing.T) {
	store, cleanup := testSetup(t)
	defer cleanup()

	insertTestData(t, store)

	outDir, err := os.MkdirTemp("", "sf-export-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	exporter := NewSalesforceExporter(store.DB(), logger)

	if err := exporter.Export(outDir); err != nil {
		t.Fatalf("export: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "cases.json"))
	if err != nil {
		t.Fatalf("read cases.json: %v", err)
	}

	var cases []map[string]interface{}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse cases.json: %v", err)
	}

	if len(cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(cases))
	}

	c := cases[0]
	// Verify Salesforce field names
	requiredFields := []string{"Id", "CaseNumber", "Subject", "Status", "Priority", "OwnerId", "AccountId", "ContactId"}
	for _, field := range requiredFields {
		if _, ok := c[field]; !ok {
			t.Errorf("case missing required field: %s", field)
		}
	}

	if c["CaseNumber"] != "00001234" {
		t.Errorf("case CaseNumber mismatch: got %v", c["CaseNumber"])
	}
	if c["IsEscalated"] != true {
		t.Errorf("case IsEscalated should be true")
	}
}

func TestSalesforceExportEmptyDatabase(t *testing.T) {
	store, cleanup := testSetup(t)
	defer cleanup()

	// No data inserted

	outDir, err := os.MkdirTemp("", "sf-export-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	exporter := NewSalesforceExporter(store.DB(), logger)

	if err := exporter.Export(outDir); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Verify accounts.json exists with empty array
	data, err := os.ReadFile(filepath.Join(outDir, "accounts.json"))
	if err != nil {
		t.Fatalf("read accounts.json: %v", err)
	}

	var accounts []map[string]interface{}
	if err := json.Unmarshal(data, &accounts); err != nil {
		t.Fatalf("parse accounts.json: %v", err)
	}

	if len(accounts) != 0 {
		t.Errorf("expected empty accounts array, got %d", len(accounts))
	}
}


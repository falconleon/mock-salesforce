package db

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

// testStore creates an in-memory test database and returns it with cleanup func.
func testStore(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	store, err := Open(tmpFile.Name(), logger)
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

// --- Account Tests ---

func TestAccountCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	acct := &Account{
		ID:            "acct-001",
		Name:          "Acme Corp",
		Industry:      "Technology",
		Type:          "Customer",
		Website:       "https://acme.example.com",
		Phone:         "555-1234",
		BillingCity:   "San Francisco",
		BillingState:  "CA",
		AnnualRevenue: 1000000.00,
		NumEmployees:  50,
		CreatedAt:     "2024-01-15T10:00:00Z",
	}

	if err := store.InsertAccount(acct); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	retrieved, err := store.GetAccountByID("acct-001")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}

	// Verify all fields round-trip correctly
	if retrieved.ID != acct.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, acct.ID)
	}
	if retrieved.Name != acct.Name {
		t.Errorf("Name mismatch: got %q, want %q", retrieved.Name, acct.Name)
	}
	if retrieved.Industry != acct.Industry {
		t.Errorf("Industry mismatch: got %q, want %q", retrieved.Industry, acct.Industry)
	}
	if retrieved.Type != acct.Type {
		t.Errorf("Type mismatch: got %q, want %q", retrieved.Type, acct.Type)
	}
	if retrieved.Website != acct.Website {
		t.Errorf("Website mismatch: got %q, want %q", retrieved.Website, acct.Website)
	}
	if retrieved.Phone != acct.Phone {
		t.Errorf("Phone mismatch: got %q, want %q", retrieved.Phone, acct.Phone)
	}
	if retrieved.BillingCity != acct.BillingCity {
		t.Errorf("BillingCity mismatch: got %q, want %q", retrieved.BillingCity, acct.BillingCity)
	}
	if retrieved.BillingState != acct.BillingState {
		t.Errorf("BillingState mismatch: got %q, want %q", retrieved.BillingState, acct.BillingState)
	}
	if retrieved.AnnualRevenue != acct.AnnualRevenue {
		t.Errorf("AnnualRevenue mismatch: got %v, want %v", retrieved.AnnualRevenue, acct.AnnualRevenue)
	}
	if retrieved.NumEmployees != acct.NumEmployees {
		t.Errorf("NumEmployees mismatch: got %d, want %d", retrieved.NumEmployees, acct.NumEmployees)
	}
	if retrieved.CreatedAt != acct.CreatedAt {
		t.Errorf("CreatedAt mismatch: got %q, want %q", retrieved.CreatedAt, acct.CreatedAt)
	}

	// Test QueryAccounts
	accounts, err := store.QueryAccounts()
	if err != nil {
		t.Fatalf("query accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(accounts))
	}
}

func TestAccountWithNullableFields(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test account with optional fields left empty
	acct := &Account{
		ID:        "acct-minimal",
		Name:      "Minimal Corp",
		Industry:  "Finance",
		Type:      "Prospect",
		CreatedAt: "2024-01-01T00:00:00Z",
		// Website, Phone, BillingCity, BillingState, AnnualRevenue, NumEmployees all empty/zero
	}

	if err := store.InsertAccount(acct); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	retrieved, err := store.GetAccountByID("acct-minimal")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}

	if retrieved.Website != "" {
		t.Errorf("expected empty Website, got %q", retrieved.Website)
	}
	if retrieved.AnnualRevenue != 0 {
		t.Errorf("expected 0 AnnualRevenue, got %v", retrieved.AnnualRevenue)
	}
}

func TestBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	accounts := []Account{
		{ID: "acct-001", Name: "Company A", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "acct-002", Name: "Company B", Industry: "Finance", Type: "Partner", CreatedAt: "2024-01-02T00:00:00Z"},
		{ID: "acct-003", Name: "Company C", Industry: "Healthcare", Type: "Prospect", CreatedAt: "2024-01-03T00:00:00Z"},
	}

	if err := store.InsertAccountsBatch(tx, accounts); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert accounts: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryAccounts()
	if err != nil {
		t.Fatalf("query accounts: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 accounts, got %d", len(results))
	}
}

// --- Contact Tests ---

func TestContactCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create account first (foreign key)
	acct := &Account{
		ID: "acct-001", Name: "Test Corp", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := store.InsertAccount(acct); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	contact := &Contact{
		ID:         "cont-001",
		AccountID:  "acct-001",
		FirstName:  "Jane",
		LastName:   "Smith",
		Email:      "jane.smith@example.com",
		Phone:      "555-9876",
		Title:      "VP Engineering",
		Department: "Engineering",
		CreatedAt:  "2024-01-15T10:00:00Z",
	}

	if err := store.InsertContact(contact); err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	retrieved, err := store.GetContactByID("cont-001")
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}

	if retrieved.ID != contact.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, contact.ID)
	}
	if retrieved.AccountID != contact.AccountID {
		t.Errorf("AccountID mismatch: got %q, want %q", retrieved.AccountID, contact.AccountID)
	}
	if retrieved.FirstName != contact.FirstName {
		t.Errorf("FirstName mismatch: got %q, want %q", retrieved.FirstName, contact.FirstName)
	}
	if retrieved.LastName != contact.LastName {
		t.Errorf("LastName mismatch: got %q, want %q", retrieved.LastName, contact.LastName)
	}
	if retrieved.Email != contact.Email {
		t.Errorf("Email mismatch: got %q, want %q", retrieved.Email, contact.Email)
	}
	if retrieved.Phone != contact.Phone {
		t.Errorf("Phone mismatch: got %q, want %q", retrieved.Phone, contact.Phone)
	}
	if retrieved.Title != contact.Title {
		t.Errorf("Title mismatch: got %q, want %q", retrieved.Title, contact.Title)
	}
	if retrieved.Department != contact.Department {
		t.Errorf("Department mismatch: got %q, want %q", retrieved.Department, contact.Department)
	}

	// Test QueryContacts
	contacts, err := store.QueryContacts()
	if err != nil {
		t.Fatalf("query contacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Errorf("expected 1 contact, got %d", len(contacts))
	}
}

func TestContactBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create account first
	if err := store.InsertAccount(&Account{
		ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	contacts := []Contact{
		{ID: "cont-001", AccountID: "acct-001", FirstName: "A", LastName: "A", Email: "a@test.com", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "cont-002", AccountID: "acct-001", FirstName: "B", LastName: "B", Email: "b@test.com", CreatedAt: "2024-01-02T00:00:00Z"},
	}

	if err := store.InsertContactsBatch(tx, contacts); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert contacts: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryContacts()
	if err != nil {
		t.Fatalf("query contacts: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 contacts, got %d", len(results))
	}
}

// --- User Tests ---

func TestUserCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	user := &User{
		ID:         "user-001",
		FirstName:  "John",
		LastName:   "Doe",
		Email:      "john.doe@example.com",
		Username:   "jdoe",
		Title:      "Engineer",
		Department: "Engineering",
		IsActive:   true,
		ManagerID:  "",
		UserRole:   "Standard",
		CreatedAt:  "2024-01-15T10:00:00Z",
	}

	if err := store.InsertUser(user); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	retrieved, err := store.GetUserByID("user-001")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, user.ID)
	}
	if retrieved.FirstName != user.FirstName {
		t.Errorf("FirstName mismatch: got %q, want %q", retrieved.FirstName, user.FirstName)
	}
	if retrieved.LastName != user.LastName {
		t.Errorf("LastName mismatch: got %q, want %q", retrieved.LastName, user.LastName)
	}
	if retrieved.Email != user.Email {
		t.Errorf("Email mismatch: got %q, want %q", retrieved.Email, user.Email)
	}
	if retrieved.Username != user.Username {
		t.Errorf("Username mismatch: got %q, want %q", retrieved.Username, user.Username)
	}
	if !retrieved.IsActive {
		t.Error("expected user to be active")
	}
	if retrieved.ManagerID != "" {
		t.Errorf("expected empty ManagerID, got %q", retrieved.ManagerID)
	}

	users, err := store.QueryUsers()
	if err != nil {
		t.Fatalf("query users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestUserWithManager(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Insert manager first
	manager := &User{
		ID: "user-mgr", FirstName: "Manager", LastName: "One",
		Email: "mgr@test.com", Username: "mgr", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := store.InsertUser(manager); err != nil {
		t.Fatalf("insert manager: %v", err)
	}

	// Insert user with manager
	user := &User{
		ID: "user-001", FirstName: "Employee", LastName: "One",
		Email: "emp@test.com", Username: "emp", IsActive: true,
		ManagerID: "user-mgr", CreatedAt: "2024-01-02T00:00:00Z",
	}
	if err := store.InsertUser(user); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	retrieved, err := store.GetUserByID("user-001")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if retrieved.ManagerID != "user-mgr" {
		t.Errorf("ManagerID mismatch: got %q, want %q", retrieved.ManagerID, "user-mgr")
	}
}

func TestUserBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	users := []User{
		{ID: "user-001", FirstName: "A", LastName: "A", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "user-002", FirstName: "B", LastName: "B", Email: "b@test.com", Username: "b", IsActive: false, CreatedAt: "2024-01-02T00:00:00Z"},
	}

	if err := store.InsertUsersBatch(tx, users); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert users: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryUsers()
	if err != nil {
		t.Fatalf("query users: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 users, got %d", len(results))
	}
}


// --- Case Tests ---

func TestCaseCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup prerequisites
	if err := store.InsertAccount(&Account{
		ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if err := store.InsertContact(&Contact{
		ID: "cont-001", AccountID: "acct-001", FirstName: "Jane", LastName: "Doe",
		Email: "jane@test.com", CreatedAt: "2024-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("insert contact: %v", err)
	}
	if err := store.InsertUser(&User{
		ID: "user-001", FirstName: "Agent", LastName: "One",
		Email: "agent@test.com", Username: "agent", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	c := &Case{
		ID:           "case-001",
		CaseNumber:   "00001234",
		Subject:      "Test Case Subject",
		Description:  "This is a test case description.",
		Status:       "Open",
		Priority:     "High",
		Product:      "Widget Pro",
		CaseType:     "Problem",
		Origin:       "Email",
		Reason:       "User Error",
		OwnerID:      "user-001",
		ContactID:    "cont-001",
		AccountID:    "acct-001",
		CreatedAt:    "2024-01-15T10:00:00Z",
		ClosedAt:     "",
		IsClosed:     false,
		IsEscalated:  true,
		JiraIssueKey: "SUPPORT-123",
	}

	if err := store.InsertCase(c); err != nil {
		t.Fatalf("insert case: %v", err)
	}

	retrieved, err := store.GetCaseByID("case-001")
	if err != nil {
		t.Fatalf("get case: %v", err)
	}

	if retrieved.ID != c.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, c.ID)
	}
	if retrieved.CaseNumber != c.CaseNumber {
		t.Errorf("CaseNumber mismatch: got %q, want %q", retrieved.CaseNumber, c.CaseNumber)
	}
	if retrieved.Subject != c.Subject {
		t.Errorf("Subject mismatch: got %q, want %q", retrieved.Subject, c.Subject)
	}
	if retrieved.Status != c.Status {
		t.Errorf("Status mismatch: got %q, want %q", retrieved.Status, c.Status)
	}
	if retrieved.Priority != c.Priority {
		t.Errorf("Priority mismatch: got %q, want %q", retrieved.Priority, c.Priority)
	}
	if retrieved.Product != c.Product {
		t.Errorf("Product mismatch: got %q, want %q", retrieved.Product, c.Product)
	}
	if retrieved.IsClosed != c.IsClosed {
		t.Errorf("IsClosed mismatch: got %v, want %v", retrieved.IsClosed, c.IsClosed)
	}
	if retrieved.IsEscalated != c.IsEscalated {
		t.Errorf("IsEscalated mismatch: got %v, want %v", retrieved.IsEscalated, c.IsEscalated)
	}
	if retrieved.JiraIssueKey != c.JiraIssueKey {
		t.Errorf("JiraIssueKey mismatch: got %q, want %q", retrieved.JiraIssueKey, c.JiraIssueKey)
	}

	cases, err := store.QueryCases()
	if err != nil {
		t.Fatalf("query cases: %v", err)
	}
	if len(cases) != 1 {
		t.Errorf("expected 1 case, got %d", len(cases))
	}
}

func TestCaseBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup prerequisites
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	cases := []Case{
		{ID: "case-001", CaseNumber: "00001", Subject: "Case 1", Description: "Desc 1", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "case-002", CaseNumber: "00002", Subject: "Case 2", Description: "Desc 2", Status: "Closed", Priority: "Low", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-02T00:00:00Z", IsClosed: true},
	}

	if err := store.InsertCasesBatch(tx, cases); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert cases: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryCases()
	if err != nil {
		t.Fatalf("query cases: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 cases, got %d", len(results))
	}
}

// --- Email Tests ---

func TestEmailCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup prerequisites (case requires account, contact, user)
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertCase(&Case{ID: "case-001", CaseNumber: "00001", Subject: "S", Description: "D", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"})

	email := &Email{
		ID:            "email-001",
		CaseID:        "case-001",
		Subject:       "RE: Your Issue",
		TextBody:      "Thank you for contacting us.",
		HTMLBody:      "<p>Thank you for contacting us.</p>",
		FromAddress:   "support@example.com",
		FromName:      "Support Team",
		ToAddress:     "customer@example.com",
		CCAddress:     "manager@example.com",
		BCCAddress:    "",
		MessageDate:   "2024-01-15T10:00:00Z",
		Status:        "Sent",
		Incoming:      false,
		HasAttachment: true,
		Headers:       "X-Custom: value",
		SequenceNum:   1,
	}

	if err := store.InsertEmail(email); err != nil {
		t.Fatalf("insert email: %v", err)
	}

	emails, err := store.QueryEmails()
	if err != nil {
		t.Fatalf("query emails: %v", err)
	}
	if len(emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(emails))
	}

	retrieved := emails[0]
	if retrieved.ID != email.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, email.ID)
	}
	if retrieved.Subject != email.Subject {
		t.Errorf("Subject mismatch: got %q, want %q", retrieved.Subject, email.Subject)
	}
	if retrieved.Incoming != email.Incoming {
		t.Errorf("Incoming mismatch: got %v, want %v", retrieved.Incoming, email.Incoming)
	}
	if retrieved.HasAttachment != email.HasAttachment {
		t.Errorf("HasAttachment mismatch: got %v, want %v", retrieved.HasAttachment, email.HasAttachment)
	}
	if retrieved.SequenceNum != email.SequenceNum {
		t.Errorf("SequenceNum mismatch: got %d, want %d", retrieved.SequenceNum, email.SequenceNum)
	}
}

func TestEmailBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertCase(&Case{ID: "case-001", CaseNumber: "00001", Subject: "S", Description: "D", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"})

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	emails := []Email{
		{ID: "email-001", CaseID: "case-001", Subject: "Email 1", TextBody: "Body 1", FromAddress: "a@test.com", FromName: "A", ToAddress: "b@test.com", MessageDate: "2024-01-01T00:00:00Z", Status: "Sent", Incoming: true, SequenceNum: 1},
		{ID: "email-002", CaseID: "case-001", Subject: "Email 2", TextBody: "Body 2", FromAddress: "b@test.com", FromName: "B", ToAddress: "a@test.com", MessageDate: "2024-01-02T00:00:00Z", Status: "Sent", Incoming: false, SequenceNum: 2},
	}

	if err := store.InsertEmailsBatch(tx, emails); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert emails: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryEmails()
	if err != nil {
		t.Fatalf("query emails: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 emails, got %d", len(results))
	}
}



// --- Comment Tests ---

func TestCommentCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertCase(&Case{ID: "case-001", CaseNumber: "00001", Subject: "S", Description: "D", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"})

	comment := &Comment{
		ID:          "comment-001",
		CaseID:      "case-001",
		CommentBody: "This is an internal note about the case.",
		CreatedByID: "user-001",
		CreatedAt:   "2024-01-15T11:00:00Z",
		IsPublished: false,
	}

	if err := store.InsertComment(comment); err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	comments, err := store.QueryComments()
	if err != nil {
		t.Fatalf("query comments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	retrieved := comments[0]
	if retrieved.ID != comment.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, comment.ID)
	}
	if retrieved.CommentBody != comment.CommentBody {
		t.Errorf("CommentBody mismatch: got %q, want %q", retrieved.CommentBody, comment.CommentBody)
	}
	if retrieved.IsPublished != comment.IsPublished {
		t.Errorf("IsPublished mismatch: got %v, want %v", retrieved.IsPublished, comment.IsPublished)
	}
}

func TestCommentBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertCase(&Case{ID: "case-001", CaseNumber: "00001", Subject: "S", Description: "D", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"})

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	comments := []Comment{
		{ID: "comment-001", CaseID: "case-001", CommentBody: "Comment 1", CreatedByID: "user-001", CreatedAt: "2024-01-01T00:00:00Z", IsPublished: false},
		{ID: "comment-002", CaseID: "case-001", CommentBody: "Comment 2", CreatedByID: "user-001", CreatedAt: "2024-01-02T00:00:00Z", IsPublished: true},
	}

	if err := store.InsertCommentsBatch(tx, comments); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert comments: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryComments()
	if err != nil {
		t.Fatalf("query comments: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 comments, got %d", len(results))
	}
}

// --- FeedItem Tests ---

func TestFeedItemCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertCase(&Case{ID: "case-001", CaseNumber: "00001", Subject: "S", Description: "D", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"})

	feedItem := &FeedItem{
		ID:          "feed-001",
		CaseID:      "case-001",
		Body:        "Case escalated to tier 2 support.",
		Type:        "TextPost",
		CreatedByID: "user-001",
		CreatedAt:   "2024-01-15T12:00:00Z",
	}

	if err := store.InsertFeedItem(feedItem); err != nil {
		t.Fatalf("insert feed item: %v", err)
	}

	items, err := store.QueryFeedItems()
	if err != nil {
		t.Fatalf("query feed items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 feed item, got %d", len(items))
	}

	retrieved := items[0]
	if retrieved.ID != feedItem.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, feedItem.ID)
	}
	if retrieved.Body != feedItem.Body {
		t.Errorf("Body mismatch: got %q, want %q", retrieved.Body, feedItem.Body)
	}
	if retrieved.Type != feedItem.Type {
		t.Errorf("Type mismatch: got %q, want %q", retrieved.Type, feedItem.Type)
	}
}

func TestFeedItemBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertContact(&Contact{ID: "cont-001", AccountID: "acct-001", FirstName: "J", LastName: "D", Email: "j@test.com", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertCase(&Case{ID: "case-001", CaseNumber: "00001", Subject: "S", Description: "D", Status: "Open", Priority: "High", OwnerID: "user-001", ContactID: "cont-001", AccountID: "acct-001", CreatedAt: "2024-01-01T00:00:00Z"})

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	items := []FeedItem{
		{ID: "feed-001", CaseID: "case-001", Body: "Update 1", Type: "TextPost", CreatedByID: "user-001", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "feed-002", CaseID: "case-001", Body: "Update 2", Type: "TrackedChange", CreatedByID: "user-001", CreatedAt: "2024-01-02T00:00:00Z"},
	}

	if err := store.InsertFeedItemsBatch(tx, items); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert feed items: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryFeedItems()
	if err != nil {
		t.Fatalf("query feed items: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 feed items, got %d", len(results))
	}
}


// --- JiraUser Tests ---

func TestJiraUserCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Optionally create SF user for linking
	store.InsertUser(&User{ID: "user-001", FirstName: "A", LastName: "B", Email: "a@test.com", Username: "a", IsActive: true, CreatedAt: "2024-01-01T00:00:00Z"})

	jiraUser := &JiraUser{
		AccountID:   "jira-user-001",
		DisplayName: "John Developer",
		Email:       "john.dev@example.com",
		AccountType: "atlassian",
		Active:      true,
		SFUserID:    "user-001",
	}

	if err := store.InsertJiraUser(jiraUser); err != nil {
		t.Fatalf("insert jira user: %v", err)
	}

	users, err := store.QueryJiraUsers()
	if err != nil {
		t.Fatalf("query jira users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 jira user, got %d", len(users))
	}

	retrieved := users[0]
	if retrieved.AccountID != jiraUser.AccountID {
		t.Errorf("AccountID mismatch: got %q, want %q", retrieved.AccountID, jiraUser.AccountID)
	}
	if retrieved.DisplayName != jiraUser.DisplayName {
		t.Errorf("DisplayName mismatch: got %q, want %q", retrieved.DisplayName, jiraUser.DisplayName)
	}
	if retrieved.Email != jiraUser.Email {
		t.Errorf("Email mismatch: got %q, want %q", retrieved.Email, jiraUser.Email)
	}
	if retrieved.Active != jiraUser.Active {
		t.Errorf("Active mismatch: got %v, want %v", retrieved.Active, jiraUser.Active)
	}
	if retrieved.SFUserID != jiraUser.SFUserID {
		t.Errorf("SFUserID mismatch: got %q, want %q", retrieved.SFUserID, jiraUser.SFUserID)
	}
}

func TestJiraUserBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	// Note: SFUserID is optional and empty string converts to NULL
	users := []JiraUser{
		{AccountID: "jira-001", DisplayName: "User A", Email: "a@jira.com", AccountType: "atlassian", Active: true},
		{AccountID: "jira-002", DisplayName: "User B", Email: "b@jira.com", AccountType: "atlassian", Active: false},
	}

	if err := store.InsertJiraUsersBatch(tx, users); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert jira users: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryJiraUsers()
	if err != nil {
		t.Fatalf("query jira users: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 jira users, got %d", len(results))
	}
}

// --- JiraIssue Tests ---

func TestJiraIssueCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup jira user for assignee/reporter (SFUserID empty -> NULL)
	store.InsertJiraUser(&JiraUser{AccountID: "jira-user-001", DisplayName: "Developer", Email: "dev@test.com", AccountType: "atlassian", Active: true})

	issue := &JiraIssue{
		ID:             "issue-001",
		Key:            "SUPPORT-123",
		ProjectKey:     "SUPPORT",
		Summary:        "Customer cannot login",
		DescriptionADF: `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"User reports login issues"}]}]}`,
		IssueType:      "Bug",
		Status:         "In Progress",
		Priority:       "High",
		AssigneeID:     "jira-user-001",
		ReporterID:     "jira-user-001",
		CreatedAt:      "2024-01-15T10:00:00Z",
		UpdatedAt:      "2024-01-16T10:00:00Z",
		Labels:         `["customer-facing","urgent"]`,
		SFCaseID:       "",
	}

	if err := store.InsertJiraIssue(issue); err != nil {
		t.Fatalf("insert jira issue: %v", err)
	}

	issues, err := store.QueryJiraIssues()
	if err != nil {
		t.Fatalf("query jira issues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 jira issue, got %d", len(issues))
	}

	retrieved := issues[0]
	if retrieved.ID != issue.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, issue.ID)
	}
	if retrieved.Key != issue.Key {
		t.Errorf("Key mismatch: got %q, want %q", retrieved.Key, issue.Key)
	}
	if retrieved.ProjectKey != issue.ProjectKey {
		t.Errorf("ProjectKey mismatch: got %q, want %q", retrieved.ProjectKey, issue.ProjectKey)
	}
	if retrieved.Summary != issue.Summary {
		t.Errorf("Summary mismatch: got %q, want %q", retrieved.Summary, issue.Summary)
	}
	if retrieved.IssueType != issue.IssueType {
		t.Errorf("IssueType mismatch: got %q, want %q", retrieved.IssueType, issue.IssueType)
	}
	if retrieved.Status != issue.Status {
		t.Errorf("Status mismatch: got %q, want %q", retrieved.Status, issue.Status)
	}
	if retrieved.Priority != issue.Priority {
		t.Errorf("Priority mismatch: got %q, want %q", retrieved.Priority, issue.Priority)
	}
}

func TestJiraIssueBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	// Note: AssigneeID, ReporterID, SFCaseID empty -> NULL
	issues := []JiraIssue{
		{ID: "issue-001", Key: "SUPPORT-1", ProjectKey: "SUPPORT", Summary: "Issue 1", DescriptionADF: "{}", IssueType: "Bug", Status: "To Do", Priority: "High", CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"},
		{ID: "issue-002", Key: "SUPPORT-2", ProjectKey: "SUPPORT", Summary: "Issue 2", DescriptionADF: "{}", IssueType: "Task", Status: "Done", Priority: "Low", CreatedAt: "2024-01-02T00:00:00Z", UpdatedAt: "2024-01-03T00:00:00Z"},
	}

	if err := store.InsertJiraIssuesBatch(tx, issues); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert jira issues: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryJiraIssues()
	if err != nil {
		t.Fatalf("query jira issues: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 jira issues, got %d", len(results))
	}
}

// --- JiraComment Tests ---

func TestJiraCommentCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup (Jira user -> Jira issue, SFUserID empty -> NULL)
	store.InsertJiraUser(&JiraUser{AccountID: "jira-user-001", DisplayName: "Developer", Email: "dev@test.com", AccountType: "atlassian", Active: true})
	store.InsertJiraIssue(&JiraIssue{ID: "issue-001", Key: "SUPPORT-1", ProjectKey: "SUPPORT", Summary: "Test", DescriptionADF: "{}", IssueType: "Bug", Status: "Open", Priority: "Medium", CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"})

	comment := &JiraComment{
		ID:        "jira-comment-001",
		IssueID:   "issue-001",
		AuthorID:  "jira-user-001",
		BodyADF:   `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Working on this now."}]}]}`,
		CreatedAt: "2024-01-15T11:00:00Z",
		UpdatedAt: "2024-01-15T11:00:00Z",
	}

	if err := store.InsertJiraComment(comment); err != nil {
		t.Fatalf("insert jira comment: %v", err)
	}

	comments, err := store.QueryJiraComments()
	if err != nil {
		t.Fatalf("query jira comments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 jira comment, got %d", len(comments))
	}

	retrieved := comments[0]
	if retrieved.ID != comment.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, comment.ID)
	}
	if retrieved.IssueID != comment.IssueID {
		t.Errorf("IssueID mismatch: got %q, want %q", retrieved.IssueID, comment.IssueID)
	}
	if retrieved.AuthorID != comment.AuthorID {
		t.Errorf("AuthorID mismatch: got %q, want %q", retrieved.AuthorID, comment.AuthorID)
	}
}

func TestJiraCommentBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Setup (Jira user -> Jira issue, SFUserID empty -> NULL)
	store.InsertJiraUser(&JiraUser{AccountID: "jira-user-001", DisplayName: "Developer", Email: "dev@test.com", AccountType: "atlassian", Active: true})
	store.InsertJiraIssue(&JiraIssue{ID: "issue-001", Key: "SUPPORT-1", ProjectKey: "SUPPORT", Summary: "Test", DescriptionADF: "{}", IssueType: "Bug", Status: "Open", Priority: "Medium", CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"})

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	comments := []JiraComment{
		{ID: "jc-001", IssueID: "issue-001", AuthorID: "jira-user-001", BodyADF: `{"text":"Comment 1"}`, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"},
		{ID: "jc-002", IssueID: "issue-001", AuthorID: "jira-user-001", BodyADF: `{"text":"Comment 2"}`, CreatedAt: "2024-01-02T00:00:00Z", UpdatedAt: "2024-01-02T00:00:00Z"},
	}

	if err := store.InsertJiraCommentsBatch(tx, comments); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert jira comments: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryJiraComments()
	if err != nil {
		t.Fatalf("query jira comments: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 jira comments, got %d", len(results))
	}
}

// --- Store Operations Tests ---

func TestStoreStats(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Insert some data
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})
	store.InsertAccount(&Account{ID: "acct-002", Name: "Test2", Industry: "Finance", Type: "Partner", CreatedAt: "2024-01-01T00:00:00Z"})

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats["accounts"] != 2 {
		t.Errorf("expected 2 accounts in stats, got %d", stats["accounts"])
	}
	if stats["contacts"] != 0 {
		t.Errorf("expected 0 contacts in stats, got %d", stats["contacts"])
	}
}

func TestStoreReset(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Insert data
	store.InsertAccount(&Account{ID: "acct-001", Name: "Test", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"})

	// Verify data exists
	accounts, _ := store.QueryAccounts()
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account before reset")
	}

	// Reset
	if err := store.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// Verify data is gone
	accounts, _ = store.QueryAccounts()
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts after reset, got %d", len(accounts))
	}
}

func TestTransactionRollback(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	accounts := []Account{
		{ID: "acct-001", Name: "Company A", Industry: "Tech", Type: "Customer", CreatedAt: "2024-01-01T00:00:00Z"},
	}

	if err := store.InsertAccountsBatch(tx, accounts); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert: %v", err)
	}

	// Rollback instead of commit
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Verify data was NOT inserted
	results, err := store.QueryAccounts()
	if err != nil {
		t.Fatalf("query accounts: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 accounts after rollback, got %d", len(results))
	}
}

// --- ProfileImage Tests ---

func TestProfileImageCRUD(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	img := &ProfileImage{
		ID:          "img-001",
		PersonaType: "contact",
		PersonaID:   "cont-001",
		ImagePath:   "assets/profile_images/img-001.png",
		FirstName:   "Jane",
		LastName:    "Smith",
		Age:         34,
		Gender:      "female",
		Ethnicity:   "Asian",
		HairColor:   "Black",
		HairStyle:   "long",
		EyeColor:    "Brown",
		Glasses:     true,
		FacialHair:  "",
		GeneratedAt: "2024-01-15T10:00:00Z",
		Prompt:      "Professional corporate headshot of a 34-year-old Asian female...",
	}

	if err := store.InsertProfileImage(img); err != nil {
		t.Fatalf("insert profile image: %v", err)
	}

	// Test GetProfileImageByID
	retrieved, err := store.GetProfileImageByID("img-001")
	if err != nil {
		t.Fatalf("get profile image by ID: %v", err)
	}

	// Verify all fields round-trip correctly
	if retrieved.ID != img.ID {
		t.Errorf("ID mismatch: got %q, want %q", retrieved.ID, img.ID)
	}
	if retrieved.PersonaType != img.PersonaType {
		t.Errorf("PersonaType mismatch: got %q, want %q", retrieved.PersonaType, img.PersonaType)
	}
	if retrieved.PersonaID != img.PersonaID {
		t.Errorf("PersonaID mismatch: got %q, want %q", retrieved.PersonaID, img.PersonaID)
	}
	if retrieved.ImagePath != img.ImagePath {
		t.Errorf("ImagePath mismatch: got %q, want %q", retrieved.ImagePath, img.ImagePath)
	}
	if retrieved.FirstName != img.FirstName {
		t.Errorf("FirstName mismatch: got %q, want %q", retrieved.FirstName, img.FirstName)
	}
	if retrieved.LastName != img.LastName {
		t.Errorf("LastName mismatch: got %q, want %q", retrieved.LastName, img.LastName)
	}
	if retrieved.Age != img.Age {
		t.Errorf("Age mismatch: got %d, want %d", retrieved.Age, img.Age)
	}
	if retrieved.Gender != img.Gender {
		t.Errorf("Gender mismatch: got %q, want %q", retrieved.Gender, img.Gender)
	}
	if retrieved.Ethnicity != img.Ethnicity {
		t.Errorf("Ethnicity mismatch: got %q, want %q", retrieved.Ethnicity, img.Ethnicity)
	}
	if retrieved.HairColor != img.HairColor {
		t.Errorf("HairColor mismatch: got %q, want %q", retrieved.HairColor, img.HairColor)
	}
	if retrieved.HairStyle != img.HairStyle {
		t.Errorf("HairStyle mismatch: got %q, want %q", retrieved.HairStyle, img.HairStyle)
	}
	if retrieved.EyeColor != img.EyeColor {
		t.Errorf("EyeColor mismatch: got %q, want %q", retrieved.EyeColor, img.EyeColor)
	}
	if retrieved.Glasses != img.Glasses {
		t.Errorf("Glasses mismatch: got %v, want %v", retrieved.Glasses, img.Glasses)
	}
	if retrieved.FacialHair != img.FacialHair {
		t.Errorf("FacialHair mismatch: got %q, want %q", retrieved.FacialHair, img.FacialHair)
	}
	if retrieved.GeneratedAt != img.GeneratedAt {
		t.Errorf("GeneratedAt mismatch: got %q, want %q", retrieved.GeneratedAt, img.GeneratedAt)
	}
	if retrieved.Prompt != img.Prompt {
		t.Errorf("Prompt mismatch: got %q, want %q", retrieved.Prompt, img.Prompt)
	}

	// Test QueryAllProfileImages
	images, err := store.QueryAllProfileImages()
	if err != nil {
		t.Fatalf("query all profile images: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 profile image, got %d", len(images))
	}
}

func TestProfileImageQueryByPersona(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Insert profile image for a contact
	contactImg := &ProfileImage{
		ID:          "img-001",
		PersonaType: "contact",
		PersonaID:   "cont-001",
		ImagePath:   "assets/profile_images/img-001.png",
		FirstName:   "Jane",
		LastName:    "Doe",
		Gender:      "female",
		GeneratedAt: "2024-01-15T10:00:00Z",
	}
	if err := store.InsertProfileImage(contactImg); err != nil {
		t.Fatalf("insert contact profile image: %v", err)
	}

	// Insert profile image for a user
	userImg := &ProfileImage{
		ID:          "img-002",
		PersonaType: "user",
		PersonaID:   "user-001",
		ImagePath:   "assets/profile_images/img-002.png",
		FirstName:   "John",
		LastName:    "Smith",
		Gender:      "male",
		GeneratedAt: "2024-01-15T11:00:00Z",
	}
	if err := store.InsertProfileImage(userImg); err != nil {
		t.Fatalf("insert user profile image: %v", err)
	}

	// Query by contact persona
	retrieved, err := store.QueryProfileImageByPersona("contact", "cont-001")
	if err != nil {
		t.Fatalf("query by contact persona: %v", err)
	}
	if retrieved.ID != "img-001" {
		t.Errorf("expected img-001, got %q", retrieved.ID)
	}
	if retrieved.FirstName != "Jane" {
		t.Errorf("expected Jane, got %q", retrieved.FirstName)
	}

	// Query by user persona
	retrieved, err = store.QueryProfileImageByPersona("user", "user-001")
	if err != nil {
		t.Fatalf("query by user persona: %v", err)
	}
	if retrieved.ID != "img-002" {
		t.Errorf("expected img-002, got %q", retrieved.ID)
	}
	if retrieved.FirstName != "John" {
		t.Errorf("expected John, got %q", retrieved.FirstName)
	}

	// Query non-existent persona
	_, err = store.QueryProfileImageByPersona("contact", "non-existent")
	if err == nil {
		t.Error("expected error for non-existent persona")
	}
}

func TestProfileImageWithOptionalFields(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Test profile image with only required fields
	img := &ProfileImage{
		ID:          "img-minimal",
		PersonaType: "user",
		PersonaID:   "user-001",
		ImagePath:   "assets/profile_images/img-minimal.png",
		// All optional fields left empty/zero
	}

	if err := store.InsertProfileImage(img); err != nil {
		t.Fatalf("insert minimal profile image: %v", err)
	}

	retrieved, err := store.GetProfileImageByID("img-minimal")
	if err != nil {
		t.Fatalf("get minimal profile image: %v", err)
	}

	if retrieved.FirstName != "" {
		t.Errorf("expected empty FirstName, got %q", retrieved.FirstName)
	}
	if retrieved.Age != 0 {
		t.Errorf("expected 0 Age, got %d", retrieved.Age)
	}
	if retrieved.Glasses != false {
		t.Errorf("expected false Glasses, got %v", retrieved.Glasses)
	}
}

func TestProfileImageBatchInsert(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	tx, err := store.BeginTx()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	images := []ProfileImage{
		{ID: "img-001", PersonaType: "contact", PersonaID: "cont-001", ImagePath: "assets/profile_images/img-001.png", FirstName: "Alice", Gender: "female", Glasses: true, GeneratedAt: "2024-01-01T00:00:00Z"},
		{ID: "img-002", PersonaType: "contact", PersonaID: "cont-002", ImagePath: "assets/profile_images/img-002.png", FirstName: "Bob", Gender: "male", Glasses: false, GeneratedAt: "2024-01-02T00:00:00Z"},
		{ID: "img-003", PersonaType: "user", PersonaID: "user-001", ImagePath: "assets/profile_images/img-003.png", FirstName: "Charlie", Gender: "male", Glasses: true, GeneratedAt: "2024-01-03T00:00:00Z"},
	}

	if err := store.InsertProfileImagesBatch(tx, images); err != nil {
		tx.Rollback()
		t.Fatalf("batch insert profile images: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	results, err := store.QueryAllProfileImages()
	if err != nil {
		t.Fatalf("query profile images: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 profile images, got %d", len(results))
	}
}

func TestProfileImageUniqueConstraint(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	img1 := &ProfileImage{
		ID:          "img-001",
		PersonaType: "contact",
		PersonaID:   "cont-001",
		ImagePath:   "assets/profile_images/img-001.png",
	}
	if err := store.InsertProfileImage(img1); err != nil {
		t.Fatalf("insert first profile image: %v", err)
	}

	// Try to insert another image for the same persona (should fail)
	img2 := &ProfileImage{
		ID:          "img-002",
		PersonaType: "contact",
		PersonaID:   "cont-001", // Same persona
		ImagePath:   "assets/profile_images/img-002.png",
	}
	err := store.InsertProfileImage(img2)
	if err == nil {
		t.Error("expected unique constraint violation for duplicate persona")
	}
}

func TestMigrateProfileImages(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Migration should be idempotent - can run multiple times
	if err := store.MigrateProfileImages(); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := store.MigrateProfileImages(); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	// Verify table exists and is usable
	img := &ProfileImage{
		ID:          "img-001",
		PersonaType: "contact",
		PersonaID:   "cont-001",
		ImagePath:   "assets/profile_images/img-001.png",
	}
	if err := store.InsertProfileImage(img); err != nil {
		t.Fatalf("insert after migration: %v", err)
	}

	images, err := store.QueryAllProfileImages()
	if err != nil {
		t.Fatalf("query after migration: %v", err)
	}
	if len(images) != 1 {
		t.Errorf("expected 1 image after migration, got %d", len(images))
	}
}
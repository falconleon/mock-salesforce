package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/store"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// setupQueryTest creates a memory store with test data
func setupQueryTest(t *testing.T) store.Store {
	memStore := store.NewMemoryStore()
	_ = zerolog.Nop() // logger not needed with direct LoadRecords

	// Manually load test data without relying on file paths
	testCases := []store.Record{
		{
			"Id":          "5003t00002AbCdEAAV",
			"CaseNumber":  "00123456",
			"Subject":     "Unable to access dashboard after update",
			"Priority":    "P1",
			"Product__c":  "Workspace ONE",
			"Status":      "In Progress",
			"IsClosed":    false,
		},
		{
			"Id":          "5003t00002XyZAbAAV",
			"CaseNumber":  "00123457",
			"Subject":     "Feature request: Export to PDF",
			"Priority":    "P3",
			"Product__c":  "Analytics Platform",
			"Status":      "Closed",
			"IsClosed":    true,
		},
		{
			"Id":          "5003t00002CdEfGAAV",
			"CaseNumber":  "00123458",
			"Subject":     "Integration sync failing",
			"Priority":    "P1",
			"Product__c":  "API Platform",
			"Status":      "Escalated",
			"IsClosed":    false,
		},
	}

	testAccounts := []store.Record{
		{
			"Id":   "0013t00002AbCdEAAV",
			"Name": "Acme Corporation",
		},
		{
			"Id":   "0013t00002FgHiJAAV",
			"Name": "GlobalRetail Inc",
		},
	}

	if err := memStore.LoadRecords("Case", testCases); err != nil {
		t.Fatalf("failed to load test case data: %v", err)
	}

	if err := memStore.LoadRecords("Account", testAccounts); err != nil {
		t.Fatalf("failed to load test account data: %v", err)
	}

	return memStore
}

// TestQueryHandler_HandleQuery_SimpleSelect tests a basic SELECT * query
func TestQueryHandler_HandleQuery_SimpleSelect(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject+FROM+Case", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalSize == 0 {
		t.Error("expected records in response")
	}
	if !resp.Done {
		t.Error("expected done flag to be true")
	}
}

// TestQueryHandler_HandleQuery_WithWhere tests SELECT with WHERE clause
func TestQueryHandler_HandleQuery_WithWhere(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Query for cases with P1 priority
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject,Priority+FROM+Case+WHERE+Priority='P1'", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all returned records have P1 priority
	for _, record := range resp.Records {
		if priority, ok := record["Priority"]; ok {
			if priority != "P1" {
				t.Errorf("expected priority P1, got %v", priority)
			}
		}
	}
}

// TestQueryHandler_HandleQuery_WithOrderBy tests SELECT with ORDER BY
func TestQueryHandler_HandleQuery_WithOrderBy(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject+FROM+Case+ORDER+BY+CaseNumber+DESC+LIMIT+5", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalSize > 5 {
		t.Errorf("expected at most 5 records, got %d", resp.TotalSize)
	}
}

// TestQueryHandler_HandleQuery_WithLimit tests SELECT with LIMIT
func TestQueryHandler_HandleQuery_WithLimit(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject+FROM+Case+LIMIT+10", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalSize > 10 {
		t.Errorf("expected at most 10 records, got %d", resp.TotalSize)
	}
}

// TestQueryHandler_HandleQuery_ComplexQuery tests a more complex query with WHERE and ORDER BY
func TestQueryHandler_HandleQuery_ComplexQuery(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Query for non-closed cases with P1 or P2 priority, ordered by status
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject,Status,Priority+FROM+Case+WHERE+IsClosed=false+AND+(Priority='P1'+OR+Priority='P2')+ORDER+BY+Status", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all records match filter criteria
	for _, record := range resp.Records {
		if isClosed, ok := record["IsClosed"].(bool); ok && isClosed {
			t.Error("expected IsClosed to be false for all records")
		}
	}
}

// TestQueryHandler_HandleQuery_NoResults tests a query that returns no results
func TestQueryHandler_HandleQuery_NoResults(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Query for a non-existent priority
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id+FROM+Case+WHERE+Priority='P99'", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalSize != 0 {
		t.Errorf("expected 0 records, got %d", resp.TotalSize)
	}
	if !resp.Done {
		t.Error("expected done flag to be true")
	}
}

// TestQueryHandler_HandleQuery_MissingQueryParameter tests error when query parameter is missing
func TestQueryHandler_HandleQuery_MissingQueryParameter(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	req := httptest.NewRequest("GET", "/services/data/v58.0/query", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var errResp []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if len(errResp) == 0 {
		t.Error("expected error response")
	}
	if errResp[0].ErrorCode != "MALFORMED_QUERY" {
		t.Errorf("expected MALFORMED_QUERY error code, got %s", errResp[0].ErrorCode)
	}
}

// TestQueryHandler_HandleQuery_InvalidSOQL tests error handling for invalid SOQL syntax
func TestQueryHandler_HandleQuery_InvalidSOQL(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Invalid SOQL - missing FROM clause
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid SOQL, got %d", rec.Code)
	}

	var errResp []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if len(errResp) == 0 {
		t.Error("expected error response for invalid SOQL")
	}
	if errResp[0].ErrorCode != "MALFORMED_QUERY" {
		t.Errorf("expected MALFORMED_QUERY error code, got %s", errResp[0].ErrorCode)
	}
}

// TestQueryHandler_HandleQuery_DifferentObject tests querying different object types
func TestQueryHandler_HandleQuery_DifferentObject(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Query accounts
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Name+FROM+Account", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalSize == 0 {
		t.Error("expected Account records in response")
	}
}

// TestQueryHandler_HandleQuery_ResponseFormat verifies response structure
func TestQueryHandler_HandleQuery_ResponseFormat(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject+FROM+Case+LIMIT+1", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Check Content-Type header
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if resp.TotalSize < 0 {
		t.Error("TotalSize should not be negative")
	}
	if resp.Records == nil {
		t.Error("Records should not be nil")
	}
}

// TestQueryHandler_HandleQuery_NotEquals tests != operator in WHERE clause
func TestQueryHandler_HandleQuery_NotEquals(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Query for cases that are not closed
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Subject+FROM+Case+WHERE+IsClosed!=true", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have at least one open case
	if resp.TotalSize == 0 {
		t.Error("expected records for open cases")
	}
}

// TestQueryHandler_HandleQuery_WithInOperator tests IN operator in WHERE clause
func TestQueryHandler_HandleQuery_WithInOperator(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	// Query for cases with P1 or P2 priority using IN
	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=SELECT+Id,Priority+FROM+Case+WHERE+Priority+IN+('P1','P2')", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.QueryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all records have P1 or P2 priority
	for _, record := range resp.Records {
		if priority, ok := record["Priority"]; ok {
			if priority != "P1" && priority != "P2" {
				t.Errorf("expected priority P1 or P2, got %v", priority)
			}
		}
	}
}

// TestQueryHandler_HandleQuery_EmptyQueryParameter tests error with empty query
func TestQueryHandler_HandleQuery_EmptyQueryParameter(t *testing.T) {
	memStore := setupQueryTest(t)
	logger := zerolog.Nop()
	handler := handlers.NewQueryHandler(memStore, logger)

	req := httptest.NewRequest("GET", "/services/data/v58.0/query?q=", nil)
	rec := httptest.NewRecorder()

	handler.HandleQuery(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

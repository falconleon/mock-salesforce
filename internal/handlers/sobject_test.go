package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/handlers"
	"github.com/falconleon/mock-salesforce/internal/store"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

func setupTestStore() *store.MemoryStore {
	s := store.NewMemoryStore()
	// Pre-populate with test data
	s.Create("Case", map[string]any{
		"Id":          "5003t00002TestAAA",
		"CaseNumber":  "00001234",
		"Subject":     "Test Case",
		"Status":      "New",
		"Priority":    "P2",
		"Description": "Test description",
	})
	s.Create("EmailMessage", map[string]any{
		"Id":          "02s3t00001TestAAA",
		"ParentId":    "5003t00002TestAAA",
		"Subject":     "Test Email",
		"TextBody":    "Test body",
		"FromAddress": "test@example.com",
		"ToAddress":   "support@example.com",
		"Incoming":    true,
	})
	return s
}

// Helper to set path values in request context (simulates mux routing)
func setPathValues(r *http.Request, values map[string]string) *http.Request {
	for k, v := range values {
		r.SetPathValue(k, v)
	}
	return r
}

func TestSObjectHandler_HandleGet_Success(t *testing.T) {
	s := setupTestStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/Case/5003t00002TestAAA", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "5003t00002TestAAA"})
	rec := httptest.NewRecorder()

	handler.HandleGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["Id"] != "5003t00002TestAAA" {
		t.Errorf("expected Id '5003t00002TestAAA', got '%v'", resp["Id"])
	}
	if resp["Subject"] != "Test Case" {
		t.Errorf("expected Subject 'Test Case', got '%v'", resp["Subject"])
	}

	// Check attributes are added
	attrs, ok := resp["attributes"].(map[string]any)
	if !ok {
		t.Fatal("expected attributes to be present")
	}
	if attrs["type"] != "Case" {
		t.Errorf("expected attributes.type 'Case', got '%v'", attrs["type"])
	}
}

func TestSObjectHandler_HandleGet_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/Case/nonexistent", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "nonexistent"})
	rec := httptest.NewRecorder()

	handler.HandleGet(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	var errors []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errors); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}

	if len(errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if errors[0].ErrorCode != "NOT_FOUND" {
		t.Errorf("expected error code 'NOT_FOUND', got '%s'", errors[0].ErrorCode)
	}
}

func TestSObjectHandler_HandleCreate_Success(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	body := map[string]any{
		"Subject":  "New Case",
		"Status":   "New",
		"Priority": "P1",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/services/data/v66.0/sobjects/Case", bytes.NewReader(bodyBytes))
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleCreate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp handlers.CreateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.ID == "" {
		t.Error("expected ID to be set")
	}
	if len(resp.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(resp.Errors))
	}

	// Verify record was created in store
	record, err := s.Get("Case", resp.ID)
	if err != nil {
		t.Fatalf("failed to get created record: %v", err)
	}
	if record["Subject"] != "New Case" {
		t.Errorf("expected Subject 'New Case', got '%v'", record["Subject"])
	}
}

func TestSObjectHandler_HandleCreate_EmptyBody(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("POST", "/services/data/v66.0/sobjects/Case", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleCreate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestSObjectHandler_HandleCreate_InvalidJSON(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("POST", "/services/data/v66.0/sobjects/Case", bytes.NewReader([]byte("not json")))
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleCreate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var errors []models.APIError
	if err := json.NewDecoder(rec.Body).Decode(&errors); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}

	if len(errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if errors[0].ErrorCode != "JSON_PARSER_ERROR" {
		t.Errorf("expected error code 'JSON_PARSER_ERROR', got '%s'", errors[0].ErrorCode)
	}
}

func TestSObjectHandler_HandleUpdate_Success(t *testing.T) {
	s := setupTestStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	body := map[string]any{
		"Status":   "In Progress",
		"Priority": "P1",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/services/data/v66.0/sobjects/Case/5003t00002TestAAA", bytes.NewReader(bodyBytes))
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "5003t00002TestAAA"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify record was updated
	record, _ := s.Get("Case", "5003t00002TestAAA")
	if record["Status"] != "In Progress" {
		t.Errorf("expected Status 'In Progress', got '%v'", record["Status"])
	}
	if record["Priority"] != "P1" {
		t.Errorf("expected Priority 'P1', got '%v'", record["Priority"])
	}
	// Original field should remain
	if record["Subject"] != "Test Case" {
		t.Errorf("expected Subject 'Test Case', got '%v'", record["Subject"])
	}
}

func TestSObjectHandler_HandleUpdate_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	body := map[string]any{"Status": "Closed"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/services/data/v66.0/sobjects/Case/nonexistent", bytes.NewReader(bodyBytes))
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "nonexistent"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestSObjectHandler_HandleUpdate_CannotChangeId(t *testing.T) {
	s := setupTestStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	body := map[string]any{
		"Id":     "5003t00002NewIdAAA",
		"Status": "In Progress",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/services/data/v66.0/sobjects/Case/5003t00002TestAAA", bytes.NewReader(bodyBytes))
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "5003t00002TestAAA"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandleUpdate(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}

	// Verify ID was not changed
	record, _ := s.Get("Case", "5003t00002TestAAA")
	if record["Id"] != "5003t00002TestAAA" {
		t.Errorf("expected Id to remain '5003t00002TestAAA', got '%v'", record["Id"])
	}
}

func TestSObjectHandler_HandleDelete_Success(t *testing.T) {
	s := setupTestStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("DELETE", "/services/data/v66.0/sobjects/Case/5003t00002TestAAA", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "5003t00002TestAAA"})
	rec := httptest.NewRecorder()

	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify record was deleted
	_, err := s.Get("Case", "5003t00002TestAAA")
	if err == nil {
		t.Error("expected record to be deleted")
	}
}

func TestSObjectHandler_HandleDelete_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("DELETE", "/services/data/v66.0/sobjects/Case/nonexistent", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case", "id": "nonexistent"})
	rec := httptest.NewRecorder()

	handler.HandleDelete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestSObjectHandler_HandleDescribe_KnownType(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/Case/describe", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "Case"})
	rec := httptest.NewRecorder()

	handler.HandleDescribe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["name"] != "Case" {
		t.Errorf("expected name 'Case', got '%v'", resp["name"])
	}
	if resp["label"] != "Case" {
		t.Errorf("expected label 'Case', got '%v'", resp["label"])
	}
	if resp["keyPrefix"] != "500" {
		t.Errorf("expected keyPrefix '500', got '%v'", resp["keyPrefix"])
	}

	fields, ok := resp["fields"].([]any)
	if !ok {
		t.Fatal("expected fields to be an array")
	}
	if len(fields) == 0 {
		t.Error("expected at least one field")
	}

	// Check for required Case fields
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		field := f.(map[string]any)
		fieldNames[field["name"].(string)] = true
	}

	requiredFields := []string{"Id", "Subject", "Status", "Priority", "Description"}
	for _, rf := range requiredFields {
		if !fieldNames[rf] {
			t.Errorf("expected field '%s' to be present", rf)
		}
	}
}

func TestSObjectHandler_HandleDescribe_AllTypes(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	types := []struct {
		name      string
		keyPrefix string
	}{
		{"Case", "500"},
		{"EmailMessage", "02s"},
		{"CaseComment", "00a"},
		{"FeedItem", "0D5"},
		{"Account", "001"},
		{"Contact", "003"},
		{"User", "005"},
	}

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/"+tc.name+"/describe", nil)
			req = setPathValues(req, map[string]string{"version": "v66.0", "type": tc.name})
			rec := httptest.NewRecorder()

			handler.HandleDescribe(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200 for %s, got %d", tc.name, rec.Code)
			}

			var resp map[string]any
			json.NewDecoder(rec.Body).Decode(&resp)

			if resp["keyPrefix"] != tc.keyPrefix {
				t.Errorf("expected keyPrefix '%s' for %s, got '%v'", tc.keyPrefix, tc.name, resp["keyPrefix"])
			}
		})
	}
}

func TestSObjectHandler_HandleDescribe_UnknownType(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/UnknownObject/describe", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "UnknownObject"})
	rec := httptest.NewRecorder()

	handler.HandleDescribe(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestSObjectHandler_HandleDescribe_DynamicType(t *testing.T) {
	s := store.NewMemoryStore()
	// Add some data for a custom object type
	s.Create("CustomObject__c", map[string]any{
		"Id":   "a003t00001TestAAA",
		"Name": "Custom Record",
	})
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	req := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/CustomObject__c/describe", nil)
	req = setPathValues(req, map[string]string{"version": "v66.0", "type": "CustomObject__c"})
	rec := httptest.NewRecorder()

	handler.HandleDescribe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for dynamic type, got %d", rec.Code)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["name"] != "CustomObject__c" {
		t.Errorf("expected name 'CustomObject__c', got '%v'", resp["name"])
	}
}

func TestSObjectHandler_CRUD_Integration(t *testing.T) {
	s := store.NewMemoryStore()
	logger := zerolog.Nop()
	handler := handlers.NewSObjectHandler(s, logger)

	// 1. Create
	createBody := map[string]any{
		"Subject":  "Integration Test Case",
		"Status":   "New",
		"Priority": "P3",
	}
	createBytes, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest("POST", "/services/data/v66.0/sobjects/Case", bytes.NewReader(createBytes))
	createReq = setPathValues(createReq, map[string]string{"version": "v66.0", "type": "Case"})
	createRec := httptest.NewRecorder()

	handler.HandleCreate(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d", createRec.Code)
	}

	var createResp handlers.CreateResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)
	id := createResp.ID

	// 2. Get
	getReq := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/Case/"+id, nil)
	getReq = setPathValues(getReq, map[string]string{"version": "v66.0", "type": "Case", "id": id})
	getRec := httptest.NewRecorder()

	handler.HandleGet(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get failed: %d", getRec.Code)
	}

	var getResp map[string]any
	json.NewDecoder(getRec.Body).Decode(&getResp)
	if getResp["Subject"] != "Integration Test Case" {
		t.Errorf("expected subject 'Integration Test Case', got '%v'", getResp["Subject"])
	}

	// 3. Update
	updateBody := map[string]any{
		"Status":   "In Progress",
		"Priority": "P1",
	}
	updateBytes, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest("PATCH", "/services/data/v66.0/sobjects/Case/"+id, bytes.NewReader(updateBytes))
	updateReq = setPathValues(updateReq, map[string]string{"version": "v66.0", "type": "Case", "id": id})
	updateRec := httptest.NewRecorder()

	handler.HandleUpdate(updateRec, updateReq)
	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("update failed: %d", updateRec.Code)
	}

	// Verify update
	getReq2 := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/Case/"+id, nil)
	getReq2 = setPathValues(getReq2, map[string]string{"version": "v66.0", "type": "Case", "id": id})
	getRec2 := httptest.NewRecorder()

	handler.HandleGet(getRec2, getReq2)
	var getResp2 map[string]any
	json.NewDecoder(getRec2.Body).Decode(&getResp2)
	if getResp2["Status"] != "In Progress" {
		t.Errorf("expected status 'In Progress', got '%v'", getResp2["Status"])
	}

	// 4. Delete
	deleteReq := httptest.NewRequest("DELETE", "/services/data/v66.0/sobjects/Case/"+id, nil)
	deleteReq = setPathValues(deleteReq, map[string]string{"version": "v66.0", "type": "Case", "id": id})
	deleteRec := httptest.NewRecorder()

	handler.HandleDelete(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete failed: %d", deleteRec.Code)
	}

	// Verify deletion
	getReq3 := httptest.NewRequest("GET", "/services/data/v66.0/sobjects/Case/"+id, nil)
	getReq3 = setPathValues(getReq3, map[string]string{"version": "v66.0", "type": "Case", "id": id})
	getRec3 := httptest.NewRecorder()

	handler.HandleGet(getRec3, getReq3)
	if getRec3.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getRec3.Code)
	}
}

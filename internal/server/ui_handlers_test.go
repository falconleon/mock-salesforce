package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/falconleon/mock-salesforce/internal/store"
)

// uiTestStore returns a memory store seeded with the small fixture used
// by the handler-level UI tests.
func uiTestStore(t *testing.T) *store.MemoryStore {
	t.Helper()
	s := store.NewMemoryStore()

	if _, err := s.Create("Account", store.Record{
		"Id": "acc-1", "Name": "Acme Corp",
		"Industry": "Technology", "Type": "Enterprise",
		"Phone": "555-0100", "Website": "https://acme.example.com",
		"BillingCity": "San Francisco", "BillingState": "CA",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := s.Create("Account", store.Record{
		"Id": "acc-2", "Name": "Beta Industries", "Industry": "Manufacturing",
	}); err != nil {
		t.Fatalf("seed account 2: %v", err)
	}

	if _, err := s.Create("Contact", store.Record{
		"Id": "ctc-1", "AccountId": "acc-1",
		"FirstName": "John", "LastName": "Smith", "Name": "John Smith",
		"Title": "IT Admin", "Email": "john@acme.example.com",
		"Phone": "555-0101", "Department": "IT",
	}); err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	if _, err := s.Create("User", store.Record{
		"Id": "usr-1", "Username": "owner@falcon.local",
		"FirstName": "Maria", "LastName": "Garcia", "Name": "Maria Garcia",
		"Email": "owner@falcon.local", "Title": "Senior Support Engineer",
		"Department": "Customer Support", "IsActive": true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if _, err := s.Create("Case", store.Record{
		"Id": "case-1", "CaseNumber": "00001000",
		"Subject": "Login broken", "Status": "Working",
		"Priority": "P1", "AccountId": "acc-1", "ContactId": "ctc-1",
		"OwnerId": "usr-1",
	}); err != nil {
		t.Fatalf("seed case: %v", err)
	}
	return s
}

func newTestUIHandler(t *testing.T) *UIHandler {
	t.Helper()
	return NewUIHandler(uiTestStore(t), "")
}

func TestHomeRendersCustomerList(t *testing.T) {
	h := newTestUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	rr := httptest.NewRecorder()

	h.Home(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Customers", "Acme Corp", "Beta Industries", "/lightning/r/Account/acc-1/view"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}

func TestContactDetailRenders(t *testing.T) {
	h := newTestUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/lightning/r/Contact/ctc-1/view", nil)
	req.SetPathValue("id", "ctc-1")
	rr := httptest.NewRecorder()

	h.ContactDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"John Smith", "IT Admin", "john@acme.example.com", "Acme Corp", "/lightning/r/Account/acc-1/view", "00001000"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}

func TestContactDetailNotFound(t *testing.T) {
	h := newTestUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/lightning/r/Contact/missing/view", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	h.ContactDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestUserDetailRendersWithOwnedCases(t *testing.T) {
	h := newTestUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/lightning/r/User/usr-1/view", nil)
	req.SetPathValue("id", "usr-1")
	rr := httptest.NewRecorder()

	h.UserDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Maria Garcia", "Senior Support Engineer", "owner@falcon.local", "Owned Cases", "00001000"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}

func TestUserDetailNotFound(t *testing.T) {
	h := newTestUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/lightning/r/User/missing/view", nil)
	req.SetPathValue("id", "missing")
	rr := httptest.NewRecorder()

	h.UserDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestAccountDetailIncludesContacts(t *testing.T) {
	h := newTestUIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/lightning/r/Account/acc-1/view", nil)
	req.SetPathValue("id", "acc-1")
	rr := httptest.NewRecorder()

	h.AccountDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{"Acme Corp", "Contacts (1)", "John Smith", "/lightning/r/Contact/ctc-1/view"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
}


// caseTabsTestStore seeds a memory store with a single case plus emails,
// comments, feed items + comments, tasks, events, files, and a content
// document link — the fixture for the T7 case-detail tab tests.
const caseTabsTestCaseID = "5003t00002CaseAAA"

func caseTabsTestStore(t *testing.T) *store.MemoryStore {
	t.Helper()
	s := store.NewMemoryStore()
	must := func(_ string, err error) {
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(s.Create("User", store.Record{"Id": "0053t00000UserAAA", "Name": "Avery Agent"}))
	must(s.Create("User", store.Record{"Id": "0053t00000UserBBB", "Name": "Blake Owner"}))
	must(s.Create("Account", store.Record{"Id": "0013t00002AcctAAA", "Name": "Test Account"}))
	must(s.Create("Contact", store.Record{
		"Id": "0033t00002CtctAAA", "FirstName": "Casey", "LastName": "Customer",
	}))
	must(s.Create("Case", store.Record{
		"Id": caseTabsTestCaseID, "CaseNumber": "00009999",
		"Subject": "Tabbed Test Case", "Status": "In Progress", "Priority": "P1",
		"AccountId": "0013t00002AcctAAA", "ContactId": "0033t00002CtctAAA",
		"Description": "Multi-tab case",
	}))
	must(s.Create("EmailMessage", store.Record{
		"Id": "02s3t00001EmailAAA", "ParentId": caseTabsTestCaseID,
		"Subject": "Re: Tabbed Test Case", "FromAddress": "support@example.com",
		"ToAddress": "casey@example.com", "TextBody": "We are investigating.",
		"MessageDate": "2024-01-20T08:00:00Z",
	}))
	must(s.Create("CaseComment", store.Record{
		"Id": "00a3t00001CommAAA", "ParentId": caseTabsTestCaseID,
		"CommentBody": "Initial triage complete.",
		"CreatedById": "0053t00000UserAAA",
		"CreatedDate": "2024-01-20T08:30:00Z", "IsPublished": false,
	}))
	must(s.Create("FeedItem", store.Record{
		"Id": "0D53t00000FeedAAA", "ParentId": caseTabsTestCaseID,
		"Body": "Posted a status update.", "Type": "TextPost",
		"CreatedById": "0053t00000UserAAA",
		"CreatedDate": "2024-01-20T09:00:00Z",
	}))
	must(s.Create("FeedComment", store.Record{
		"Id": "0D73t00000FCmtAAA", "FeedItemId": "0D53t00000FeedAAA",
		"ParentId": caseTabsTestCaseID, "CommentBody": "Thanks for the update.",
		"CreatedById": "0053t00000UserBBB",
		"CreatedDate": "2024-01-20T09:15:00Z",
	}))
	must(s.Create("Task", store.Record{
		"Id": "00T3t00000TaskAAA", "WhatId": caseTabsTestCaseID,
		"Subject": "Call customer back", "Status": "Open", "Priority": "High",
		"OwnerId": "0053t00000UserAAA", "ActivityDate": "2024-01-21",
	}))
	must(s.Create("Event", store.Record{
		"Id": "00U3t00000EvtAAA", "WhatId": caseTabsTestCaseID,
		"Subject": "Post-incident review", "ActivityDateTime": "2024-01-22T15:00:00Z",
		"DurationInMinutes": 60, "Location": "Zoom",
		"OwnerId": "0053t00000UserBBB",
	}))
	must(s.Create("ContentDocument", store.Record{
		"Id": "0693t00000DocAAA", "Title": "Incident report.pdf",
		"FileType": "PDF", "ContentSize": 184320,
		"CreatedDate": "2024-01-20T18:00:00Z",
	}))
	must(s.Create("ContentDocumentLink", store.Record{
		"Id": "06A3t00000LinkAAA", "LinkedEntityId": caseTabsTestCaseID,
		"ContentDocumentId": "0693t00000DocAAA",
	}))
	return s
}

func runCasePartial(h func(http.ResponseWriter, *http.Request), id string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", id)
	h(rr, req)
	return rr
}

func TestCaseDetail_RendersTabsAndScript(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseDetail, caseTabsTestCaseID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		`hx-get="/lightning/r/Case/` + caseTabsTestCaseID + `/related/emails"`,
		`hx-get="/lightning/r/Case/` + caseTabsTestCaseID + `/related/comments"`,
		`hx-get="/lightning/r/Case/` + caseTabsTestCaseID + `/related/feed"`,
		`hx-get="/lightning/r/Case/` + caseTabsTestCaseID + `/related/activities"`,
		`hx-get="/lightning/r/Case/` + caseTabsTestCaseID + `/related/files"`,
		`src="/static/htmx.min.js"`,
		"Emails (1)", "Comments (1)", "Feed (1)", "Activities (2)", "Files (1)",
		`hx-trigger="load"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	for _, banned := range []string{"unpkg.com", "cdn.jsdelivr", "cdnjs.cloudflare"} {
		if strings.Contains(body, banned) {
			t.Errorf("body must not reference CDN %q", banned)
		}
	}
}

func TestCaseDetail_NotFound_T7(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseDetail, "5003t00002Missing")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestCaseEmailsPartial(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseEmailsPartial, caseTabsTestCaseID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Re: Tabbed Test Case") {
		t.Errorf("missing email subject in body: %s", body)
	}
	if !strings.Contains(body, `data-tab-content="emails"`) {
		t.Errorf("missing tab marker")
	}
	if strings.Contains(body, "<html") || strings.Contains(body, "<body") {
		t.Errorf("partial must not include full document chrome")
	}
}

func TestCaseCommentsPartial_ResolvesUserName(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseCommentsPartial, caseTabsTestCaseID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Avery Agent") {
		t.Errorf("comment author name not resolved: %s", body)
	}
	if !strings.Contains(body, "Initial triage complete.") {
		t.Errorf("comment body missing")
	}
}

func TestCaseFeedPartial_IncludesNestedComments(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseFeedPartial, caseTabsTestCaseID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Posted a status update.",
		"Thanks for the update.",
		"Blake Owner",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestCaseActivitiesPartial(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseActivitiesPartial, caseTabsTestCaseID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"Call customer back", "Post-incident review",
		"Tasks", "Events", "Avery Agent", "Blake Owner",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestCaseFilesPartial(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseFilesPartial, caseTabsTestCaseID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Incident report.pdf") {
		t.Errorf("file title missing: %s", body)
	}
	if !strings.Contains(body, "180.0 KB") {
		t.Errorf("humanBytes formatting not applied: %s", body)
	}
}

func TestCasePartials_NotFound(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	for name, fn := range map[string]func(http.ResponseWriter, *http.Request){
		"emails":     h.CaseEmailsPartial,
		"comments":   h.CaseCommentsPartial,
		"feed":       h.CaseFeedPartial,
		"activities": h.CaseActivitiesPartial,
		"files":      h.CaseFilesPartial,
	} {
		rr := runCasePartial(fn, "5003t00002Missing")
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s partial: status = %d, want 404", name, rr.Code)
		}
	}
}

func TestCasePartials_EmptyState(t *testing.T) {
	s := store.NewMemoryStore()
	if _, err := s.Create("Case", store.Record{
		"Id": "5003t00002EmptyAA", "CaseNumber": "00000001", "Subject": "No related",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := NewUIHandler(s, "")
	for name, fn := range map[string]func(http.ResponseWriter, *http.Request){
		"emails":     h.CaseEmailsPartial,
		"comments":   h.CaseCommentsPartial,
		"feed":       h.CaseFeedPartial,
		"activities": h.CaseActivitiesPartial,
		"files":      h.CaseFilesPartial,
	} {
		rr := runCasePartial(fn, "5003t00002EmptyAA")
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", name, rr.Code)
			continue
		}
		if !strings.Contains(rr.Body.String(), "related-empty") {
			t.Errorf("%s: expected empty-state class, got: %s", name, rr.Body.String())
		}
	}
}

func TestCaseDetail_ResolvesByCaseNumber(t *testing.T) {
	h := NewUIHandler(caseTabsTestStore(t), "")
	rr := runCasePartial(h.CaseDetail, "00009999")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Tabbed Test Case") {
		t.Errorf("case detail by CaseNumber failed")
	}
}

func TestStaticHTMX_IsSelfHosted(t *testing.T) {
	data, err := staticFS.ReadFile("static/htmx.min.js")
	if err != nil {
		t.Fatalf("htmx.min.js not embedded: %v", err)
	}
	if len(data) < 1024 {
		t.Errorf("htmx.min.js seems too small (%d bytes)", len(data))
	}
	if !strings.Contains(string(data[:200]), "htmx") {
		t.Errorf("htmx.min.js header does not look right")
	}
}


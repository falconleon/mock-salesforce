package soql_test

import (
	"testing"

	"github.com/falconleon/mock-salesforce/internal/soql"
	"github.com/falconleon/mock-salesforce/internal/store"
)

func runQuery(t *testing.T, s store.Store, q string) *soql.QueryResult {
	t.Helper()
	stmt, err := soql.NewParser(q).Parse()
	if err != nil {
		t.Fatalf("parse error for %q: %v", q, err)
	}
	res, err := soql.NewExecutor(s).Execute(stmt)
	if err != nil {
		t.Fatalf("execute error for %q: %v", q, err)
	}
	return res
}

func seedAccountWithCases(t *testing.T) store.Store {
	t.Helper()
	s := store.NewMemoryStore()
	if _, err := s.Create("Account", store.Record{"Id": "001A", "Name": "Acme"}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := s.Create("Account", store.Record{"Id": "001B", "Name": "Globex"}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	cases := []store.Record{
		{"Id": "500A1", "AccountId": "001A", "Subject": "Login broken", "Status": "Open"},
		{"Id": "500A2", "AccountId": "001A", "Subject": "Slow page", "Status": "Closed"},
		{"Id": "500A3", "AccountId": "001A", "Subject": "Bad data", "Status": "Open"},
		{"Id": "500B1", "AccountId": "001B", "Subject": "Other", "Status": "Open"},
	}
	for _, c := range cases {
		if _, err := s.Create("Case", c); err != nil {
			t.Fatalf("seed case: %v", err)
		}
	}
	return s
}

func TestExecutor_AccountWithCasesSubQuery(t *testing.T) {
	s := seedAccountWithCases(t)
	res := runQuery(t, s, "SELECT Id, Name, (SELECT Id, Subject FROM Cases) FROM Account")

	if res.TotalSize != 2 {
		t.Fatalf("expected 2 accounts, got %d", res.TotalSize)
	}

	for _, rec := range res.Records {
		id := rec["Id"].(string)
		nested, ok := rec["Cases"].(map[string]any)
		if id == "001A" {
			if !ok {
				t.Fatalf("expected nested Cases map for 001A, got %T", rec["Cases"])
			}
			if got := nested["totalSize"].(int); got != 3 {
				t.Errorf("expected 3 cases for 001A, got %d", got)
			}
			recs := nested["records"].([]store.Record)
			if len(recs) != 3 {
				t.Errorf("expected 3 case records, got %d", len(recs))
			}
			// Each child record should carry attributes with type "Case"
			attrs := recs[0]["attributes"].(map[string]any)
			if attrs["type"] != "Case" {
				t.Errorf("expected child attributes.type 'Case', got %v", attrs["type"])
			}
		}
		if id == "001B" {
			if !ok {
				t.Fatalf("expected nested Cases map for 001B, got %T", rec["Cases"])
			}
			if got := nested["totalSize"].(int); got != 1 {
				t.Errorf("expected 1 case for 001B, got %d", got)
			}
		}
	}
}

func TestExecutor_CaseWithCaseCommentsSubQuery(t *testing.T) {
	s := store.NewMemoryStore()
	s.Create("Case", store.Record{"Id": "500X", "Subject": "Top"})
	s.Create("CaseComment", store.Record{"Id": "C1", "ParentId": "500X", "CommentBody": "first"})
	s.Create("CaseComment", store.Record{"Id": "C2", "ParentId": "500X", "CommentBody": "second"})
	s.Create("CaseComment", store.Record{"Id": "C3", "ParentId": "500Y", "CommentBody": "other"})

	res := runQuery(t, s, "SELECT Id, Subject, (SELECT Id, CommentBody FROM CaseComments) FROM Case WHERE Id = '500X'")

	if res.TotalSize != 1 {
		t.Fatalf("expected 1 case, got %d", res.TotalSize)
	}
	nested := res.Records[0]["CaseComments"].(map[string]any)
	if nested["totalSize"].(int) != 2 {
		t.Errorf("expected 2 comments, got %d", nested["totalSize"])
	}
}

func TestExecutor_SubQueryLimit(t *testing.T) {
	s := seedAccountWithCases(t)
	res := runQuery(t, s, "SELECT Id, (SELECT Id, Subject FROM Cases LIMIT 2) FROM Account WHERE Id = '001A'")
	if res.TotalSize != 1 {
		t.Fatalf("expected 1 account, got %d", res.TotalSize)
	}
	nested := res.Records[0]["Cases"].(map[string]any)
	if nested["totalSize"].(int) != 2 {
		t.Errorf("expected LIMIT 2 to cap nested totalSize, got %d", nested["totalSize"])
	}
	if recs := nested["records"].([]store.Record); len(recs) != 2 {
		t.Errorf("expected 2 nested records, got %d", len(recs))
	}
}

func TestExecutor_SubQueryWhereInside(t *testing.T) {
	s := seedAccountWithCases(t)
	res := runQuery(t, s, "SELECT Id, (SELECT Id, Subject FROM Cases WHERE Status = 'Open') FROM Account WHERE Id = '001A'")
	nested := res.Records[0]["Cases"].(map[string]any)
	if nested["totalSize"].(int) != 2 {
		t.Errorf("expected 2 open cases, got %d", nested["totalSize"])
	}
}

func TestExecutor_SubQueryEmpty(t *testing.T) {
	s := store.NewMemoryStore()
	s.Create("Account", store.Record{"Id": "001Z", "Name": "Lonely"})
	res := runQuery(t, s, "SELECT Id, (SELECT Id, Subject FROM Cases) FROM Account")
	if got, ok := res.Records[0]["Cases"]; ok && got != nil {
		t.Errorf("expected nil Cases for empty subquery, got %v", got)
	}
}

func TestExecutor_SubQueryUnknownRelationship(t *testing.T) {
	s := store.NewMemoryStore()
	s.Create("Account", store.Record{"Id": "001Q", "Name": "Q"})
	stmt, err := soql.NewParser("SELECT Id, (SELECT Id FROM Bogus) FROM Account").Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if _, err := soql.NewExecutor(s).Execute(stmt); err == nil {
		t.Error("expected error for unknown child relationship, got nil")
	}
}

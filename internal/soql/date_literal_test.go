package soql_test

import (
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/soql"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// fixedNow returns a deterministic time used to anchor date literal evaluation
// in tests. Wednesday, 2024-06-12 12:00 UTC sits in the middle of the week,
// month, and a quarter so range edges are easy to reason about.
func fixedNow() time.Time {
	return time.Date(2024, 6, 12, 12, 0, 0, 0, time.UTC)
}

func TestParser_DateLiteral_Today(t *testing.T) {
	stmt, err := soql.NewParser("SELECT Id FROM Case WHERE CreatedDate = TODAY").Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	cond, ok := stmt.Where.Condition.(*soql.ComparisonCondition)
	if !ok {
		t.Fatalf("expected ComparisonCondition, got %T", stmt.Where.Condition)
	}
	dl, ok := cond.Value.(soql.DateLiteral)
	if !ok {
		t.Fatalf("expected DateLiteral value, got %T (%v)", cond.Value, cond.Value)
	}
	if dl.Name != "TODAY" || dl.N != 0 {
		t.Errorf("expected TODAY, got %+v", dl)
	}
}

func TestParser_DateLiteral_Parameterized(t *testing.T) {
	stmt, err := soql.NewParser("SELECT Id FROM Case WHERE CreatedDate > LAST_N_DAYS:30").Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	cond := stmt.Where.Condition.(*soql.ComparisonCondition)
	dl, ok := cond.Value.(soql.DateLiteral)
	if !ok {
		t.Fatalf("expected DateLiteral, got %T", cond.Value)
	}
	if dl.Name != "LAST_N_DAYS" || dl.N != 30 {
		t.Errorf("expected LAST_N_DAYS:30, got %+v", dl)
	}
}

func TestParser_DateLiteral_RequiresColonForParameterized(t *testing.T) {
	_, err := soql.NewParser("SELECT Id FROM Case WHERE CreatedDate = LAST_N_DAYS").Parse()
	if err == nil {
		t.Fatal("expected parse error for missing ':N' on LAST_N_DAYS")
	}
}

func TestParser_DateLiteral_LowercaseAccepted(t *testing.T) {
	// SOQL identifiers are case-insensitive; ensure we canonicalize to upper.
	stmt, err := soql.NewParser("SELECT Id FROM Case WHERE CreatedDate = today").Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	dl := stmt.Where.Condition.(*soql.ComparisonCondition).Value.(soql.DateLiteral)
	if dl.Name != "TODAY" {
		t.Errorf("expected canonical TODAY, got %q", dl.Name)
	}
}

func TestDateLiteral_Range(t *testing.T) {
	now := fixedNow()
	tests := []struct {
		lit             soql.DateLiteral
		startISO, endISO string
	}{
		{soql.DateLiteral{Name: "TODAY"}, "2024-06-12T00:00:00Z", "2024-06-13T00:00:00Z"},
		{soql.DateLiteral{Name: "YESTERDAY"}, "2024-06-11T00:00:00Z", "2024-06-12T00:00:00Z"},
		{soql.DateLiteral{Name: "TOMORROW"}, "2024-06-13T00:00:00Z", "2024-06-14T00:00:00Z"},
		{soql.DateLiteral{Name: "THIS_WEEK"}, "2024-06-09T00:00:00Z", "2024-06-16T00:00:00Z"},
		{soql.DateLiteral{Name: "THIS_MONTH"}, "2024-06-01T00:00:00Z", "2024-07-01T00:00:00Z"},
		{soql.DateLiteral{Name: "LAST_MONTH"}, "2024-05-01T00:00:00Z", "2024-06-01T00:00:00Z"},
		{soql.DateLiteral{Name: "THIS_QUARTER"}, "2024-04-01T00:00:00Z", "2024-07-01T00:00:00Z"},
		{soql.DateLiteral{Name: "THIS_YEAR"}, "2024-01-01T00:00:00Z", "2025-01-01T00:00:00Z"},
		{soql.DateLiteral{Name: "LAST_N_DAYS", N: 7}, "2024-06-05T00:00:00Z", "2024-06-13T00:00:00Z"},
		{soql.DateLiteral{Name: "NEXT_N_DAYS", N: 3}, "2024-06-13T00:00:00Z", "2024-06-16T00:00:00Z"},
		{soql.DateLiteral{Name: "N_DAYS_AGO", N: 5}, "2024-06-07T00:00:00Z", "2024-06-08T00:00:00Z"},
		{soql.DateLiteral{Name: "LAST_90_DAYS"}, "2024-03-14T00:00:00Z", "2024-06-13T00:00:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.lit.String(), func(t *testing.T) {
			start, end := tt.lit.Range(now)
			wantStart, _ := time.Parse(time.RFC3339, tt.startISO)
			wantEnd, _ := time.Parse(time.RFC3339, tt.endISO)
			if !start.Equal(wantStart) {
				t.Errorf("start: got %s, want %s", start.Format(time.RFC3339), tt.startISO)
			}
			if !end.Equal(wantEnd) {
				t.Errorf("end: got %s, want %s", end.Format(time.RFC3339), tt.endISO)
			}
		})
	}
}

func TestExecutor_DateLiteral_FiltersByRange(t *testing.T) {
	s := store.NewMemoryStore()
	mustCreate := func(rec store.Record) {
		t.Helper()
		if _, err := s.Create("Case", rec); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	mustCreate(store.Record{"Id": "C-OLD", "CreatedDate": "2024-05-01T10:00:00Z"})
	mustCreate(store.Record{"Id": "C-MTHSTART", "CreatedDate": "2024-06-01T00:00:00Z"})
	mustCreate(store.Record{"Id": "C-MID", "CreatedDate": "2024-06-12T08:30:00Z"})
	mustCreate(store.Record{"Id": "C-FUTURE", "CreatedDate": "2024-07-15T00:00:00Z"})

	run := func(q string) []string {
		t.Helper()
		stmt, err := soql.NewParser(q).Parse()
		if err != nil {
			t.Fatalf("parse %q: %v", q, err)
		}
		ex := soql.NewExecutor(s)
		ex.SetNow(fixedNow)
		res, err := ex.Execute(stmt)
		if err != nil {
			t.Fatalf("execute %q: %v", q, err)
		}
		ids := make([]string, 0, len(res.Records))
		for _, r := range res.Records {
			ids = append(ids, r["Id"].(string))
		}
		return ids
	}

	got := run("SELECT Id FROM Case WHERE CreatedDate = THIS_MONTH")
	wantIDs(t, got, "C-MTHSTART", "C-MID")

	got = run("SELECT Id FROM Case WHERE CreatedDate < TODAY")
	wantIDs(t, got, "C-OLD", "C-MTHSTART")

	got = run("SELECT Id FROM Case WHERE CreatedDate >= TODAY")
	wantIDs(t, got, "C-MID", "C-FUTURE")

	got = run("SELECT Id FROM Case WHERE CreatedDate = LAST_N_DAYS:30")
	wantIDs(t, got, "C-MTHSTART", "C-MID")
}

func wantIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ids: got %v, want %v", got, want)
	}
	gotSet := map[string]bool{}
	for _, id := range got {
		gotSet[id] = true
	}
	for _, id := range want {
		if !gotSet[id] {
			t.Errorf("missing id %q in result %v", id, got)
		}
	}
}

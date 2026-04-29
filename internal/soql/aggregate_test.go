package soql_test

import (
	"testing"

	"github.com/falconleon/mock-salesforce/internal/store"
)

func seedAggregateCases(t *testing.T) store.Store {
	t.Helper()
	s := store.NewMemoryStore()
	cases := []store.Record{
		{"Id": "500A1", "AccountId": "001A", "Status": "Open", "Priority": "High", "Amount": 100},
		{"Id": "500A2", "AccountId": "001A", "Status": "Open", "Priority": "Low", "Amount": 50},
		{"Id": "500A3", "AccountId": "001A", "Status": "Closed", "Priority": "High", "Amount": 30},
		{"Id": "500B1", "AccountId": "001B", "Status": "Open", "Priority": "Low", "Amount": 200},
		{"Id": "500B2", "AccountId": "001B", "Status": "Closed", "Priority": "High", "Amount": 75},
	}
	for _, c := range cases {
		if _, err := s.Create("Case", c); err != nil {
			t.Fatalf("seed case: %v", err)
		}
	}
	return s
}

func TestExecutor_BareCount(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT() FROM Case")
	if res.TotalSize != 5 {
		t.Errorf("expected totalSize 5, got %d", res.TotalSize)
	}
	if len(res.Records) != 0 {
		t.Errorf("expected empty records for COUNT(), got %d", len(res.Records))
	}
	if !res.Done {
		t.Error("expected done=true")
	}
}

func TestExecutor_CountWithAlias(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT(Id) total FROM Case")
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Records))
	}
	if got := res.Records[0]["total"]; got != 5 {
		t.Errorf("expected total 5, got %v", got)
	}
}

func TestExecutor_GroupByOneField(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT(Id) total, Status FROM Case GROUP BY Status")
	if len(res.Records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Records))
	}
	counts := map[string]int{}
	for _, r := range res.Records {
		status, _ := r["Status"].(string)
		counts[status] = r["total"].(int)
	}
	if counts["Open"] != 3 || counts["Closed"] != 2 {
		t.Errorf("unexpected per-status counts: %v", counts)
	}
}

func TestExecutor_GroupByTwoFields(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT(Id) total, Status, Priority FROM Case GROUP BY Status, Priority")
	// Distinct (Status, Priority) tuples in the seed: Open/High, Open/Low, Closed/High.
	if len(res.Records) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(res.Records))
	}
}

func TestExecutor_HavingAggregate(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT(Id) total, Status FROM Case GROUP BY Status HAVING COUNT(Id) > 2")
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Records))
	}
	if got := res.Records[0]["Status"]; got != "Open" {
		t.Errorf("expected Status Open, got %v", got)
	}
}

func TestExecutor_OrderByAggregateAlias(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT(Id) total, Status FROM Case GROUP BY Status ORDER BY total DESC")
	if len(res.Records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Records))
	}
	if got := res.Records[0]["Status"]; got != "Open" {
		t.Errorf("expected Open first, got %v", got)
	}
}

func TestExecutor_AggregatesSumAvgMinMax(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT SUM(Amount) s, AVG(Amount) a, MIN(Amount) lo, MAX(Amount) hi FROM Case")
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Records))
	}
	r := res.Records[0]
	if r["s"] != 455 {
		t.Errorf("expected SUM 455, got %v", r["s"])
	}
	if r["a"].(float64) != 91.0 {
		t.Errorf("expected AVG 91, got %v", r["a"])
	}
	if r["lo"] != 30 {
		t.Errorf("expected MIN 30, got %v", r["lo"])
	}
	if r["hi"] != 200 {
		t.Errorf("expected MAX 200, got %v", r["hi"])
	}
}

func TestExecutor_CountDistinct(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT_DISTINCT(AccountId) accts FROM Case")
	if got := res.Records[0]["accts"]; got != 2 {
		t.Errorf("expected 2 distinct accounts, got %v", got)
	}
}

func TestExecutor_AggregateNoAliasUsesExprN(t *testing.T) {
	s := seedAggregateCases(t)
	res := runQuery(t, s, "SELECT COUNT(Id), MIN(Amount) FROM Case")
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Records))
	}
	r := res.Records[0]
	if r["expr0"] != 5 {
		t.Errorf("expected expr0=5 (COUNT(Id)), got %v", r["expr0"])
	}
	if r["expr1"] != 30 {
		t.Errorf("expected expr1=30 (MIN(Amount)), got %v", r["expr1"])
	}
}

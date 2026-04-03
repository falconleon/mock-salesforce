package soql_test

import (
	"testing"

	"github.com/falconleon/mock-salesforce/internal/soql"
)

func TestParser_SimpleSelect(t *testing.T) {
	input := "SELECT Id, Subject, Status FROM Case"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Object != "Case" {
		t.Errorf("expected object 'Case', got '%s'", stmt.Object)
	}

	if len(stmt.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(stmt.Fields))
	}

	expectedFields := []string{"Id", "Subject", "Status"}
	for i, f := range stmt.Fields {
		if f.Name != expectedFields[i] {
			t.Errorf("expected field[%d] = '%s', got '%s'", i, expectedFields[i], f.Name)
		}
	}
}

func TestParser_WhereEquals(t *testing.T) {
	input := "SELECT Id FROM Case WHERE Id = '5003t00002AbCdEAAV'"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}

	cond, ok := stmt.Where.Condition.(*soql.ComparisonCondition)
	if !ok {
		t.Fatalf("expected ComparisonCondition, got %T", stmt.Where.Condition)
	}

	if cond.Field.Name != "Id" {
		t.Errorf("expected field 'Id', got '%s'", cond.Field.Name)
	}

	if cond.Operator != "=" {
		t.Errorf("expected operator '=', got '%s'", cond.Operator)
	}

	if cond.Value != "5003t00002AbCdEAAV" {
		t.Errorf("expected value '5003t00002AbCdEAAV', got '%v'", cond.Value)
	}
}

func TestParser_WhereAnd(t *testing.T) {
	input := "SELECT Id FROM Case WHERE Status = 'Open' AND Priority = 'P1'"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}

	cond, ok := stmt.Where.Condition.(*soql.LogicalCondition)
	if !ok {
		t.Fatalf("expected LogicalCondition, got %T", stmt.Where.Condition)
	}

	if cond.Operator != "AND" {
		t.Errorf("expected operator 'AND', got '%s'", cond.Operator)
	}
}

func TestParser_OrderBy(t *testing.T) {
	input := "SELECT Id, MessageDate FROM EmailMessage ORDER BY MessageDate DESC"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stmt.OrderBy) != 1 {
		t.Fatalf("expected 1 ORDER BY field, got %d", len(stmt.OrderBy))
	}

	if stmt.OrderBy[0].Field.Name != "MessageDate" {
		t.Errorf("expected field 'MessageDate', got '%s'", stmt.OrderBy[0].Field.Name)
	}

	if !stmt.OrderBy[0].Descending {
		t.Error("expected Descending = true")
	}
}

func TestParser_Limit(t *testing.T) {
	input := "SELECT Id FROM Case LIMIT 10"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Limit == nil {
		t.Fatal("expected LIMIT")
	}

	if *stmt.Limit != 10 {
		t.Errorf("expected LIMIT 10, got %d", *stmt.Limit)
	}
}

func TestParser_RelationshipField(t *testing.T) {
	input := "SELECT Id, Owner.Name FROM Case"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stmt.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(stmt.Fields))
	}

	ownerField := stmt.Fields[1]
	if ownerField.Relation != "Owner" {
		t.Errorf("expected Relation 'Owner', got '%s'", ownerField.Relation)
	}
	if ownerField.Name != "Name" {
		t.Errorf("expected Name 'Name', got '%s'", ownerField.Name)
	}
}

func TestParser_InClause(t *testing.T) {
	input := "SELECT Id FROM Case WHERE Status IN ('Open', 'In Progress', 'Escalated')"
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Where == nil {
		t.Fatal("expected WHERE clause")
	}

	cond, ok := stmt.Where.Condition.(*soql.InCondition)
	if !ok {
		t.Fatalf("expected InCondition, got %T", stmt.Where.Condition)
	}

	if len(cond.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(cond.Values))
	}
}

func TestParser_CompleteQuery(t *testing.T) {
	input := `SELECT Id, Subject, Status, Priority
			  FROM Case
			  WHERE Status = 'Open' AND Priority = 'P1'
			  ORDER BY CreatedDate DESC
			  LIMIT 20`
	parser := soql.NewParser(input)
	stmt, err := parser.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stmt.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(stmt.Fields))
	}

	if stmt.Object != "Case" {
		t.Errorf("expected object 'Case', got '%s'", stmt.Object)
	}

	if stmt.Where == nil {
		t.Error("expected WHERE clause")
	}

	if len(stmt.OrderBy) != 1 {
		t.Errorf("expected 1 ORDER BY, got %d", len(stmt.OrderBy))
	}

	if stmt.Limit == nil || *stmt.Limit != 20 {
		t.Error("expected LIMIT 20")
	}
}

func TestParser_InvalidQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing FROM", "SELECT Id"},
		{"missing object", "SELECT Id FROM"},
		{"invalid WHERE", "SELECT Id FROM Case WHERE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := soql.NewParser(tt.input)
			_, err := parser.Parse()
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

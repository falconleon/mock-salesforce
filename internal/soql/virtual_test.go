package soql_test

import (
	"testing"

	"github.com/falconleon/mock-salesforce/internal/store"
)

// emptyStore is enough — virtual SObjects bypass the store entirely.
func emptyStore() store.Store { return store.NewMemoryStore() }

func TestExecutor_EntityDefinition_All(t *testing.T) {
	res := runQuery(t, emptyStore(), "SELECT QualifiedApiName, KeyPrefix FROM EntityDefinition")
	if res.TotalSize < 14 {
		t.Errorf("expected at least 14 entity definitions, got %d", res.TotalSize)
	}
	seen := map[string]string{}
	for _, r := range res.Records {
		name, _ := r["QualifiedApiName"].(string)
		kp, _ := r["KeyPrefix"].(string)
		seen[name] = kp
	}
	for _, want := range []string{"Account", "Contact", "Case", "EntityDefinition", "FieldDefinition"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("expected EntityDefinition for %q", want)
		}
	}
	if seen["Case"] != "500" {
		t.Errorf("expected Case keyPrefix '500', got %q", seen["Case"])
	}
}

func TestExecutor_EntityDefinition_WhereAndLimit(t *testing.T) {
	res := runQuery(t, emptyStore(),
		"SELECT QualifiedApiName FROM EntityDefinition WHERE QualifiedApiName = 'Case'")
	if res.TotalSize != 1 {
		t.Fatalf("expected 1 record, got %d", res.TotalSize)
	}
	if res.Records[0]["QualifiedApiName"] != "Case" {
		t.Errorf("got %v", res.Records[0]["QualifiedApiName"])
	}

	res = runQuery(t, emptyStore(),
		"SELECT QualifiedApiName FROM EntityDefinition ORDER BY QualifiedApiName ASC LIMIT 3")
	if len(res.Records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(res.Records))
	}
	prev := ""
	for _, r := range res.Records {
		cur, _ := r["QualifiedApiName"].(string)
		if prev != "" && cur < prev {
			t.Errorf("ORDER BY broken: %q < %q", cur, prev)
		}
		prev = cur
	}
}

func TestExecutor_FieldDefinition_FilterByEntity(t *testing.T) {
	res := runQuery(t, emptyStore(),
		"SELECT QualifiedApiName, DataType, Label FROM FieldDefinition WHERE EntityDefinition.QualifiedApiName = 'Case'")
	if res.TotalSize == 0 {
		t.Fatal("expected at least one Case field")
	}
	wantFields := map[string]bool{
		"Id": true, "Subject": true, "Status": true, "CaseNumber": true,
	}
	got := map[string]bool{}
	for _, r := range res.Records {
		name, _ := r["QualifiedApiName"].(string)
		got[name] = true
	}
	for f := range wantFields {
		if !got[f] {
			t.Errorf("expected Case field %q in FieldDefinition results", f)
		}
	}
}

func TestExecutor_FieldDefinition_FilterByEntityId(t *testing.T) {
	res := runQuery(t, emptyStore(),
		"SELECT QualifiedApiName FROM FieldDefinition WHERE EntityDefinitionId = 'AccountEntityDef'")
	if res.TotalSize == 0 {
		t.Fatal("expected Account fields")
	}
	for _, r := range res.Records {
		// projected results expose EntityDefinitionId only when selected; the
		// filter ran before projection so totalSize > 0 is enough here.
		if _, ok := r["QualifiedApiName"]; !ok {
			t.Errorf("missing QualifiedApiName in record %v", r)
		}
	}
}

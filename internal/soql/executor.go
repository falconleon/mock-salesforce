package soql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/falconleon/mock-salesforce/internal/store"
)

// Executor executes SOQL queries against a store.
type Executor struct {
	store store.Store
}

// NewExecutor creates a new query executor.
func NewExecutor(s store.Store) *Executor {
	return &Executor{store: s}
}

// QueryResult represents the result of a SOQL query.
type QueryResult struct {
	TotalSize int
	Done      bool
	Records   []store.Record
}

// Execute runs a SOQL query and returns the results.
func (e *Executor) Execute(stmt *SelectStatement) (*QueryResult, error) {
	// Build filter function from WHERE clause
	filter := e.buildFilter(stmt.Where)

	// Query the store
	records, err := e.store.Query(stmt.Object, filter)
	if err != nil {
		return nil, fmt.Errorf("querying store: %w", err)
	}

	// Apply ORDER BY
	if len(stmt.OrderBy) > 0 {
		e.sortRecords(records, stmt.OrderBy)
	}

	// Apply OFFSET
	if stmt.Offset != nil && *stmt.Offset > 0 {
		if *stmt.Offset >= len(records) {
			records = []store.Record{}
		} else {
			records = records[*stmt.Offset:]
		}
	}

	// Apply LIMIT
	if stmt.Limit != nil && *stmt.Limit < len(records) {
		records = records[:*stmt.Limit]
	}

	// Project fields
	projected := e.projectFields(records, stmt.Fields, stmt.Object)

	return &QueryResult{
		TotalSize: len(projected),
		Done:      true,
		Records:   projected,
	}, nil
}

// buildFilter creates a filter function from a WHERE clause.
func (e *Executor) buildFilter(where *WhereClause) func(store.Record) bool {
	if where == nil {
		return nil
	}
	return e.buildConditionFilter(where.Condition)
}

// buildConditionFilter creates a filter function from a condition.
func (e *Executor) buildConditionFilter(cond Condition) func(store.Record) bool {
	switch c := cond.(type) {
	case *ComparisonCondition:
		return e.buildComparisonFilter(c)
	case *InCondition:
		return e.buildInFilter(c)
	case *LogicalCondition:
		return e.buildLogicalFilter(c)
	case *NotCondition:
		inner := e.buildConditionFilter(c.Condition)
		return func(r store.Record) bool {
			return !inner(r)
		}
	default:
		return func(r store.Record) bool { return true }
	}
}

// buildComparisonFilter creates a filter for a comparison condition.
func (e *Executor) buildComparisonFilter(c *ComparisonCondition) func(store.Record) bool {
	return func(r store.Record) bool {
		fieldValue := e.getFieldValue(r, c.Field)
		return e.compare(fieldValue, c.Operator, c.Value)
	}
}

// buildInFilter creates a filter for an IN condition.
func (e *Executor) buildInFilter(c *InCondition) func(store.Record) bool {
	return func(r store.Record) bool {
		fieldValue := e.getFieldValue(r, c.Field)
		for _, v := range c.Values {
			if e.valuesEqual(fieldValue, v) {
				return !c.Not
			}
		}
		return c.Not
	}
}

// buildLogicalFilter creates a filter for AND/OR conditions.
func (e *Executor) buildLogicalFilter(c *LogicalCondition) func(store.Record) bool {
	left := e.buildConditionFilter(c.Left)
	right := e.buildConditionFilter(c.Right)

	if c.Operator == "AND" {
		return func(r store.Record) bool {
			return left(r) && right(r)
		}
	}
	// OR
	return func(r store.Record) bool {
		return left(r) || right(r)
	}
}

// getFieldValue retrieves a field value from a record, supporting relationship fields.
func (e *Executor) getFieldValue(r store.Record, f Field) any {
	if f.Relation != "" {
		// Handle relationship field (e.g., Owner.Name)
		if related, ok := r[f.Relation].(map[string]any); ok {
			return related[f.Name]
		}
		return nil
	}
	return r[f.Name]
}

// compare performs a comparison operation.
func (e *Executor) compare(fieldValue any, op string, value any) bool {
	switch op {
	case "=":
		return e.valuesEqual(fieldValue, value)
	case "!=":
		return !e.valuesEqual(fieldValue, value)
	case "<":
		return e.compareLess(fieldValue, value)
	case "<=":
		return e.compareLess(fieldValue, value) || e.valuesEqual(fieldValue, value)
	case ">":
		return e.compareGreater(fieldValue, value)
	case ">=":
		return e.compareGreater(fieldValue, value) || e.valuesEqual(fieldValue, value)
	case "LIKE":
		return e.matchLike(fieldValue, value)
	default:
		return false
	}
}

// valuesEqual checks if two values are equal.
func (e *Executor) valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to string for comparison if types differ
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return aStr == bStr
}

// compareLess checks if a < b.
func (e *Executor) compareLess(a, b any) bool {
	switch av := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			return av < bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av < bv
		}
		if bv, ok := b.(int); ok {
			return av < float64(bv)
		}
	case string:
		if bv, ok := b.(string); ok {
			return av < bv
		}
	}
	return false
}

// compareGreater checks if a > b.
func (e *Executor) compareGreater(a, b any) bool {
	switch av := a.(type) {
	case int:
		if bv, ok := b.(int); ok {
			return av > bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av > bv
		}
		if bv, ok := b.(int); ok {
			return av > float64(bv)
		}
	case string:
		if bv, ok := b.(string); ok {
			return av > bv
		}
	}
	return false
}

// matchLike performs LIKE pattern matching.
func (e *Executor) matchLike(fieldValue, pattern any) bool {
	fv, ok := fieldValue.(string)
	if !ok {
		return false
	}
	pv, ok := pattern.(string)
	if !ok {
		return false
	}

	// Convert SOQL LIKE pattern to Go pattern
	// % matches any sequence, _ matches single character
	// For simplicity, use contains for %pattern% matching
	pv = strings.ToLower(pv)
	fv = strings.ToLower(fv)

	if strings.HasPrefix(pv, "%") && strings.HasSuffix(pv, "%") {
		// Contains match
		return strings.Contains(fv, pv[1:len(pv)-1])
	} else if strings.HasPrefix(pv, "%") {
		// Ends with
		return strings.HasSuffix(fv, pv[1:])
	} else if strings.HasSuffix(pv, "%") {
		// Starts with
		return strings.HasPrefix(fv, pv[:len(pv)-1])
	}
	// Exact match
	return fv == pv
}

// sortRecords sorts records by ORDER BY fields.
func (e *Executor) sortRecords(records []store.Record, orderBy []OrderByField) {
	sort.Slice(records, func(i, j int) bool {
		for _, ob := range orderBy {
			vi := e.getFieldValue(records[i], ob.Field)
			vj := e.getFieldValue(records[j], ob.Field)

			// Handle nulls
			if vi == nil && vj == nil {
				continue
			}
			if vi == nil {
				// NULLS LAST by default in descending, NULLS FIRST in ascending
				if ob.NullsFirst != nil {
					return *ob.NullsFirst
				}
				return !ob.Descending
			}
			if vj == nil {
				if ob.NullsFirst != nil {
					return !*ob.NullsFirst
				}
				return ob.Descending
			}

			// Compare values
			if e.valuesEqual(vi, vj) {
				continue
			}

			less := e.compareLess(vi, vj)
			if ob.Descending {
				return !less
			}
			return less
		}
		return false
	})
}

// relationshipMeta defines how to resolve relationship fields via FK lookup.
var relationshipMeta = map[string]map[string]struct {
	FKField    string
	TargetType string
}{
	"Case": {
		"Account":   {FKField: "AccountId", TargetType: "Account"},
		"CreatedBy":  {FKField: "CreatedById", TargetType: "User"},
		"Owner":      {FKField: "OwnerId", TargetType: "User"},
	},
	"CaseComment": {
		"CreatedBy": {FKField: "CreatedById", TargetType: "User"},
	},
	"FeedItem": {
		"CreatedBy": {FKField: "CreatedById", TargetType: "User"},
	},
	"EmailMessage": {},
}

// resolveRelationshipField looks up a related record via FK and returns the target field value.
func (e *Executor) resolveRelationshipField(record store.Record, objectType, relation, targetField string) any {
	rels, ok := relationshipMeta[objectType]
	if !ok {
		return nil
	}
	meta, ok := rels[relation]
	if !ok {
		return nil
	}
	fkValue, ok := record[meta.FKField]
	if !ok || fkValue == nil {
		return nil
	}
	related, err := e.store.Get(meta.TargetType, fmt.Sprint(fkValue))
	if err != nil {
		return nil
	}
	return related[targetField]
}

// projectFields selects only the requested fields from records.
func (e *Executor) projectFields(records []store.Record, fields []Field, objectType string) []store.Record {
	result := make([]store.Record, 0, len(records))

	for _, r := range records {
		projected := store.Record{
			"attributes": map[string]any{
				"type": objectType,
				"url":  fmt.Sprintf("/services/data/v66.0/sobjects/%s/%s", objectType, r["Id"]),
			},
		}

		for _, f := range fields {
			if f.Relation != "" {
				// Relationship field — try nested object first, then FK lookup
				var val any
				if related, ok := r[f.Relation].(map[string]any); ok {
					val = related[f.Name]
				} else {
					val = e.resolveRelationshipField(r, objectType, f.Relation, f.Name)
				}

				// Merge into relation object in output
				if projected[f.Relation] == nil {
					projected[f.Relation] = map[string]any{}
				}
				projected[f.Relation].(map[string]any)[f.Name] = val
			} else {
				// Simple field — return nil for missing (custom fields)
				projected[f.Name] = r[f.Name]
			}
		}

		result = append(result, projected)
	}

	return result
}

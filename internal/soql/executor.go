package soql

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/falconleon/mock-salesforce/internal/schema"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// Executor executes SOQL queries against a store.
type Executor struct {
	store store.Store
	now   func() time.Time
}

// NewExecutor creates a new query executor.
func NewExecutor(s store.Store) *Executor {
	return &Executor{store: s, now: time.Now}
}

// SetNow overrides the time source used to evaluate date literals. Intended
// for tests; production code should use the default time.Now.
func (e *Executor) SetNow(now func() time.Time) {
	if now == nil {
		e.now = time.Now
		return
	}
	e.now = now
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

	// Source records: virtual schema-discovery objects bypass the store and
	// are generated on the fly from the describe registry.
	var (
		records []store.Record
		err     error
	)
	if schema.IsVirtual(stmt.Object) {
		records = virtualRecords(stmt.Object, filter)
	} else {
		records, err = e.store.Query(stmt.Object, filter)
		if err != nil {
			return nil, fmt.Errorf("querying store: %w", err)
		}
	}

	// Aggregate / GROUP BY path
	if hasAggregateFields(stmt.Fields) || len(stmt.GroupBy) > 0 {
		return e.executeAggregate(stmt, records)
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

	// Resolve parent-child subqueries and attach as nested QueryResult-shaped maps
	if len(stmt.SubQueries) > 0 {
		for i, parent := range records {
			parentID, _ := parent["Id"].(string)
			for _, sub := range stmt.SubQueries {
				nested, err := e.executeSubQuery(stmt.Object, parentID, sub)
				if err != nil {
					return nil, err
				}
				projected[i][sub.Relationship] = nested
			}
		}
	}

	return &QueryResult{
		TotalSize: len(projected),
		Done:      true,
		Records:   projected,
	}, nil
}

// executeSubQuery resolves a parent-child subquery for a given parent record.
// Returns a nil value when the subquery yields no rows (matching SF's behaviour
// of omitting empty child collections); otherwise a {totalSize, done, records} map.
func (e *Executor) executeSubQuery(parentType, parentID string, sub SubQuery) (any, error) {
	rels, ok := childRelationshipMeta[parentType]
	if !ok {
		return nil, fmt.Errorf("no child relationships registered for %s", parentType)
	}
	meta, ok := rels[sub.Relationship]
	if !ok {
		return nil, fmt.Errorf("unknown child relationship %s on %s", sub.Relationship, parentType)
	}

	// Load child records that point back to this parent via the FK.
	filter := e.buildFilter(sub.Where)
	fkValue := parentID
	children, err := e.store.Query(meta.ChildType, func(r store.Record) bool {
		v, ok := r[meta.FKField]
		if !ok || v == nil {
			return false
		}
		if fmt.Sprint(v) != fkValue {
			return false
		}
		if filter != nil {
			return filter(r)
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("querying child %s: %w", meta.ChildType, err)
	}

	if len(sub.OrderBy) > 0 {
		e.sortRecords(children, sub.OrderBy)
	}
	if sub.Offset != nil && *sub.Offset > 0 {
		if *sub.Offset >= len(children) {
			children = []store.Record{}
		} else {
			children = children[*sub.Offset:]
		}
	}
	if sub.Limit != nil && *sub.Limit < len(children) {
		children = children[:*sub.Limit]
	}

	if len(children) == 0 {
		return nil, nil
	}

	projected := e.projectFields(children, sub.Fields, meta.ChildType)
	return map[string]any{
		"totalSize": len(projected),
		"done":      true,
		"records":   projected,
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
			if e.compare(fieldValue, "=", v) {
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
	if f.Aggregate != "" {
		if v, ok := r[f.AggregateKey()]; ok {
			return v
		}
		if f.Alias != "" {
			return r[f.Alias]
		}
		return nil
	}
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
	if dl, ok := value.(DateLiteral); ok {
		return e.compareDateLiteral(fieldValue, op, dl)
	}
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

// compareDateLiteral applies SOQL date-literal comparison semantics. The
// literal evaluates to a half-open range [start, end); the comparison is
// performed against those range edges:
//
//	= : start <= field < end
//	!= : field < start || field >= end
//	<  : field < start
//	<= : field < end
//	>  : field >= end
//	>= : field >= start
func (e *Executor) compareDateLiteral(fieldValue any, op string, d DateLiteral) bool {
	t, ok := parseFieldTime(fieldValue)
	if !ok {
		return false
	}
	start, end := d.Range(e.now())
	if start.IsZero() && end.IsZero() {
		return false
	}
	switch op {
	case "=":
		return !t.Before(start) && t.Before(end)
	case "!=":
		return t.Before(start) || !t.Before(end)
	case "<":
		return t.Before(start)
	case "<=":
		return t.Before(end)
	case ">":
		return !t.Before(end)
	case ">=":
		return !t.Before(start)
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

// childRelationshipMeta defines parent-child relationships keyed on
// (parent SObject type, plural relationship name) -> child SObject and FK field on that child.
// Names match the real Salesforce relationship names exposed via describe.
var childRelationshipMeta = map[string]map[string]struct {
	ChildType string
	FKField   string
}{
	"Account": {
		"Cases":    {ChildType: "Case", FKField: "AccountId"},
		"Contacts": {ChildType: "Contact", FKField: "AccountId"},
	},
	"Case": {
		"CaseComments":             {ChildType: "CaseComment", FKField: "ParentId"},
		"EmailMessages":            {ChildType: "EmailMessage", FKField: "ParentId"},
		"Feeds":                    {ChildType: "FeedItem", FKField: "ParentId"},
		"Tasks":                    {ChildType: "Task", FKField: "WhatId"},
		"Events":                   {ChildType: "Event", FKField: "WhatId"},
		"AttachedContentDocuments": {ChildType: "ContentDocumentLink", FKField: "LinkedEntityId"},
	},
	"Contact": {
		"Cases": {ChildType: "Case", FKField: "ContactId"},
	},
	"User": {
		"Cases": {ChildType: "Case", FKField: "OwnerId"},
	},
	"FeedItem": {
		"FeedComments": {ChildType: "FeedComment", FKField: "FeedItemId"},
	},
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

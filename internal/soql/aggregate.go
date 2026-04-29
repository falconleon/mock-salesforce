package soql

import (
	"fmt"
	"strconv"

	"github.com/falconleon/mock-salesforce/internal/store"
)

// hasAggregateFields reports whether any field in the SELECT list is an aggregate.
func hasAggregateFields(fields []Field) bool {
	for _, f := range fields {
		if f.Aggregate != "" {
			return true
		}
	}
	return false
}

// isBareCount reports whether the SELECT list is a single COUNT() with no arg —
// the special form that returns just totalSize and an empty records array.
func isBareCount(stmt *SelectStatement) bool {
	if len(stmt.Fields) != 1 || len(stmt.GroupBy) > 0 {
		return false
	}
	f := stmt.Fields[0]
	return f.Aggregate == "COUNT" && f.Name == "" && f.Relation == ""
}

// executeAggregate handles SELECT statements with aggregate functions and/or
// GROUP BY. The bare COUNT() form returns just totalSize.
func (e *Executor) executeAggregate(stmt *SelectStatement, records []store.Record) (*QueryResult, error) {
	if isBareCount(stmt) {
		return &QueryResult{TotalSize: len(records), Done: true, Records: []store.Record{}}, nil
	}

	groups := groupRecords(records, stmt.GroupBy)
	out := make([]store.Record, 0, len(groups))
	for _, g := range groups {
		row := buildAggregateRow(stmt, g.records)
		out = append(out, row)
	}

	if stmt.Having != nil {
		filter := e.buildFilter(stmt.Having)
		filtered := out[:0]
		for _, r := range out {
			if filter(r) {
				filtered = append(filtered, r)
			}
		}
		out = filtered
	}

	if len(stmt.OrderBy) > 0 {
		e.sortRecords(out, stmt.OrderBy)
	}

	if stmt.Offset != nil && *stmt.Offset > 0 {
		if *stmt.Offset >= len(out) {
			out = []store.Record{}
		} else {
			out = out[*stmt.Offset:]
		}
	}
	if stmt.Limit != nil && *stmt.Limit < len(out) {
		out = out[:*stmt.Limit]
	}

	return &QueryResult{TotalSize: len(out), Done: true, Records: out}, nil
}

// aggregateGroup is one group of records sharing the same GROUP BY key.
type aggregateGroup struct {
	key     []any
	records []store.Record
}

// groupRecords partitions records by the values of the GROUP BY fields,
// preserving first-encountered order. With no GROUP BY all records form one
// group with an empty key.
func groupRecords(records []store.Record, groupBy []Field) []aggregateGroup {
	if len(groupBy) == 0 {
		return []aggregateGroup{{key: nil, records: records}}
	}
	indexByKey := map[string]int{}
	var groups []aggregateGroup
	for _, r := range records {
		key := make([]any, len(groupBy))
		for i, f := range groupBy {
			if f.Relation != "" {
				if rel, ok := r[f.Relation].(map[string]any); ok {
					key[i] = rel[f.Name]
				}
			} else {
				key[i] = r[f.Name]
			}
		}
		hash := fmt.Sprintf("%v", key)
		if idx, ok := indexByKey[hash]; ok {
			groups[idx].records = append(groups[idx].records, r)
			continue
		}
		indexByKey[hash] = len(groups)
		groups = append(groups, aggregateGroup{key: key, records: []store.Record{r}})
	}
	return groups
}

// buildAggregateRow projects one output record from a group, computing each
// SELECT-list aggregate and copying GROUP BY values. Aggregate values are
// stored under both the canonical AGG(field) key (for HAVING/ORDER BY without
// alias) and the alias / expr-N output key.
func buildAggregateRow(stmt *SelectStatement, groupRecs []store.Record) store.Record {
	row := store.Record{
		"attributes": map[string]any{"type": "AggregateResult"},
	}
	exprIdx := 0
	for _, f := range stmt.Fields {
		if f.Aggregate != "" {
			val := computeAggregate(f, groupRecs)
			row[f.AggregateKey()] = val
			outKey := f.Alias
			if outKey == "" {
				outKey = "expr" + strconv.Itoa(exprIdx)
				exprIdx++
			}
			row[outKey] = val
			continue
		}
		// Non-aggregate field — copy from first record in group; for grouped
		// queries this is the GROUP BY column.
		if len(groupRecs) > 0 {
			if f.Relation != "" {
				if rel, ok := groupRecs[0][f.Relation].(map[string]any); ok {
					if row[f.Relation] == nil {
						row[f.Relation] = map[string]any{}
					}
					row[f.Relation].(map[string]any)[f.Name] = rel[f.Name]
				}
			} else {
				row[f.Name] = groupRecs[0][f.Name]
			}
		}
	}
	return row
}

// computeAggregate computes the aggregate value over the records in a group.
func computeAggregate(f Field, recs []store.Record) any {
	switch f.Aggregate {
	case "COUNT":
		if f.Name == "" {
			return len(recs)
		}
		n := 0
		for _, r := range recs {
			if v, ok := r[f.Name]; ok && v != nil {
				n++
			}
		}
		return n
	case "COUNT_DISTINCT":
		seen := map[string]struct{}{}
		for _, r := range recs {
			v, ok := r[f.Name]
			if !ok || v == nil {
				continue
			}
			seen[fmt.Sprintf("%v", v)] = struct{}{}
		}
		return len(seen)
	case "SUM":
		var sum float64
		var hadInt = true
		for _, r := range recs {
			x, ok := numericValue(r[f.Name])
			if !ok {
				continue
			}
			if x != float64(int64(x)) {
				hadInt = false
			}
			sum += x
		}
		if hadInt {
			return int(sum)
		}
		return sum
	case "AVG":
		var sum float64
		var n int
		for _, r := range recs {
			x, ok := numericValue(r[f.Name])
			if !ok {
				continue
			}
			sum += x
			n++
		}
		if n == 0 {
			return nil
		}
		return sum / float64(n)
	case "MIN":
		return aggregateExtreme(recs, f.Name, true)
	case "MAX":
		return aggregateExtreme(recs, f.Name, false)
	}
	return nil
}

// numericValue coerces a stored field value to float64 for arithmetic aggregates.
func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

// aggregateExtreme returns MIN (least=true) or MAX (least=false) of the named
// field across recs. Numeric and string types are supported; nils are skipped.
func aggregateExtreme(recs []store.Record, name string, least bool) any {
	var best any
	for _, r := range recs {
		v, ok := r[name]
		if !ok || v == nil {
			continue
		}
		if best == nil {
			best = v
			continue
		}
		if cmpLess(v, best) == least {
			best = v
		}
	}
	return best
}

// cmpLess reports whether a < b using numeric comparison if possible, else string.
func cmpLess(a, b any) bool {
	if af, ok := numericValue(a); ok {
		if bf, ok2 := numericValue(b); ok2 {
			return af < bf
		}
	}
	return fmt.Sprintf("%v", a) < fmt.Sprintf("%v", b)
}

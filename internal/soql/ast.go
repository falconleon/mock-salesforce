package soql

// SelectStatement represents a SOQL SELECT query.
type SelectStatement struct {
	Fields  []Field
	Object  string
	Where   *WhereClause
	OrderBy []OrderByField
	Limit   *int
	Offset  *int
}

// Field represents a field in the SELECT clause.
type Field struct {
	Name     string // e.g., "Subject" or "Name" for "Owner.Name"
	Relation string // e.g., "Owner" for "Owner.Name" (empty for direct fields)
}

// FullName returns the full field name including relation.
func (f Field) FullName() string {
	if f.Relation != "" {
		return f.Relation + "." + f.Name
	}
	return f.Name
}

// WhereClause represents a WHERE clause with conditions.
type WhereClause struct {
	Condition Condition
}

// Condition is an interface for WHERE conditions.
type Condition interface {
	isCondition()
}

// ComparisonCondition represents a field comparison (e.g., Id = '...')
type ComparisonCondition struct {
	Field    Field
	Operator string // =, !=, <, <=, >, >=, LIKE
	Value    any    // string, number, bool, or nil for NULL
}

func (c *ComparisonCondition) isCondition() {}

// InCondition represents an IN clause (e.g., Status IN ('Open', 'Closed'))
type InCondition struct {
	Field  Field
	Values []any
	Not    bool // true for NOT IN
}

func (c *InCondition) isCondition() {}

// LogicalCondition represents AND/OR combinations.
type LogicalCondition struct {
	Operator string // AND, OR
	Left     Condition
	Right    Condition
}

func (c *LogicalCondition) isCondition() {}

// NotCondition represents a NOT condition.
type NotCondition struct {
	Condition Condition
}

func (c *NotCondition) isCondition() {}

// OrderByField represents an ORDER BY field.
type OrderByField struct {
	Field      Field
	Descending bool
	NullsFirst *bool // nil means default, true = NULLS FIRST, false = NULLS LAST
}

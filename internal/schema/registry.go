// Package schema is the single source of truth for the mock SObject describe
// metadata. Both the REST describe handler and the SOQL executor (for the
// virtual EntityDefinition / FieldDefinition objects) consume this registry.
package schema

import "sort"

// FieldMeta describes a single SObject field.
type FieldMeta struct {
	Name        string
	Label       string
	Type        string // id, string, picklist, datetime, boolean, reference, ...
	Length      int
	Nillable    bool
	Updateable  bool
	Createable  bool
	Filterable  bool
	Sortable    bool
	Unique      bool
	ExternalID  bool
	ReferenceTo []string
}

// ObjectMeta describes a single SObject type.
type ObjectMeta struct {
	Name         string
	Label        string
	LabelPlural  string
	KeyPrefix    string
	Custom       bool
	Queryable    bool
	Createable   bool
	Updateable   bool
	Deletable    bool
	Retrieveable bool
	Searchable   bool
	customFields []FieldMeta
}

// Fields returns the full ordered field list (common fields prepended).
func (o ObjectMeta) Fields() []FieldMeta {
	out := make([]FieldMeta, 0, len(commonFields)+len(o.customFields))
	out = append(out, commonFields...)
	out = append(out, o.customFields...)
	return out
}

// commonFields are the always-present audit fields on every SObject.
var commonFields = []FieldMeta{
	{Name: "Id", Label: "Record ID", Type: "id", Length: 18, Nillable: false, Filterable: true, Sortable: true, Unique: true},
	{Name: "CreatedDate", Label: "Created Date", Type: "datetime", Filterable: true, Sortable: true},
	{Name: "LastModifiedDate", Label: "Last Modified Date", Type: "datetime", Nillable: true, Filterable: true, Sortable: true},
}

// registry is the package-level set of statically described SObjects.
// Order is preserved by names slice for deterministic enumeration.
var (
	registry = map[string]ObjectMeta{}
	names    []string
)

// Register adds an ObjectMeta to the registry. Last write wins for the type.
func Register(m ObjectMeta) {
	if _, exists := registry[m.Name]; !exists {
		names = append(names, m.Name)
	}
	registry[m.Name] = m
}

// Get returns the ObjectMeta for a type, ok false if unknown.
func Get(name string) (ObjectMeta, bool) {
	m, ok := registry[name]
	return m, ok
}

// All returns the registered objects in registration order.
func All() []ObjectMeta {
	out := make([]ObjectMeta, 0, len(names))
	for _, n := range names {
		out = append(out, registry[n])
	}
	return out
}

// Names returns the registered object names in registration order.
func Names() []string {
	out := make([]string, len(names))
	copy(out, names)
	return out
}

// SortedNames returns the registered object names in alphabetical order.
func SortedNames() []string {
	out := Names()
	sort.Strings(out)
	return out
}

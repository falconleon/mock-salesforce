package soql

import (
	"github.com/falconleon/mock-salesforce/internal/schema"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// virtualRecords returns the synthetic record set for a schema-discovery
// virtual SObject (EntityDefinition / FieldDefinition), pre-filtered by the
// given WHERE-clause filter when non-nil.
func virtualRecords(object string, filter func(store.Record) bool) []store.Record {
	var raw []map[string]any
	switch object {
	case "EntityDefinition":
		raw = schema.EntityDefinitionRecords()
	case "FieldDefinition":
		raw = schema.FieldDefinitionRecords()
	default:
		return nil
	}
	out := make([]store.Record, 0, len(raw))
	for _, m := range raw {
		r := store.Record(m)
		if filter == nil || filter(r) {
			out = append(out, r)
		}
	}
	return out
}

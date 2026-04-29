package schema

// EntityDefinitionRecords builds one record per registered ObjectMeta in the
// shape expected by a SOQL select against EntityDefinition.
func EntityDefinitionRecords() []map[string]any {
	objs := All()
	out := make([]map[string]any, 0, len(objs))
	for _, m := range objs {
		out = append(out, map[string]any{
			"Id":               m.Name + "EntityDef",
			"QualifiedApiName": m.Name,
			"DeveloperName":    m.Name,
			"MasterLabel":      m.Label,
			"Label":            m.Label,
			"KeyPrefix":        m.KeyPrefix,
			"PluralLabel":      m.LabelPlural,
			"IsCustomizable":   !m.Custom,
		})
	}
	return out
}

// FieldDefinitionRecords builds one record per (entity, field) pair in the
// registry. The EntityDefinition relationship is materialized as a nested
// map so SOQL relationship filters such as
//
//	WHERE EntityDefinition.QualifiedApiName = 'Case'
//
// resolve through the executor's existing nested-field lookup path.
func FieldDefinitionRecords() []map[string]any {
	objs := All()
	out := make([]map[string]any, 0, 256)
	for _, m := range objs {
		entityID := m.Name + "EntityDef"
		for _, f := range m.Fields() {
			length := any(nil)
			if f.Length > 0 {
				length = f.Length
			}
			out = append(out, map[string]any{
				"Id":                 m.Name + "." + f.Name,
				"QualifiedApiName":   f.Name,
				"DeveloperName":      f.Name,
				"Label":              f.Label,
				"DataType":           f.Type,
				"Length":             length,
				"IsNillable":         f.Nillable,
				"EntityDefinitionId": entityID,
				"EntityDefinition": map[string]any{
					"QualifiedApiName": m.Name,
					"DeveloperName":    m.Name,
					"Label":            m.Label,
					"KeyPrefix":        m.KeyPrefix,
				},
			})
		}
	}
	return out
}

// IsVirtual reports whether the given object name is one of the schema-
// discovery virtual SObjects served from the registry rather than the store.
func IsVirtual(name string) bool {
	return name == "EntityDefinition" || name == "FieldDefinition"
}

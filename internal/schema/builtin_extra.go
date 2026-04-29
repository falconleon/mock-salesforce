package schema

func feedCommentMeta() ObjectMeta {
	return ObjectMeta{
		Name: "FeedComment", Label: "Feed Comment", LabelPlural: "Feed Comments", KeyPrefix: "0D7",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "FeedItemId", Label: "Feed Item ID", Type: "reference", ReferenceTo: []string{"FeedItem"}, Createable: true, Filterable: true, Sortable: true},
			{Name: "ParentId", Label: "Parent ID", Type: "reference", ReferenceTo: []string{"Case", "Account", "Contact", "User"}, Nillable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "CommentBody", Label: "Comment Body", Type: "textarea", Length: 5000, Nillable: true, Updateable: true, Createable: true},
			{Name: "CreatedById", Label: "Created By ID", Type: "reference", ReferenceTo: []string{"User"}, Filterable: true, Sortable: true},
		},
	}
}

func taskMeta() ObjectMeta {
	return ObjectMeta{
		Name: "Task", Label: "Task", LabelPlural: "Tasks", KeyPrefix: "00T",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "WhoId", Label: "Name ID", Type: "reference", ReferenceTo: []string{"Contact", "Lead"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "WhatId", Label: "Related To ID", Type: "reference", ReferenceTo: []string{"Account", "Case", "Opportunity"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Subject", Label: "Subject", Type: "string", Length: 255, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "ActivityDate", Label: "Due Date", Type: "date", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Status", Label: "Status", Type: "picklist", Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Priority", Label: "Priority", Type: "picklist", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "OwnerId", Label: "Owner ID", Type: "reference", ReferenceTo: []string{"User", "Group"}, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Description", Label: "Description", Type: "textarea", Length: 32000, Nillable: true, Updateable: true, Createable: true},
		},
	}
}

func eventMeta() ObjectMeta {
	return ObjectMeta{
		Name: "Event", Label: "Event", LabelPlural: "Events", KeyPrefix: "00U",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "WhoId", Label: "Name ID", Type: "reference", ReferenceTo: []string{"Contact", "Lead"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "WhatId", Label: "Related To ID", Type: "reference", ReferenceTo: []string{"Account", "Case", "Opportunity"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Subject", Label: "Subject", Type: "string", Length: 255, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "ActivityDateTime", Label: "Start", Type: "datetime", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "DurationInMinutes", Label: "Duration", Type: "int", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Location", Label: "Location", Type: "string", Length: 255, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "OwnerId", Label: "Owner ID", Type: "reference", ReferenceTo: []string{"User", "Group"}, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Description", Label: "Description", Type: "textarea", Length: 32000, Nillable: true, Updateable: true, Createable: true},
		},
	}
}

func contentDocumentMeta() ObjectMeta {
	return ObjectMeta{
		Name: "ContentDocument", Label: "Content Document", LabelPlural: "Content Documents", KeyPrefix: "069",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "Title", Label: "Title", Type: "string", Length: 255, Updateable: true, Filterable: true, Sortable: true},
			{Name: "FileType", Label: "File Type", Type: "string", Length: 20, Nillable: true, Filterable: true, Sortable: true},
			{Name: "ContentSize", Label: "Size", Type: "int", Nillable: true, Filterable: true, Sortable: true},
			{Name: "OwnerId", Label: "Owner ID", Type: "reference", ReferenceTo: []string{"User"}, Updateable: true, Filterable: true, Sortable: true},
		},
	}
}

func contentVersionMeta() ObjectMeta {
	return ObjectMeta{
		Name: "ContentVersion", Label: "Content Version", LabelPlural: "Content Versions", KeyPrefix: "068",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "ContentDocumentId", Label: "Content Document ID", Type: "reference", ReferenceTo: []string{"ContentDocument"}, Nillable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Title", Label: "Title", Type: "string", Length: 255, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "VersionNumber", Label: "Version Number", Type: "string", Length: 20, Nillable: true, Filterable: true, Sortable: true},
			{Name: "FileType", Label: "File Type", Type: "string", Length: 20, Nillable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "ContentSize", Label: "Size", Type: "int", Nillable: true, Filterable: true, Sortable: true},
		},
	}
}

func entityDefinitionMeta() ObjectMeta {
	return ObjectMeta{
		Name: "EntityDefinition", Label: "Entity Definition", LabelPlural: "Entity Definitions", KeyPrefix: "000",
		Queryable: true, Retrieveable: true,
		customFields: []FieldMeta{
			{Name: "QualifiedApiName", Label: "Qualified API Name", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "DeveloperName", Label: "Object Name", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "MasterLabel", Label: "Master Label", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "Label", Label: "Label", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "KeyPrefix", Label: "Key Prefix", Type: "string", Length: 3, Nillable: true, Filterable: true, Sortable: true},
			{Name: "PluralLabel", Label: "Plural Label", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "IsCustomizable", Label: "Customizable", Type: "boolean", Filterable: true, Sortable: true},
		},
	}
}

func fieldDefinitionMeta() ObjectMeta {
	return ObjectMeta{
		Name: "FieldDefinition", Label: "Field Definition", LabelPlural: "Field Definitions", KeyPrefix: "000",
		Queryable: true, Retrieveable: true,
		customFields: []FieldMeta{
			{Name: "QualifiedApiName", Label: "Qualified API Name", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "DeveloperName", Label: "Field Name", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "Label", Label: "Label", Type: "string", Length: 240, Filterable: true, Sortable: true},
			{Name: "DataType", Label: "Data Type", Type: "string", Length: 60, Filterable: true, Sortable: true},
			{Name: "Length", Label: "Length", Type: "int", Nillable: true, Filterable: true, Sortable: true},
			{Name: "IsNillable", Label: "Nillable", Type: "boolean", Filterable: true, Sortable: true},
			{Name: "EntityDefinitionId", Label: "Entity Definition ID", Type: "string", Length: 240, Filterable: true, Sortable: true},
		},
	}
}

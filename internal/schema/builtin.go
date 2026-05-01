package schema

// init populates the registry with the built-in concrete SObject types and
// the virtual EntityDefinition / FieldDefinition types used for schema
// discovery via SOQL. Field metadata mirrors the legacy describe table in
// internal/handlers/sobject.go.
func init() {
	for _, m := range builtinObjects() {
		Register(m)
	}
}

func builtinObjects() []ObjectMeta {
	return []ObjectMeta{
		accountMeta(), contactMeta(), userMeta(), caseMeta(),
		emailMessageMeta(), caseCommentMeta(), feedItemMeta(), feedCommentMeta(),
		taskMeta(), eventMeta(), contentDocumentMeta(), contentVersionMeta(),
		entityDefinitionMeta(), fieldDefinitionMeta(),
	}
}

func accountMeta() ObjectMeta {
	return ObjectMeta{
		Name: "Account", Label: "Account", LabelPlural: "Accounts", KeyPrefix: "001",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "Name", Label: "Account Name", Type: "string", Length: 255, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Industry", Label: "Industry", Type: "picklist", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Type", Label: "Account Type", Type: "picklist", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Website", Label: "Website", Type: "url", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Phone", Label: "Phone", Type: "phone", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
		},
	}
}

func contactMeta() ObjectMeta {
	return ObjectMeta{
		Name: "Contact", Label: "Contact", LabelPlural: "Contacts", KeyPrefix: "003",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "FirstName", Label: "First Name", Type: "string", Length: 40, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "LastName", Label: "Last Name", Type: "string", Length: 80, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Email", Label: "Email", Type: "email", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Phone", Label: "Phone", Type: "phone", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "AccountId", Label: "Account ID", Type: "reference", ReferenceTo: []string{"Account"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
		},
	}
}

func userMeta() ObjectMeta {
	return ObjectMeta{
		Name: "User", Label: "User", LabelPlural: "Users", KeyPrefix: "005",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "Name", Label: "Full Name", Type: "string", Length: 255, Filterable: true, Sortable: true},
			{Name: "FirstName", Label: "First Name", Type: "string", Length: 40, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "LastName", Label: "Last Name", Type: "string", Length: 80, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Email", Label: "Email", Type: "email", Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Username", Label: "Username", Type: "string", Length: 80, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "IsActive", Label: "Active", Type: "boolean", Updateable: true, Createable: true, Filterable: true, Sortable: true},
		},
	}
}

func caseMeta() ObjectMeta {
	return ObjectMeta{
		Name: "Case", Label: "Case", LabelPlural: "Cases", KeyPrefix: "500",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "CaseNumber", Label: "Case Number", Type: "string", Length: 30, Filterable: true, Sortable: true},
			{Name: "Subject", Label: "Subject", Type: "string", Length: 255, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Status", Label: "Status", Type: "picklist", Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Priority", Label: "Priority", Type: "picklist", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Description", Label: "Description", Type: "textarea", Length: 32000, Nillable: true, Updateable: true, Createable: true},
			{Name: "AccountId", Label: "Account ID", Type: "reference", ReferenceTo: []string{"Account"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "ContactId", Label: "Contact ID", Type: "reference", ReferenceTo: []string{"Contact"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "OwnerId", Label: "Owner ID", Type: "reference", ReferenceTo: []string{"User", "Group"}, Updateable: true, Createable: true, Filterable: true, Sortable: true},
		},
	}
}

func emailMessageMeta() ObjectMeta {
	return ObjectMeta{
		Name: "EmailMessage", Label: "Email Message", LabelPlural: "Email Messages", KeyPrefix: "02s",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "ParentId", Label: "Parent ID", Type: "reference", ReferenceTo: []string{"Case"}, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Subject", Label: "Subject", Type: "string", Length: 3000, Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "TextBody", Label: "Text Body", Type: "textarea", Length: 32000, Nillable: true, Updateable: true, Createable: true},
			{Name: "HtmlBody", Label: "HTML Body", Type: "textarea", Length: 32000, Nillable: true, Updateable: true, Createable: true},
			{Name: "FromAddress", Label: "From Address", Type: "email", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "ToAddress", Label: "To Address", Type: "email", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "MessageDate", Label: "Message Date", Type: "datetime", Nillable: true, Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "Incoming", Label: "Is Incoming", Type: "boolean", Updateable: true, Createable: true, Filterable: true, Sortable: true},
		},
	}
}

func caseCommentMeta() ObjectMeta {
	return ObjectMeta{
		Name: "CaseComment", Label: "Case Comment", LabelPlural: "Case Comments", KeyPrefix: "00a",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "ParentId", Label: "Parent ID", Type: "reference", ReferenceTo: []string{"Case"}, Createable: true, Filterable: true, Sortable: true},
			{Name: "CommentBody", Label: "Body", Type: "textarea", Length: 4000, Nillable: true, Updateable: true, Createable: true},
			{Name: "IsPublished", Label: "Published", Type: "boolean", Updateable: true, Createable: true, Filterable: true, Sortable: true},
			{Name: "CreatedById", Label: "Created By ID", Type: "reference", ReferenceTo: []string{"User"}, Filterable: true, Sortable: true},
		},
	}
}

func feedItemMeta() ObjectMeta {
	return ObjectMeta{
		Name: "FeedItem", Label: "Feed Item", LabelPlural: "Feed Items", KeyPrefix: "0D5",
		Queryable: true, Createable: true, Updateable: true, Deletable: true, Retrieveable: true, Searchable: true,
		customFields: []FieldMeta{
			{Name: "ParentId", Label: "Parent ID", Type: "reference", ReferenceTo: []string{"Case", "Account", "Contact", "User"}, Createable: true, Filterable: true, Sortable: true},
			{Name: "Body", Label: "Body", Type: "textarea", Length: 10000, Nillable: true, Updateable: true, Createable: true},
			{Name: "Type", Label: "Type", Type: "picklist", Createable: true, Filterable: true, Sortable: true},
			{Name: "CreatedById", Label: "Created By ID", Type: "reference", ReferenceTo: []string{"User"}, Filterable: true, Sortable: true},
		},
	}
}

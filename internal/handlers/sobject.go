package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/store"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// SObjectHandler handles SObject CRUD requests.
type SObjectHandler struct {
	store  store.Store
	logger zerolog.Logger
}

// NewSObjectHandler creates a new SObject handler.
func NewSObjectHandler(s store.Store, logger zerolog.Logger) *SObjectHandler {
	return &SObjectHandler{
		store:  s,
		logger: logger.With().Str("handler", "sobject").Logger(),
	}
}

// CreateResponse represents the response from creating a record.
type CreateResponse struct {
	ID      string            `json:"id"`
	Success bool              `json:"success"`
	Errors  []models.APIError `json:"errors"`
}

// HandleGet retrieves a single SObject by ID.
// GET /services/data/vXX.0/sobjects/{type}/{id}
func (h *SObjectHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	objectType := r.PathValue("type")
	id := r.PathValue("id")

	h.logger.Debug().
		Str("objectType", objectType).
		Str("id", id).
		Msg("Getting SObject")

	record, err := h.store.Get(objectType, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND",
				"The requested resource does not exist")
			return
		}
		h.logger.Error().Err(err).Msg("Failed to get record")
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Add attributes metadata
	record["attributes"] = map[string]any{
		"type": objectType,
		"url":  "/services/data/v66.0/sobjects/" + objectType + "/" + id,
	}

	h.logger.Info().
		Str("objectType", objectType).
		Str("id", id).
		Msg("SObject retrieved")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(record)
}

// HandleCreate creates a new SObject.
// POST /services/data/vXX.0/sobjects/{type}
func (h *SObjectHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	objectType := r.PathValue("type")

	h.logger.Debug().
		Str("objectType", objectType).
		Msg("Creating SObject")

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body is required")
		return
	}

	var data store.Record
	if err := json.Unmarshal(body, &data); err != nil {
		h.writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", "Invalid JSON: "+err.Error())
		return
	}

	// Add created timestamp if not present
	if _, ok := data["CreatedDate"]; !ok {
		data["CreatedDate"] = time.Now().UTC().Format(time.RFC3339)
	}

	// Create record
	id, err := h.store.Create(objectType, data)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to create record")
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("objectType", objectType).
		Str("id", id).
		Msg("SObject created")

	response := CreateResponse{
		ID:      id,
		Success: true,
		Errors:  []models.APIError{},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// HandleUpdate updates an existing SObject.
// PATCH /services/data/vXX.0/sobjects/{type}/{id}
func (h *SObjectHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	objectType := r.PathValue("type")
	id := r.PathValue("id")

	h.logger.Debug().
		Str("objectType", objectType).
		Str("id", id).
		Msg("Updating SObject")

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body is required")
		return
	}

	var data store.Record
	if err := json.Unmarshal(body, &data); err != nil {
		h.writeError(w, http.StatusBadRequest, "JSON_PARSER_ERROR", "Invalid JSON: "+err.Error())
		return
	}

	// Don't allow changing the ID
	delete(data, "Id")

	// Add modified timestamp
	data["LastModifiedDate"] = time.Now().UTC().Format(time.RFC3339)

	// Update record
	if err := h.store.Update(objectType, id, data); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND",
				"The requested resource does not exist")
			return
		}
		h.logger.Error().Err(err).Msg("Failed to update record")
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("objectType", objectType).
		Str("id", id).
		Msg("SObject updated")

	w.WriteHeader(http.StatusNoContent)
}

// HandleDelete deletes an SObject.
// DELETE /services/data/vXX.0/sobjects/{type}/{id}
func (h *SObjectHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	objectType := r.PathValue("type")
	id := r.PathValue("id")

	h.logger.Debug().
		Str("objectType", objectType).
		Str("id", id).
		Msg("Deleting SObject")

	if err := h.store.Delete(objectType, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND",
				"The requested resource does not exist")
			return
		}
		h.logger.Error().Err(err).Msg("Failed to delete record")
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("objectType", objectType).
		Str("id", id).
		Msg("SObject deleted")

	w.WriteHeader(http.StatusNoContent)
}

// HandleDescribe returns metadata about an SObject type.
// GET /services/data/vXX.0/sobjects/{type}/describe
func (h *SObjectHandler) HandleDescribe(w http.ResponseWriter, r *http.Request) {
	objectType := r.PathValue("type")

	h.logger.Debug().
		Str("objectType", objectType).
		Msg("Describing SObject")

	// Get object metadata
	metadata := h.getObjectMetadata(objectType)
	if metadata == nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND",
			"The requested resource does not exist")
		return
	}

	h.logger.Info().
		Str("objectType", objectType).
		Msg("SObject described")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metadata)
}

// getObjectMetadata returns metadata for known object types.
func (h *SObjectHandler) getObjectMetadata(objectType string) map[string]any {
	// Common fields for all objects
	commonFields := []map[string]any{
		{
			"name":         "Id",
			"label":        "Record ID",
			"type":         "id",
			"length":       18,
			"nillable":     false,
			"updateable":   false,
			"createable":   false,
			"filterable":   true,
			"sortable":     true,
			"unique":       true,
			"externalId":   false,
			"defaultValue": nil,
		},
		{
			"name":         "CreatedDate",
			"label":        "Created Date",
			"type":         "datetime",
			"nillable":     false,
			"updateable":   false,
			"createable":   false,
			"filterable":   true,
			"sortable":     true,
			"unique":       false,
			"externalId":   false,
			"defaultValue": nil,
		},
		{
			"name":         "LastModifiedDate",
			"label":        "Last Modified Date",
			"type":         "datetime",
			"nillable":     true,
			"updateable":   false,
			"createable":   false,
			"filterable":   true,
			"sortable":     true,
			"unique":       false,
			"externalId":   false,
			"defaultValue": nil,
		},
	}

	// Object-specific metadata
	objectMeta := map[string]struct {
		label        string
		labelPlural  string
		keyPrefix    string
		customFields []map[string]any
	}{
		"Case": {
			label:       "Case",
			labelPlural: "Cases",
			keyPrefix:   "500",
			customFields: []map[string]any{
				{
					"name":       "CaseNumber",
					"label":      "Case Number",
					"type":       "string",
					"length":     30,
					"nillable":   false,
					"updateable": false,
					"createable": false,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Subject",
					"label":      "Subject",
					"type":       "string",
					"length":     255,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Status",
					"label":      "Status",
					"type":       "picklist",
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Priority",
					"label":      "Priority",
					"type":       "picklist",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Description",
					"label":      "Description",
					"type":       "textarea",
					"length":     32000,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": false,
					"sortable":   false,
				},
				{
					"name":       "AccountId",
					"label":      "Account ID",
					"type":       "reference",
					"referenceTo": []string{"Account"},
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "ContactId",
					"label":      "Contact ID",
					"type":       "reference",
					"referenceTo": []string{"Contact"},
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "OwnerId",
					"label":      "Owner ID",
					"type":       "reference",
					"referenceTo": []string{"User", "Group"},
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
		"EmailMessage": {
			label:       "Email Message",
			labelPlural: "Email Messages",
			keyPrefix:   "02s",
			customFields: []map[string]any{
				{
					"name":       "ParentId",
					"label":      "Parent ID",
					"type":       "reference",
					"referenceTo": []string{"Case"},
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Subject",
					"label":      "Subject",
					"type":       "string",
					"length":     3000,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "TextBody",
					"label":      "Text Body",
					"type":       "textarea",
					"length":     32000,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": false,
					"sortable":   false,
				},
				{
					"name":       "HtmlBody",
					"label":      "HTML Body",
					"type":       "textarea",
					"length":     32000,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": false,
					"sortable":   false,
				},
				{
					"name":       "FromAddress",
					"label":      "From Address",
					"type":       "email",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "ToAddress",
					"label":      "To Address",
					"type":       "email",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "MessageDate",
					"label":      "Message Date",
					"type":       "datetime",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Incoming",
					"label":      "Is Incoming",
					"type":       "boolean",
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
		"CaseComment": {
			label:       "Case Comment",
			labelPlural: "Case Comments",
			keyPrefix:   "00a",
			customFields: []map[string]any{
				{
					"name":       "ParentId",
					"label":      "Parent ID",
					"type":       "reference",
					"referenceTo": []string{"Case"},
					"nillable":   false,
					"updateable": false,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "CommentBody",
					"label":      "Body",
					"type":       "textarea",
					"length":     4000,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": false,
					"sortable":   false,
				},
				{
					"name":       "IsPublished",
					"label":      "Published",
					"type":       "boolean",
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "CreatedById",
					"label":      "Created By ID",
					"type":       "reference",
					"referenceTo": []string{"User"},
					"nillable":   false,
					"updateable": false,
					"createable": false,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
		"FeedItem": {
			label:       "Feed Item",
			labelPlural: "Feed Items",
			keyPrefix:   "0D5",
			customFields: []map[string]any{
				{
					"name":       "ParentId",
					"label":      "Parent ID",
					"type":       "reference",
					"referenceTo": []string{"Case", "Account", "Contact", "User"},
					"nillable":   false,
					"updateable": false,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Body",
					"label":      "Body",
					"type":       "textarea",
					"length":     10000,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": false,
					"sortable":   false,
				},
				{
					"name":       "Type",
					"label":      "Type",
					"type":       "picklist",
					"nillable":   false,
					"updateable": false,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "CreatedById",
					"label":      "Created By ID",
					"type":       "reference",
					"referenceTo": []string{"User"},
					"nillable":   false,
					"updateable": false,
					"createable": false,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
		"Account": {
			label:       "Account",
			labelPlural: "Accounts",
			keyPrefix:   "001",
			customFields: []map[string]any{
				{
					"name":       "Name",
					"label":      "Account Name",
					"type":       "string",
					"length":     255,
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Industry",
					"label":      "Industry",
					"type":       "picklist",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Type",
					"label":      "Account Type",
					"type":       "picklist",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Website",
					"label":      "Website",
					"type":       "url",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Phone",
					"label":      "Phone",
					"type":       "phone",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
		"Contact": {
			label:       "Contact",
			labelPlural: "Contacts",
			keyPrefix:   "003",
			customFields: []map[string]any{
				{
					"name":       "FirstName",
					"label":      "First Name",
					"type":       "string",
					"length":     40,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "LastName",
					"label":      "Last Name",
					"type":       "string",
					"length":     80,
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Email",
					"label":      "Email",
					"type":       "email",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Phone",
					"label":      "Phone",
					"type":       "phone",
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "AccountId",
					"label":      "Account ID",
					"type":       "reference",
					"referenceTo": []string{"Account"},
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
		"User": {
			label:       "User",
			labelPlural: "Users",
			keyPrefix:   "005",
			customFields: []map[string]any{
				{
					"name":       "Name",
					"label":      "Full Name",
					"type":       "string",
					"length":     255,
					"nillable":   false,
					"updateable": false,
					"createable": false,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "FirstName",
					"label":      "First Name",
					"type":       "string",
					"length":     40,
					"nillable":   true,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "LastName",
					"label":      "Last Name",
					"type":       "string",
					"length":     80,
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Email",
					"label":      "Email",
					"type":       "email",
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "Username",
					"label":      "Username",
					"type":       "string",
					"length":     80,
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
				{
					"name":       "IsActive",
					"label":      "Active",
					"type":       "boolean",
					"nillable":   false,
					"updateable": true,
					"createable": true,
					"filterable": true,
					"sortable":   true,
				},
			},
		},
	}

	meta, ok := objectMeta[objectType]
	if !ok {
		// Check if data exists for this object type (for dynamic objects)
		types := h.store.ObjectTypes()
		found := false
		for _, t := range types {
			if strings.EqualFold(t, objectType) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		// Return minimal metadata for unknown but existing types
		meta = struct {
			label        string
			labelPlural  string
			keyPrefix    string
			customFields []map[string]any
		}{
			label:        objectType,
			labelPlural:  objectType + "s",
			keyPrefix:    "000",
			customFields: []map[string]any{},
		}
	}

	// Combine common and custom fields
	allFields := make([]map[string]any, 0, len(commonFields)+len(meta.customFields))
	allFields = append(allFields, commonFields...)
	allFields = append(allFields, meta.customFields...)

	return map[string]any{
		"name":        objectType,
		"label":       meta.label,
		"labelPlural": meta.labelPlural,
		"keyPrefix":   meta.keyPrefix,
		"urls": map[string]string{
			"sobject":   "/services/data/v66.0/sobjects/" + objectType,
			"describe":  "/services/data/v66.0/sobjects/" + objectType + "/describe",
			"rowTemplate": "/services/data/v66.0/sobjects/" + objectType + "/{ID}",
		},
		"fields":       allFields,
		"createable":   true,
		"updateable":   true,
		"deletable":    true,
		"queryable":    true,
		"retrieveable": true,
		"searchable":   true,
		"custom":       false,
	}
}

// writeError writes a Salesforce-style error response.
func (h *SObjectHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode([]models.APIError{
		{
			Message:   message,
			ErrorCode: code,
		},
	})
}

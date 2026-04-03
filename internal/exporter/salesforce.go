package exporter

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
)

// SalesforceExporter exports mock data to Salesforce JSON seed format.
type SalesforceExporter struct {
	db     *sql.DB
	logger zerolog.Logger
}

// intToBool returns a transform function that converts integer fields (0/1) to booleans.
func intToBool(fields ...string) func([]map[string]any) []map[string]any {
	return func(rows []map[string]any) []map[string]any {
		for _, row := range rows {
			for _, field := range fields {
				if v, ok := row[field]; ok {
					switch val := v.(type) {
					case int64:
						row[field] = val != 0
					case int:
						row[field] = val != 0
					case float64:
						row[field] = val != 0
					}
				}
			}
		}
		return rows
	}
}

// NewSalesforceExporter creates a new Salesforce exporter.
func NewSalesforceExporter(db *sql.DB, logger zerolog.Logger) *SalesforceExporter {
	return &SalesforceExporter{
		db:     db,
		logger: logger.With().Str("exporter", "salesforce").Logger(),
	}
}

// Export writes all Salesforce seed files to the output directory.
func (e *SalesforceExporter) Export(outDir string) error {
	e.logger.Info().Str("dir", outDir).Msg("Exporting Salesforce seed data")

	exports := []struct {
		name  string
		query string
		transform func([]map[string]any) []map[string]any
	}{
		{
			name: "accounts.json",
			query: `SELECT id AS Id, name AS Name, industry AS Industry, type AS Type,
				website AS Website, phone AS Phone, billing_city AS BillingCity,
				billing_state AS BillingState, annual_revenue AS AnnualRevenue,
				num_employees AS NumberOfEmployees, created_at AS CreatedDate
				FROM accounts ORDER BY name`,
		},
		{
			name: "contacts.json",
			query: `SELECT id AS Id, account_id AS AccountId, first_name AS FirstName,
				last_name AS LastName, email AS Email, phone AS Phone,
				title AS Title, department AS Department, created_at AS CreatedDate
				FROM contacts ORDER BY last_name`,
		},
		{
			name: "users.json",
			query: `SELECT id AS Id, first_name || ' ' || last_name AS Name,
				first_name AS FirstName, last_name AS LastName, email AS Email,
				username AS Username, title AS Title, department AS Department,
				is_active AS IsActive, user_role AS UserRole, created_at AS CreatedDate
				FROM users ORDER BY last_name`,
		},
		{
			name: "cases.json",
			query: `SELECT c.id AS Id, c.case_number AS CaseNumber, c.subject AS Subject,
				c.description AS Description, c.status AS Status, c.priority AS Priority,
				c.product AS "Product__c", c.case_type AS Type, c.origin AS Origin,
				c.reason AS Reason, c.owner_id AS OwnerId, c.contact_id AS ContactId,
				c.account_id AS AccountId, c.created_at AS CreatedDate,
				c.closed_at AS ClosedDate,
				ct.email AS ContactEmail, ct.phone AS ContactPhone,
				c.is_closed AS IsClosed, c.is_escalated AS IsEscalated
				FROM cases c
				LEFT JOIN contacts ct ON c.contact_id = ct.id
				ORDER BY c.created_at`,
			transform: intToBool("IsClosed", "IsEscalated"),
		},
		{
			name: "email_messages.json",
			query: `SELECT id AS Id, case_id AS ParentId, subject AS Subject,
				text_body AS TextBody, html_body AS HtmlBody, from_address AS FromAddress,
				from_name AS FromName, to_address AS ToAddress, cc_address AS CcAddress,
				bcc_address AS BccAddress, message_date AS MessageDate, status AS Status,
				incoming AS Incoming, has_attachment AS HasAttachment, headers AS Headers
				FROM email_messages ORDER BY case_id, sequence_num`,
		},
		{
			name: "case_comments.json",
			query: `SELECT id AS Id, case_id AS ParentId, comment_body AS CommentBody,
				created_by_id AS CreatedById, created_at AS CreatedDate,
				is_published AS IsPublished
				FROM case_comments ORDER BY case_id, created_at`,
		},
		{
			name: "feed_items.json",
			query: `SELECT id AS Id, case_id AS ParentId, body AS Body, type AS Type,
				created_by_id AS CreatedById, created_at AS CreatedDate
				FROM feed_items ORDER BY case_id, created_at`,
		},
	}

	for _, exp := range exports {
		e.logger.Info().Str("file", exp.name).Msg("Exporting")

		data, err := queryToMaps(e.db, exp.query, e.logger)
		if err != nil {
			return fmt.Errorf("export %s: %w", exp.name, err)
		}

		if exp.transform != nil {
			data = exp.transform(data)
		}

		path := filepath.Join(outDir, exp.name)
		if err := writeJSON(path, data); err != nil {
			return fmt.Errorf("write %s: %w", exp.name, err)
		}

		e.logger.Info().Str("file", exp.name).Int("records", len(data)).Msg("Exported")
	}

	return nil
}

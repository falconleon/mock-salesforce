package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// EmailGenerator generates EmailMessage records for cases.
type EmailGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewEmailGenerator creates a new email generator.
func NewEmailGenerator(ctx *Context) *EmailGenerator {
	return &EmailGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "emails").Logger(),
	}
}

// EmailMessage represents a generated email.
type EmailMessage struct {
	ID            string `json:"id"`
	CaseID        string `json:"case_id"`
	Subject       string `json:"subject"`
	TextBody      string `json:"text_body"`
	HtmlBody      string `json:"html_body"`
	FromAddress   string `json:"from_address"`
	FromName      string `json:"from_name"`
	ToAddress     string `json:"to_address"`
	CcAddress     string `json:"cc_address,omitempty"`
	BccAddress    string `json:"bcc_address,omitempty"`
	MessageDate   string `json:"message_date"`
	Status        string `json:"status"`
	Incoming      bool   `json:"incoming"`
	HasAttachment bool   `json:"has_attachment"`
	Headers       string `json:"headers"`
	SequenceNum   int    `json:"sequence_num"`
}

// LLMEmailResponse represents a single email in the LLM-generated thread.
type LLMEmailResponse struct {
	Subject     string `json:"subject"`
	TextBody    string `json:"text_body"`
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
	MessageDate string `json:"message_date"`
	Incoming    bool   `json:"incoming"`
}

// caseEmailContext holds case and participant data for email generation.
type caseEmailContext struct {
	caseID       string
	subject      string
	description  string
	status       string
	priority     string
	createdAt    time.Time
	contactName  string
	contactEmail string
	agentName    string
	agentEmail   string
}

// Generate creates email threads for all cases in the database.
func (g *EmailGenerator) Generate() error {
	g.logger.Info().Msg("Generating email threads")

	// Fetch all cases with their contacts and owners
	cases, err := g.fetchCasesWithParticipants()
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		g.logger.Warn().Msg("No cases found, skipping email generation")
		return nil
	}

	g.logger.Debug().Int("cases", len(cases)).Msg("Found cases for email generation")

	// Process each case
	totalEmails := 0
	for i, c := range cases {
		// Determine thread length based on distribution
		threadLen := g.pickThreadLength()

		// Generate email thread via LLM
		emails, err := g.generateEmailThread(c, threadLen)
		if err != nil {
			g.logger.Warn().Err(err).Str("case", c.caseID).Msg("LLM failed, using fallback")
			emails = g.defaultEmailThread(c, threadLen)
		}

		// Insert emails into database
		if err := g.insertEmails(emails); err != nil {
			g.logger.Error().Err(err).Str("case", c.caseID).Msg("Failed to insert emails")
			continue
		}

		totalEmails += len(emails)

		// Progress logging for each case
		g.logger.Info().
			Int("progress", i+1).
			Int("total", len(cases)).
			Str("case_id", c.caseID).
			Int("emails", len(emails)).
			Msg("Emails generated for case")
	}

	g.logger.Info().Int("emails", totalEmails).Int("cases", len(cases)).Msg("Email threads generated")
	return nil
}

// fetchCasesWithParticipants retrieves cases with contact and owner details.
func (g *EmailGenerator) fetchCasesWithParticipants() ([]caseEmailContext, error) {
	rows, err := g.db.Query(`
		SELECT
			ca.id, ca.subject, ca.description, ca.status, ca.priority, ca.created_at,
			c.first_name || ' ' || c.last_name, c.email,
			u.first_name || ' ' || u.last_name, u.email
		FROM cases ca
		JOIN contacts c ON c.id = ca.contact_id
		JOIN users u ON u.id = ca.owner_id
	`)
	if err != nil {
		return nil, fmt.Errorf("query cases with participants: %w", err)
	}
	defer rows.Close()

	var result []caseEmailContext
	for rows.Next() {
		var c caseEmailContext
		var createdAtStr string
		if err := rows.Scan(
			&c.caseID, &c.subject, &c.description, &c.status, &c.priority, &createdAtStr,
			&c.contactName, &c.contactEmail,
			&c.agentName, &c.agentEmail,
		); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		c.createdAt, _ = time.Parse(time.RFC3339, createdAtStr)
		result = append(result, c)
	}
	return result, rows.Err()
}

// pickThreadLength returns the number of emails based on distribution.
// 40% short (2-3), 40% medium (4-6), 20% long (7-10)
func (g *EmailGenerator) pickThreadLength() int {
	r := rand.Float64()
	switch {
	case r < 0.40:
		// Short: 2-3 emails
		return 2 + rand.Intn(2)
	case r < 0.80:
		// Medium: 4-6 emails
		return 4 + rand.Intn(3)
	default:
		// Long: 7-10 emails
		return 7 + rand.Intn(4)
	}
}

// generateEmailThread uses LLM to generate a coherent email thread.
func (g *EmailGenerator) generateEmailThread(c caseEmailContext, threadLen int) ([]EmailMessage, error) {
	prompt := fmt.Sprintf(`Generate a realistic email thread for a support case.

Case Details:
- Subject: %s
- Description: %s
- Status: %s
- Priority: %s
- Contact: %s <%s>
- Support Agent: %s <%s>

Generate exactly %d emails in the thread as a JSON array. The thread should:
1. Start with an initial inquiry from the contact (incoming=true)
2. Alternate between contact and support agent
3. Be contextually coherent - each email should logically follow the previous
4. End appropriately based on the conversation flow

Each email must have these fields:
- subject: Email subject line (use "Re: " prefix for replies)
- text_body: Email body text (2-5 sentences)
- from_address: Sender email
- to_address: Recipient email
- message_date: ISO 8601 timestamp
- incoming: true if from contact, false if from agent

Return ONLY a valid JSON array:
[{"subject":"...","text_body":"...","from_address":"...","to_address":"...","message_date":"...","incoming":true}]`,
		c.subject, truncateDescription(c.description), c.status, c.priority,
		c.contactName, c.contactEmail, c.agentName, c.agentEmail, threadLen)

	resp, err := g.llm.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	// Extract JSON array from response
	jsonStr := extractJSONArray(resp)

	var llmEmails []LLMEmailResponse
	if err := json.Unmarshal([]byte(jsonStr), &llmEmails); err != nil {
		return nil, fmt.Errorf("parse llm response: %w", err)
	}

	// Convert to EmailMessage with proper IDs and timestamps
	return g.buildEmailMessages(c, llmEmails)
}

// buildEmailMessages converts LLM responses to EmailMessage records.
func (g *EmailGenerator) buildEmailMessages(c caseEmailContext, llmEmails []LLMEmailResponse) ([]EmailMessage, error) {
	emails := make([]EmailMessage, 0, len(llmEmails))
	baseTime := c.createdAt

	for i, le := range llmEmails {
		// Parse message date from LLM or generate one
		msgDate := baseTime.Add(time.Duration(i) * time.Hour * 4) // ~4 hours between emails
		if le.MessageDate != "" {
			if parsed, err := time.Parse(time.RFC3339, le.MessageDate); err == nil {
				msgDate = parsed
			}
		}

		// Ensure chronological order
		if i > 0 && len(emails) > 0 {
			prevDate, _ := time.Parse(time.RFC3339, emails[i-1].MessageDate)
			if !msgDate.After(prevDate) {
				msgDate = prevDate.Add(time.Hour * 2)
			}
		}

		// Determine from/to based on incoming flag
		fromAddr, fromName, toAddr := le.FromAddress, "", le.ToAddress
		if le.Incoming {
			fromAddr = c.contactEmail
			fromName = c.contactName
			toAddr = c.agentEmail
		} else {
			fromAddr = c.agentEmail
			fromName = c.agentName
			toAddr = c.contactEmail
		}

		email := EmailMessage{
			ID:          SalesforceID("EmailMessage"),
			CaseID:      c.caseID,
			Subject:     le.Subject,
			TextBody:    le.TextBody,
			FromAddress: fromAddr,
			FromName:    fromName,
			ToAddress:   toAddr,
			MessageDate: msgDate.Format(time.RFC3339),
			Status:      "Sent",
			Incoming:    le.Incoming,
			SequenceNum: i + 1,
		}
		emails = append(emails, email)
	}

	return emails, nil
}

// defaultEmailThread generates fallback content when LLM fails.
func (g *EmailGenerator) defaultEmailThread(c caseEmailContext, threadLen int) []EmailMessage {
	emails := make([]EmailMessage, 0, threadLen)
	baseTime := c.createdAt

	// Template responses for alternating messages
	contactOpeners := []string{
		"Hello, I'm experiencing an issue with %s. Could you please help?",
		"Hi Support Team, I need assistance with %s. This is affecting our workflow.",
		"Hi, We've encountered a problem related to %s. Can you look into this?",
	}
	agentResponses := []string{
		"Thank you for reaching out. I understand you're experiencing issues with %s. Let me investigate this for you.",
		"Hi %s, I've reviewed your case and I'd like to gather more information. Could you provide the error messages you're seeing?",
		"I've looked into this issue and found a potential solution. Please try the following steps...",
		"Following up on your case - have the suggested steps resolved your issue?",
		"Great news! The issue has been identified and a fix has been applied. Please verify on your end.",
	}
	contactFollowups := []string{
		"Thank you for the quick response. I've tried the steps but still see the issue.",
		"I've provided the additional information requested. Please let me know if you need anything else.",
		"The solution worked! Thank you for your help in resolving this.",
		"I can confirm the issue is now resolved. Appreciate your support!",
	}

	for i := 0; i < threadLen; i++ {
		msgDate := baseTime.Add(time.Duration(i) * time.Hour * 4)
		incoming := i%2 == 0 // Alternate: contact, agent, contact, agent...

		var subject, body, fromAddr, fromName, toAddr string
		if incoming {
			fromAddr = c.contactEmail
			fromName = c.contactName
			toAddr = c.agentEmail
			if i == 0 {
				subject = c.subject
				body = fmt.Sprintf(contactOpeners[rand.Intn(len(contactOpeners))], c.subject)
			} else {
				subject = "Re: " + c.subject
				body = contactFollowups[rand.Intn(len(contactFollowups))]
			}
		} else {
			fromAddr = c.agentEmail
			fromName = c.agentName
			toAddr = c.contactEmail
			subject = "Re: " + c.subject
			resp := agentResponses[rand.Intn(len(agentResponses))]
			if strings.Contains(resp, "%s") {
				body = fmt.Sprintf(resp, c.contactName)
			} else {
				body = resp
			}
		}

		email := EmailMessage{
			ID:          SalesforceID("EmailMessage"),
			CaseID:      c.caseID,
			Subject:     subject,
			TextBody:    body,
			FromAddress: fromAddr,
			FromName:    fromName,
			ToAddress:   toAddr,
			MessageDate: msgDate.Format(time.RFC3339),
			Status:      "Sent",
			Incoming:    incoming,
			SequenceNum: i + 1,
		}
		emails = append(emails, email)
	}

	return emails
}

// insertEmails inserts a batch of emails into the database.
func (g *EmailGenerator) insertEmails(emails []EmailMessage) error {
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO email_messages
		(id, case_id, subject, text_body, html_body, from_address, from_name, to_address,
		 cc_address, bcc_address, message_date, status, incoming, has_attachment, headers, sequence_num)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, e := range emails {
		incoming := 0
		if e.Incoming {
			incoming = 1
		}
		hasAttachment := 0
		if e.HasAttachment {
			hasAttachment = 1
		}

		_, err := stmt.Exec(
			e.ID, e.CaseID, e.Subject, e.TextBody, e.HtmlBody,
			e.FromAddress, e.FromName, e.ToAddress, e.CcAddress, e.BccAddress,
			e.MessageDate, e.Status, incoming, hasAttachment, e.Headers, e.SequenceNum,
		)
		if err != nil {
			return fmt.Errorf("insert email %s: %w", e.ID, err)
		}
	}

	return tx.Commit()
}

// extractJSONArray extracts JSON array from a response that may contain markdown.
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// truncateDescription shortens description for LLM prompt.
func truncateDescription(desc string) string {
	const maxLen = 500
	if len(desc) <= maxLen {
		return desc
	}
	return desc[:maxLen] + "..."
}

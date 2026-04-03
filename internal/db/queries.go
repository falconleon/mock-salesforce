package db

import (
	"database/sql"
	"fmt"
)

// boolToInt converts a boolean to SQLite integer (0/1).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableString returns nil for empty strings (for NULL in SQLite).
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// BeginTx starts a new transaction.
func (s *Store) BeginTx() (*sql.Tx, error) {
	return s.db.Begin()
}

// --- Account Operations ---

// InsertAccount inserts a single account.
func (s *Store) InsertAccount(a *Account) error {
	_, err := s.db.Exec(`INSERT INTO accounts (id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Industry, a.Type, a.Website, a.Phone, a.BillingCity, a.BillingState, a.AnnualRevenue, a.NumEmployees, a.CreatedAt)
	return err
}

// InsertAccountsBatch inserts multiple accounts in a transaction.
func (s *Store) InsertAccountsBatch(tx *sql.Tx, accounts []Account) error {
	stmt, err := tx.Prepare(`INSERT INTO accounts (id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare account insert: %w", err)
	}
	defer stmt.Close()

	for _, a := range accounts {
		if _, err := stmt.Exec(a.ID, a.Name, a.Industry, a.Type, a.Website, a.Phone, a.BillingCity, a.BillingState, a.AnnualRevenue, a.NumEmployees, a.CreatedAt); err != nil {
			return fmt.Errorf("insert account %s: %w", a.ID, err)
		}
	}
	return nil
}

// GetAccountByID retrieves an account by ID.
func (s *Store) GetAccountByID(id string) (*Account, error) {
	row := s.db.QueryRow(`SELECT id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at FROM accounts WHERE id = ?`, id)
	a := &Account{}
	var website, phone, billingCity, billingState sql.NullString
	var annualRevenue sql.NullFloat64
	var numEmployees sql.NullInt64
	err := row.Scan(&a.ID, &a.Name, &a.Industry, &a.Type, &website, &phone, &billingCity, &billingState, &annualRevenue, &numEmployees, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	a.Website = website.String
	a.Phone = phone.String
	a.BillingCity = billingCity.String
	a.BillingState = billingState.String
	a.AnnualRevenue = annualRevenue.Float64
	a.NumEmployees = int(numEmployees.Int64)
	return a, nil
}

// QueryAccounts retrieves all accounts.
func (s *Store) QueryAccounts() ([]Account, error) {
	rows, err := s.db.Query(`SELECT id, name, industry, type, website, phone, billing_city, billing_state, annual_revenue, num_employees, created_at FROM accounts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		var website, phone, billingCity, billingState sql.NullString
		var annualRevenue sql.NullFloat64
		var numEmployees sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Name, &a.Industry, &a.Type, &website, &phone, &billingCity, &billingState, &annualRevenue, &numEmployees, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Website = website.String
		a.Phone = phone.String
		a.BillingCity = billingCity.String
		a.BillingState = billingState.String
		a.AnnualRevenue = annualRevenue.Float64
		a.NumEmployees = int(numEmployees.Int64)
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// --- Contact Operations ---

// InsertContact inserts a single contact.
func (s *Store) InsertContact(c *Contact) error {
	_, err := s.db.Exec(`INSERT INTO contacts (id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.AccountID, c.FirstName, c.LastName, c.Email, c.Phone, c.Title, c.Department, boolToInt(c.IsPrimary), c.CreatedAt)
	return err
}

// InsertContactsBatch inserts multiple contacts in a transaction.
func (s *Store) InsertContactsBatch(tx *sql.Tx, contacts []Contact) error {
	stmt, err := tx.Prepare(`INSERT INTO contacts (id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare contact insert: %w", err)
	}
	defer stmt.Close()

	for _, c := range contacts {
		if _, err := stmt.Exec(c.ID, c.AccountID, c.FirstName, c.LastName, c.Email, c.Phone, c.Title, c.Department, boolToInt(c.IsPrimary), c.CreatedAt); err != nil {
			return fmt.Errorf("insert contact %s: %w", c.ID, err)
		}
	}
	return nil
}

// GetContactByID retrieves a contact by ID.
func (s *Store) GetContactByID(id string) (*Contact, error) {
	row := s.db.QueryRow(`SELECT id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at FROM contacts WHERE id = ?`, id)
	c := &Contact{}
	var phone, title, department sql.NullString
	var isPrimary int
	err := row.Scan(&c.ID, &c.AccountID, &c.FirstName, &c.LastName, &c.Email, &phone, &title, &department, &isPrimary, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.Phone = phone.String
	c.Title = title.String
	c.Department = department.String
	c.IsPrimary = isPrimary == 1
	return c, nil
}

// QueryContacts retrieves all contacts.
func (s *Store) QueryContacts() ([]Contact, error) {
	rows, err := s.db.Query(`SELECT id, account_id, first_name, last_name, email, phone, title, department, is_primary, created_at FROM contacts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		var phone, title, department sql.NullString
		var isPrimary int
		if err := rows.Scan(&c.ID, &c.AccountID, &c.FirstName, &c.LastName, &c.Email, &phone, &title, &department, &isPrimary, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Phone = phone.String
		c.Title = title.String
		c.Department = department.String
		c.IsPrimary = isPrimary == 1
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// --- User Operations ---

// InsertUser inserts a single user.
func (s *Store) InsertUser(u *User) error {
	_, err := s.db.Exec(`INSERT INTO users (id, first_name, last_name, email, username, title, department, is_active, manager_id, user_role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.FirstName, u.LastName, u.Email, u.Username, u.Title, u.Department, boolToInt(u.IsActive), nullableString(u.ManagerID), u.UserRole, u.CreatedAt)
	return err
}

// InsertUsersBatch inserts multiple users in a transaction.
func (s *Store) InsertUsersBatch(tx *sql.Tx, users []User) error {
	stmt, err := tx.Prepare(`INSERT INTO users (id, first_name, last_name, email, username, title, department, is_active, manager_id, user_role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare user insert: %w", err)
	}
	defer stmt.Close()

	for _, u := range users {
		if _, err := stmt.Exec(u.ID, u.FirstName, u.LastName, u.Email, u.Username, u.Title, u.Department, boolToInt(u.IsActive), nullableString(u.ManagerID), u.UserRole, u.CreatedAt); err != nil {
			return fmt.Errorf("insert user %s: %w", u.ID, err)
		}
	}
	return nil
}

// GetUserByID retrieves a user by ID.
func (s *Store) GetUserByID(id string) (*User, error) {
	row := s.db.QueryRow(`SELECT id, first_name, last_name, email, username, title, department, is_active, manager_id, user_role, created_at FROM users WHERE id = ?`, id)
	u := &User{}
	var title, department, managerID, userRole sql.NullString
	var isActive int
	err := row.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Username, &title, &department, &isActive, &managerID, &userRole, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.Title = title.String
	u.Department = department.String
	u.IsActive = isActive == 1
	u.ManagerID = managerID.String
	u.UserRole = userRole.String
	return u, nil
}

// QueryUsers retrieves all users.
func (s *Store) QueryUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, first_name, last_name, email, username, title, department, is_active, manager_id, user_role, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var title, department, managerID, userRole sql.NullString
		var isActive int
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Username, &title, &department, &isActive, &managerID, &userRole, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Title = title.String
		u.Department = department.String
		u.IsActive = isActive == 1
		u.ManagerID = managerID.String
		u.UserRole = userRole.String
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- Case Operations ---

// InsertCase inserts a single case.
func (s *Store) InsertCase(c *Case) error {
	_, err := s.db.Exec(`INSERT INTO cases (id, case_number, subject, description, status, priority, product, case_type, origin, reason, owner_id, contact_id, account_id, created_at, closed_at, is_closed, is_escalated, jira_issue_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.CaseNumber, c.Subject, c.Description, c.Status, c.Priority, c.Product, c.CaseType, c.Origin, c.Reason, c.OwnerID, c.ContactID, c.AccountID, c.CreatedAt, c.ClosedAt, boolToInt(c.IsClosed), boolToInt(c.IsEscalated), c.JiraIssueKey)
	return err
}

// InsertCasesBatch inserts multiple cases in a transaction.
func (s *Store) InsertCasesBatch(tx *sql.Tx, cases []Case) error {
	stmt, err := tx.Prepare(`INSERT INTO cases (id, case_number, subject, description, status, priority, product, case_type, origin, reason, owner_id, contact_id, account_id, created_at, closed_at, is_closed, is_escalated, jira_issue_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare case insert: %w", err)
	}
	defer stmt.Close()

	for _, c := range cases {
		if _, err := stmt.Exec(c.ID, c.CaseNumber, c.Subject, c.Description, c.Status, c.Priority, c.Product, c.CaseType, c.Origin, c.Reason, c.OwnerID, c.ContactID, c.AccountID, c.CreatedAt, c.ClosedAt, boolToInt(c.IsClosed), boolToInt(c.IsEscalated), c.JiraIssueKey); err != nil {
			return fmt.Errorf("insert case %s: %w", c.ID, err)
		}
	}
	return nil
}

// GetCaseByID retrieves a case by ID.
func (s *Store) GetCaseByID(id string) (*Case, error) {
	row := s.db.QueryRow(`SELECT id, case_number, subject, description, status, priority, product, case_type, origin, reason, owner_id, contact_id, account_id, created_at, closed_at, is_closed, is_escalated, jira_issue_key FROM cases WHERE id = ?`, id)
	c := &Case{}
	var product, caseType, origin, reason, closedAt, jiraIssueKey sql.NullString
	var isClosed, isEscalated int
	err := row.Scan(&c.ID, &c.CaseNumber, &c.Subject, &c.Description, &c.Status, &c.Priority, &product, &caseType, &origin, &reason, &c.OwnerID, &c.ContactID, &c.AccountID, &c.CreatedAt, &closedAt, &isClosed, &isEscalated, &jiraIssueKey)
	if err != nil {
		return nil, err
	}
	c.Product = product.String
	c.CaseType = caseType.String
	c.Origin = origin.String
	c.Reason = reason.String
	c.ClosedAt = closedAt.String
	c.IsClosed = isClosed == 1
	c.IsEscalated = isEscalated == 1
	c.JiraIssueKey = jiraIssueKey.String
	return c, nil
}

// QueryCases retrieves all cases.
func (s *Store) QueryCases() ([]Case, error) {
	rows, err := s.db.Query(`SELECT id, case_number, subject, description, status, priority, product, case_type, origin, reason, owner_id, contact_id, account_id, created_at, closed_at, is_closed, is_escalated, jira_issue_key FROM cases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []Case
	for rows.Next() {
		var c Case
		var product, caseType, origin, reason, closedAt, jiraIssueKey sql.NullString
		var isClosed, isEscalated int
		if err := rows.Scan(&c.ID, &c.CaseNumber, &c.Subject, &c.Description, &c.Status, &c.Priority, &product, &caseType, &origin, &reason, &c.OwnerID, &c.ContactID, &c.AccountID, &c.CreatedAt, &closedAt, &isClosed, &isEscalated, &jiraIssueKey); err != nil {
			return nil, err
		}
		c.Product = product.String
		c.CaseType = caseType.String
		c.Origin = origin.String
		c.Reason = reason.String
		c.ClosedAt = closedAt.String
		c.IsClosed = isClosed == 1
		c.IsEscalated = isEscalated == 1
		c.JiraIssueKey = jiraIssueKey.String
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

// --- Email Operations ---

// InsertEmail inserts a single email.
func (s *Store) InsertEmail(e *Email) error {
	_, err := s.db.Exec(`INSERT INTO email_messages (id, case_id, subject, text_body, html_body, from_address, from_name, to_address, cc_address, bcc_address, message_date, status, incoming, has_attachment, headers, sequence_num)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.CaseID, e.Subject, e.TextBody, e.HTMLBody, e.FromAddress, e.FromName, e.ToAddress, e.CCAddress, e.BCCAddress, e.MessageDate, e.Status, boolToInt(e.Incoming), boolToInt(e.HasAttachment), e.Headers, e.SequenceNum)
	return err
}

// InsertEmailsBatch inserts multiple emails in a transaction.
func (s *Store) InsertEmailsBatch(tx *sql.Tx, emails []Email) error {
	stmt, err := tx.Prepare(`INSERT INTO email_messages (id, case_id, subject, text_body, html_body, from_address, from_name, to_address, cc_address, bcc_address, message_date, status, incoming, has_attachment, headers, sequence_num)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare email insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range emails {
		if _, err := stmt.Exec(e.ID, e.CaseID, e.Subject, e.TextBody, e.HTMLBody, e.FromAddress, e.FromName, e.ToAddress, e.CCAddress, e.BCCAddress, e.MessageDate, e.Status, boolToInt(e.Incoming), boolToInt(e.HasAttachment), e.Headers, e.SequenceNum); err != nil {
			return fmt.Errorf("insert email %s: %w", e.ID, err)
		}
	}
	return nil
}

// QueryEmails retrieves all emails.
func (s *Store) QueryEmails() ([]Email, error) {
	rows, err := s.db.Query(`SELECT id, case_id, subject, text_body, html_body, from_address, from_name, to_address, cc_address, bcc_address, message_date, status, incoming, has_attachment, headers, sequence_num FROM email_messages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []Email
	for rows.Next() {
		var e Email
		var htmlBody, ccAddress, bccAddress, headers sql.NullString
		var incoming, hasAttachment int
		if err := rows.Scan(&e.ID, &e.CaseID, &e.Subject, &e.TextBody, &htmlBody, &e.FromAddress, &e.FromName, &e.ToAddress, &ccAddress, &bccAddress, &e.MessageDate, &e.Status, &incoming, &hasAttachment, &headers, &e.SequenceNum); err != nil {
			return nil, err
		}
		e.HTMLBody = htmlBody.String
		e.CCAddress = ccAddress.String
		e.BCCAddress = bccAddress.String
		e.Headers = headers.String
		e.Incoming = incoming == 1
		e.HasAttachment = hasAttachment == 1
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// --- Comment Operations ---

// InsertComment inserts a single comment.
func (s *Store) InsertComment(c *Comment) error {
	_, err := s.db.Exec(`INSERT INTO case_comments (id, case_id, comment_body, created_by_id, created_at, is_published)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.CaseID, c.CommentBody, c.CreatedByID, c.CreatedAt, boolToInt(c.IsPublished))
	return err
}

// InsertCommentsBatch inserts multiple comments in a transaction.
func (s *Store) InsertCommentsBatch(tx *sql.Tx, comments []Comment) error {
	stmt, err := tx.Prepare(`INSERT INTO case_comments (id, case_id, comment_body, created_by_id, created_at, is_published)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare comment insert: %w", err)
	}
	defer stmt.Close()

	for _, c := range comments {
		if _, err := stmt.Exec(c.ID, c.CaseID, c.CommentBody, c.CreatedByID, c.CreatedAt, boolToInt(c.IsPublished)); err != nil {
			return fmt.Errorf("insert comment %s: %w", c.ID, err)
		}
	}
	return nil
}

// QueryComments retrieves all comments.
func (s *Store) QueryComments() ([]Comment, error) {
	rows, err := s.db.Query(`SELECT id, case_id, comment_body, created_by_id, created_at, is_published FROM case_comments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		var isPublished int
		if err := rows.Scan(&c.ID, &c.CaseID, &c.CommentBody, &c.CreatedByID, &c.CreatedAt, &isPublished); err != nil {
			return nil, err
		}
		c.IsPublished = isPublished == 1
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// --- FeedItem Operations ---

// InsertFeedItem inserts a single feed item.
func (s *Store) InsertFeedItem(f *FeedItem) error {
	_, err := s.db.Exec(`INSERT INTO feed_items (id, case_id, body, type, created_by_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		f.ID, f.CaseID, f.Body, f.Type, f.CreatedByID, f.CreatedAt)
	return err
}

// InsertFeedItemsBatch inserts multiple feed items in a transaction.
func (s *Store) InsertFeedItemsBatch(tx *sql.Tx, items []FeedItem) error {
	stmt, err := tx.Prepare(`INSERT INTO feed_items (id, case_id, body, type, created_by_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare feed item insert: %w", err)
	}
	defer stmt.Close()

	for _, f := range items {
		if _, err := stmt.Exec(f.ID, f.CaseID, f.Body, f.Type, f.CreatedByID, f.CreatedAt); err != nil {
			return fmt.Errorf("insert feed item %s: %w", f.ID, err)
		}
	}
	return nil
}

// QueryFeedItems retrieves all feed items.
func (s *Store) QueryFeedItems() ([]FeedItem, error) {
	rows, err := s.db.Query(`SELECT id, case_id, body, type, created_by_id, created_at FROM feed_items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []FeedItem
	for rows.Next() {
		var f FeedItem
		if err := rows.Scan(&f.ID, &f.CaseID, &f.Body, &f.Type, &f.CreatedByID, &f.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	return items, rows.Err()
}

// --- JiraIssue Operations ---

// InsertJiraIssue inserts a single Jira issue.
func (s *Store) InsertJiraIssue(j *JiraIssue) error {
	_, err := s.db.Exec(`INSERT INTO jira_issues (id, key, project_key, summary, description_adf, issue_type, status, priority, assignee_id, reporter_id, created_at, updated_at, labels, sf_case_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Key, j.ProjectKey, j.Summary, j.DescriptionADF, j.IssueType, j.Status, j.Priority,
		nullableString(j.AssigneeID), nullableString(j.ReporterID), j.CreatedAt, j.UpdatedAt,
		nullableString(j.Labels), nullableString(j.SFCaseID))
	return err
}

// InsertJiraIssuesBatch inserts multiple Jira issues in a transaction.
func (s *Store) InsertJiraIssuesBatch(tx *sql.Tx, issues []JiraIssue) error {
	stmt, err := tx.Prepare(`INSERT INTO jira_issues (id, key, project_key, summary, description_adf, issue_type, status, priority, assignee_id, reporter_id, created_at, updated_at, labels, sf_case_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare jira issue insert: %w", err)
	}
	defer stmt.Close()

	for _, j := range issues {
		if _, err := stmt.Exec(j.ID, j.Key, j.ProjectKey, j.Summary, j.DescriptionADF, j.IssueType, j.Status, j.Priority,
			nullableString(j.AssigneeID), nullableString(j.ReporterID), j.CreatedAt, j.UpdatedAt,
			nullableString(j.Labels), nullableString(j.SFCaseID)); err != nil {
			return fmt.Errorf("insert jira issue %s: %w", j.ID, err)
		}
	}
	return nil
}

// QueryJiraIssues retrieves all Jira issues.
func (s *Store) QueryJiraIssues() ([]JiraIssue, error) {
	rows, err := s.db.Query(`SELECT id, key, project_key, summary, description_adf, issue_type, status, priority, assignee_id, reporter_id, created_at, updated_at, labels, sf_case_id FROM jira_issues`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []JiraIssue
	for rows.Next() {
		var j JiraIssue
		var assigneeID, reporterID, labels, sfCaseID sql.NullString
		if err := rows.Scan(&j.ID, &j.Key, &j.ProjectKey, &j.Summary, &j.DescriptionADF, &j.IssueType, &j.Status, &j.Priority, &assigneeID, &reporterID, &j.CreatedAt, &j.UpdatedAt, &labels, &sfCaseID); err != nil {
			return nil, err
		}
		j.AssigneeID = assigneeID.String
		j.ReporterID = reporterID.String
		j.Labels = labels.String
		j.SFCaseID = sfCaseID.String
		issues = append(issues, j)
	}
	return issues, rows.Err()
}

// --- JiraComment Operations ---

// InsertJiraComment inserts a single Jira comment.
func (s *Store) InsertJiraComment(c *JiraComment) error {
	_, err := s.db.Exec(`INSERT INTO jira_comments (id, issue_id, author_id, body_adf, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.IssueID, c.AuthorID, c.BodyADF, c.CreatedAt, c.UpdatedAt)
	return err
}

// InsertJiraCommentsBatch inserts multiple Jira comments in a transaction.
func (s *Store) InsertJiraCommentsBatch(tx *sql.Tx, comments []JiraComment) error {
	stmt, err := tx.Prepare(`INSERT INTO jira_comments (id, issue_id, author_id, body_adf, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare jira comment insert: %w", err)
	}
	defer stmt.Close()

	for _, c := range comments {
		if _, err := stmt.Exec(c.ID, c.IssueID, c.AuthorID, c.BodyADF, c.CreatedAt, c.UpdatedAt); err != nil {
			return fmt.Errorf("insert jira comment %s: %w", c.ID, err)
		}
	}
	return nil
}

// QueryJiraComments retrieves all Jira comments.
func (s *Store) QueryJiraComments() ([]JiraComment, error) {
	rows, err := s.db.Query(`SELECT id, issue_id, author_id, body_adf, created_at, updated_at FROM jira_comments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []JiraComment
	for rows.Next() {
		var c JiraComment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.AuthorID, &c.BodyADF, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// --- JiraUser Operations ---

// InsertJiraUser inserts a single Jira user.
func (s *Store) InsertJiraUser(u *JiraUser) error {
	_, err := s.db.Exec(`INSERT INTO jira_users (account_id, display_name, email, account_type, active, sf_user_id)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.AccountID, u.DisplayName, nullableString(u.Email), u.AccountType, boolToInt(u.Active), nullableString(u.SFUserID))
	return err
}

// InsertJiraUsersBatch inserts multiple Jira users in a transaction.
func (s *Store) InsertJiraUsersBatch(tx *sql.Tx, users []JiraUser) error {
	stmt, err := tx.Prepare(`INSERT INTO jira_users (account_id, display_name, email, account_type, active, sf_user_id)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare jira user insert: %w", err)
	}
	defer stmt.Close()

	for _, u := range users {
		if _, err := stmt.Exec(u.AccountID, u.DisplayName, nullableString(u.Email), u.AccountType, boolToInt(u.Active), nullableString(u.SFUserID)); err != nil {
			return fmt.Errorf("insert jira user %s: %w", u.AccountID, err)
		}
	}
	return nil
}

// QueryJiraUsers retrieves all Jira users.
func (s *Store) QueryJiraUsers() ([]JiraUser, error) {
	rows, err := s.db.Query(`SELECT account_id, display_name, email, account_type, active, sf_user_id FROM jira_users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []JiraUser
	for rows.Next() {
		var u JiraUser
		var email, sfUserID sql.NullString
		var active int
		if err := rows.Scan(&u.AccountID, &u.DisplayName, &email, &u.AccountType, &active, &sfUserID); err != nil {
			return nil, err
		}
		u.Email = email.String
		u.Active = active == 1
		u.SFUserID = sfUserID.String
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- ProfileImage Operations ---

// InsertProfileImage inserts a single profile image record.
func (s *Store) InsertProfileImage(p *ProfileImage) error {
	_, err := s.db.Exec(`INSERT INTO profile_images (id, persona_type, persona_id, image_path, first_name, last_name, age, gender, ethnicity, hair_color, hair_style, eye_color, glasses, facial_hair, generated_at, prompt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.PersonaType, p.PersonaID, p.ImagePath,
		nullableString(p.FirstName), nullableString(p.LastName), nullableInt(p.Age),
		nullableString(p.Gender), nullableString(p.Ethnicity), nullableString(p.HairColor),
		nullableString(p.HairStyle), nullableString(p.EyeColor), boolToInt(p.Glasses),
		nullableString(p.FacialHair), nullableString(p.GeneratedAt), nullableString(p.Prompt))
	return err
}

// InsertProfileImagesBatch inserts multiple profile images in a transaction.
func (s *Store) InsertProfileImagesBatch(tx *sql.Tx, images []ProfileImage) error {
	stmt, err := tx.Prepare(`INSERT INTO profile_images (id, persona_type, persona_id, image_path, first_name, last_name, age, gender, ethnicity, hair_color, hair_style, eye_color, glasses, facial_hair, generated_at, prompt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare profile image insert: %w", err)
	}
	defer stmt.Close()

	for _, p := range images {
		if _, err := stmt.Exec(p.ID, p.PersonaType, p.PersonaID, p.ImagePath,
			nullableString(p.FirstName), nullableString(p.LastName), nullableInt(p.Age),
			nullableString(p.Gender), nullableString(p.Ethnicity), nullableString(p.HairColor),
			nullableString(p.HairStyle), nullableString(p.EyeColor), boolToInt(p.Glasses),
			nullableString(p.FacialHair), nullableString(p.GeneratedAt), nullableString(p.Prompt)); err != nil {
			return fmt.Errorf("insert profile image %s: %w", p.ID, err)
		}
	}
	return nil
}

// GetProfileImageByID retrieves a profile image by ID.
func (s *Store) GetProfileImageByID(id string) (*ProfileImage, error) {
	row := s.db.QueryRow(`SELECT id, persona_type, persona_id, image_path, first_name, last_name, age, gender, ethnicity, hair_color, hair_style, eye_color, glasses, facial_hair, generated_at, prompt FROM profile_images WHERE id = ?`, id)
	return scanProfileImage(row)
}

// QueryProfileImageByPersona retrieves a profile image by persona type and ID.
func (s *Store) QueryProfileImageByPersona(personaType, personaID string) (*ProfileImage, error) {
	row := s.db.QueryRow(`SELECT id, persona_type, persona_id, image_path, first_name, last_name, age, gender, ethnicity, hair_color, hair_style, eye_color, glasses, facial_hair, generated_at, prompt FROM profile_images WHERE persona_type = ? AND persona_id = ?`, personaType, personaID)
	return scanProfileImage(row)
}

// QueryAllProfileImages retrieves all profile images.
func (s *Store) QueryAllProfileImages() ([]ProfileImage, error) {
	rows, err := s.db.Query(`SELECT id, persona_type, persona_id, image_path, first_name, last_name, age, gender, ethnicity, hair_color, hair_style, eye_color, glasses, facial_hair, generated_at, prompt FROM profile_images`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []ProfileImage
	for rows.Next() {
		var p ProfileImage
		var firstName, lastName, gender, ethnicity, hairColor, hairStyle, eyeColor, facialHair, generatedAt, prompt sql.NullString
		var age sql.NullInt64
		var glasses int
		if err := rows.Scan(&p.ID, &p.PersonaType, &p.PersonaID, &p.ImagePath, &firstName, &lastName, &age, &gender, &ethnicity, &hairColor, &hairStyle, &eyeColor, &glasses, &facialHair, &generatedAt, &prompt); err != nil {
			return nil, err
		}
		p.FirstName = firstName.String
		p.LastName = lastName.String
		p.Age = int(age.Int64)
		p.Gender = gender.String
		p.Ethnicity = ethnicity.String
		p.HairColor = hairColor.String
		p.HairStyle = hairStyle.String
		p.EyeColor = eyeColor.String
		p.Glasses = glasses == 1
		p.FacialHair = facialHair.String
		p.GeneratedAt = generatedAt.String
		p.Prompt = prompt.String
		images = append(images, p)
	}
	return images, rows.Err()
}

// scanProfileImage scans a single row into a ProfileImage.
func scanProfileImage(row *sql.Row) (*ProfileImage, error) {
	p := &ProfileImage{}
	var firstName, lastName, gender, ethnicity, hairColor, hairStyle, eyeColor, facialHair, generatedAt, prompt sql.NullString
	var age sql.NullInt64
	var glasses int
	err := row.Scan(&p.ID, &p.PersonaType, &p.PersonaID, &p.ImagePath, &firstName, &lastName, &age, &gender, &ethnicity, &hairColor, &hairStyle, &eyeColor, &glasses, &facialHair, &generatedAt, &prompt)
	if err != nil {
		return nil, err
	}
	p.FirstName = firstName.String
	p.LastName = lastName.String
	p.Age = int(age.Int64)
	p.Gender = gender.String
	p.Ethnicity = ethnicity.String
	p.HairColor = hairColor.String
	p.HairStyle = hairStyle.String
	p.EyeColor = eyeColor.String
	p.Glasses = glasses == 1
	p.FacialHair = facialHair.String
	p.GeneratedAt = generatedAt.String
	p.Prompt = prompt.String
	return p, nil
}

// nullableInt returns nil for zero values (for NULL in SQLite).
func nullableInt(i int) any {
	if i == 0 {
		return nil
	}
	return i
}

// DeleteAllProfileImages removes all profile images from the database.
func (s *Store) DeleteAllProfileImages() error {
	_, err := s.db.Exec(`DELETE FROM profile_images`)
	return err
}


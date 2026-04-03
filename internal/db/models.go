// Package db provides SQLite storage for generated mock data.
package db

// Account represents a Salesforce Account record.
type Account struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Industry      string  `json:"industry"`
	Type          string  `json:"type"`
	Website       string  `json:"website,omitempty"`
	Phone         string  `json:"phone,omitempty"`
	BillingCity   string  `json:"billing_city,omitempty"`
	BillingState  string  `json:"billing_state,omitempty"`
	AnnualRevenue float64 `json:"annual_revenue,omitempty"`
	NumEmployees  int     `json:"num_employees,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// Contact represents a Salesforce Contact record.
type Contact struct {
	ID         string `json:"id"`
	AccountID  string `json:"account_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Phone      string `json:"phone,omitempty"`
	Title      string `json:"title,omitempty"`
	Department string `json:"department,omitempty"`
	IsPrimary  bool   `json:"is_primary"`
	CreatedAt  string `json:"created_at"`
}

// User represents a Salesforce User record.
type User struct {
	ID         string `json:"id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Title      string `json:"title,omitempty"`
	Department string `json:"department,omitempty"`
	IsActive   bool   `json:"is_active"`
	ManagerID  string `json:"manager_id,omitempty"`
	UserRole   string `json:"user_role,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Case represents a Salesforce Case record.
type Case struct {
	ID           string `json:"id"`
	CaseNumber   string `json:"case_number"`
	Subject      string `json:"subject"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Priority     string `json:"priority"`
	Product      string `json:"product,omitempty"`
	CaseType     string `json:"case_type,omitempty"`
	Origin       string `json:"origin,omitempty"`
	Reason       string `json:"reason,omitempty"`
	OwnerID      string `json:"owner_id"`
	ContactID    string `json:"contact_id"`
	AccountID    string `json:"account_id"`
	CreatedAt    string `json:"created_at"`
	ClosedAt     string `json:"closed_at,omitempty"`
	IsClosed     bool   `json:"is_closed"`
	IsEscalated  bool   `json:"is_escalated"`
	JiraIssueKey string `json:"jira_issue_key,omitempty"`
}

// Email represents an email message associated with a case.
type Email struct {
	ID            string `json:"id"`
	CaseID        string `json:"case_id"`
	Subject       string `json:"subject"`
	TextBody      string `json:"text_body"`
	HTMLBody      string `json:"html_body,omitempty"`
	FromAddress   string `json:"from_address"`
	FromName      string `json:"from_name"`
	ToAddress     string `json:"to_address"`
	CCAddress     string `json:"cc_address,omitempty"`
	BCCAddress    string `json:"bcc_address,omitempty"`
	MessageDate   string `json:"message_date"`
	Status        string `json:"status"`
	Incoming      bool   `json:"incoming"`
	HasAttachment bool   `json:"has_attachment"`
	Headers       string `json:"headers,omitempty"`
	SequenceNum   int    `json:"sequence_num"`
}

// Comment represents a case comment.
type Comment struct {
	ID          string `json:"id"`
	CaseID      string `json:"case_id"`
	CommentBody string `json:"comment_body"`
	CreatedByID string `json:"created_by_id"`
	CreatedAt   string `json:"created_at"`
	IsPublished bool   `json:"is_published"`
}

// FeedItem represents a Chatter feed item.
type FeedItem struct {
	ID          string `json:"id"`
	CaseID      string `json:"case_id"`
	Body        string `json:"body"`
	Type        string `json:"type"`
	CreatedByID string `json:"created_by_id"`
	CreatedAt   string `json:"created_at"`
}

// JiraIssue represents a Jira issue.
type JiraIssue struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	ProjectKey     string `json:"project_key"`
	Summary        string `json:"summary"`
	DescriptionADF string `json:"description_adf"`
	IssueType      string `json:"issue_type"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	AssigneeID     string `json:"assignee_id,omitempty"`
	ReporterID     string `json:"reporter_id,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	Labels         string `json:"labels,omitempty"`
	SFCaseID       string `json:"sf_case_id,omitempty"`
}

// JiraComment represents a Jira issue comment.
type JiraComment struct {
	ID        string `json:"id"`
	IssueID   string `json:"issue_id"`
	AuthorID  string `json:"author_id"`
	BodyADF   string `json:"body_adf"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// JiraUser represents a Jira user.
type JiraUser struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	AccountType string `json:"account_type"`
	Active      bool   `json:"active"`
	SFUserID    string `json:"sf_user_id,omitempty"`
}

// ProfileImage represents a generated profile picture for a persona.
type ProfileImage struct {
	ID          string `json:"id"`
	PersonaType string `json:"persona_type"` // "contact" or "user"
	PersonaID   string `json:"persona_id"`   // FK to contacts.id or users.id
	ImagePath   string `json:"image_path"`   // Relative path: assets/profile_images/uuid.png
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	Age         int    `json:"age,omitempty"`
	Gender      string `json:"gender,omitempty"`
	Ethnicity   string `json:"ethnicity,omitempty"`
	HairColor   string `json:"hair_color,omitempty"`
	HairStyle   string `json:"hair_style,omitempty"`
	EyeColor    string `json:"eye_color,omitempty"`
	Glasses     bool   `json:"glasses"`
	FacialHair  string `json:"facial_hair,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
	Prompt      string `json:"prompt,omitempty"` // Full CogView-4 prompt used
}


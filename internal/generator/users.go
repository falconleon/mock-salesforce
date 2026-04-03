package generator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/falconleon/mock-salesforce/internal/generator/seed"
	"github.com/rs/zerolog"
)

// UserGenerator generates support agent User records and corresponding Jira users.
type UserGenerator struct {
	db     *sql.DB
	llm    LLM
	logger zerolog.Logger
}

// NewUserGenerator creates a new user generator.
func NewUserGenerator(ctx *Context) *UserGenerator {
	return &UserGenerator{
		db:     ctx.DB,
		llm:    ctx.LLM,
		logger: ctx.Logger.With().Str("generator", "users").Logger(),
	}
}

// UserConfig configures user generation.
type UserConfig struct {
	// Count is the total number of users to generate.
	Count int
	// RoleDistribution specifies the percentage of each role.
	// Keys: "agent" (L1/L2 frontline), "engineer" (L3 technical escalation), "manager"
	RoleDistribution map[string]float64
}

// DefaultUserConfig returns sensible defaults for role distribution.
func DefaultUserConfig(count int) UserConfig {
	return UserConfig{
		Count: count,
		RoleDistribution: map[string]float64{
			"agent":    0.60, // 60% frontline support (L1/L2)
			"engineer": 0.25, // 25% technical escalation (L3)
			"manager":  0.15, // 15% oversight
		},
	}
}

// User represents a generated support agent.
type User struct {
	ID         string `json:"id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Title      string `json:"title"`
	Department string `json:"department"`
	IsActive   bool   `json:"is_active"`
	ManagerID  string `json:"manager_id,omitempty"`
	UserRole   string `json:"user_role"`
	CreatedAt  string `json:"created_at"`
}

// JiraUser represents a Jira user linked to a Salesforce user.
type JiraUser struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AccountType string `json:"account_type"`
	Active      bool   `json:"active"`
	SFUserID    string `json:"sf_user_id"`
}

// GeneratedUsers holds the result of user generation.
type GeneratedUsers struct {
	Users     []User
	JiraUsers []JiraUser
}

// Generate creates support team users with the default configuration.
func (g *UserGenerator) Generate(count int) error {
	return g.GenerateWithConfig(DefaultUserConfig(count))
}

// GenerateWithConfig creates support team users with custom configuration.
func (g *UserGenerator) GenerateWithConfig(cfg UserConfig) error {
	g.logger.Info().Int("count", cfg.Count).Msg("Generating users with hierarchy")

	// Calculate role counts from distribution
	managerCount := max(1, int(float64(cfg.Count)*cfg.RoleDistribution["manager"]))
	engineerCount := max(1, int(float64(cfg.Count)*cfg.RoleDistribution["engineer"]))
	agentCount := cfg.Count - managerCount - engineerCount
	if agentCount < 0 {
		agentCount = 0
	}

	// Split agents into L1 and L2 (roughly 60/40)
	l1Count := (agentCount * 6) / 10
	l2Count := agentCount - l1Count

	g.logger.Debug().
		Int("managers", managerCount).
		Int("engineers", engineerCount).
		Int("l1_agents", l1Count).
		Int("l2_agents", l2Count).
		Msg("Role distribution calculated")

	// Generate users via LLM
	users, err := g.generateUsersViaLLM(managerCount, engineerCount, l1Count, l2Count)
	if err != nil {
		return err
	}

	// Assign Salesforce IDs and build manager hierarchy
	managerIDs := g.assignIDsAndHierarchy(users, managerCount)

	// Create corresponding Jira users
	jiraUsers := g.createJiraUsers(users)

	// Insert into database
	if err := g.insertUsers(users, jiraUsers, managerIDs); err != nil {
		return err
	}

	g.logger.Info().
		Int("users", len(users)).
		Int("jira_users", len(jiraUsers)).
		Msg("Users and Jira users generated")

	return nil
}

// userRoleSpec defines the role and count for batch generation.
type userRoleSpec struct {
	role      seed.RoleLevel
	userRole  string // User.UserRole field value
	count     int
	hireYear  string // Base year for hire date
}

// generateUsersViaLLM uses the LLM to generate realistic user profiles.
func (g *UserGenerator) generateUsersViaLLM(managers, engineers, l1, l2 int) ([]User, error) {
	// Define role specs with appropriate RoleLevel for age-appropriate seeds
	specs := []userRoleSpec{
		{seed.RoleManager, "Manager", managers, "2022"},
		{seed.RoleSupportL3, "L3 Support", engineers, "2022"},
		{seed.RoleSupportL2, "L2 Support", l2, "2023"},
		{seed.RoleSupportL1, "L1 Support", l1, "2023"},
	}

	var users []User
	for _, spec := range specs {
		for i := 0; i < spec.count; i++ {
			personSeed := seed.NewPersonSeed(spec.role)

			prompt := fmt.Sprintf(`Generate a support team member for a B2B SaaS company.

Person details:
%s

Role: %s
Department: Customer Support

Generate an appropriate job title for this role and a hire date in %s.

The person's name is %s %s.
Email: %s.%s@acme.com
Username: %s%s@acme.com

Return ONLY a JSON object with fields: title, created_at

Example:
{"title":"Senior Support Manager","created_at":"2022-01-15T09:00:00Z"}`,
				personSeed.ToPromptFragment(),
				spec.userRole,
				spec.hireYear,
				personSeed.FirstName, personSeed.LastName,
				strings.ToLower(personSeed.FirstName), strings.ToLower(personSeed.LastName),
				strings.ToLower(string(personSeed.FirstName[0])), strings.ToLower(personSeed.LastName))

			resp, err := g.llm.Generate(prompt)
			if err != nil {
				g.logger.Error().Err(err).Str("role", spec.userRole).Msg("Failed to generate user")
				continue
			}

			// Extract JSON from response (handle potential markdown wrapping)
			jsonStr := extractJSON(resp)

			var userData struct {
				Title     string `json:"title"`
				CreatedAt string `json:"created_at"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &userData); err != nil {
				g.logger.Error().Str("response", resp).Msg("Failed to parse LLM response")
				continue
			}

			user := User{
				FirstName:  personSeed.FirstName,
				LastName:   personSeed.LastName,
				Email:      fmt.Sprintf("%s.%s@acme.com", strings.ToLower(personSeed.FirstName), strings.ToLower(personSeed.LastName)),
				Username:   fmt.Sprintf("%s%s@acme.com", strings.ToLower(string(personSeed.FirstName[0])), strings.ToLower(personSeed.LastName)),
				Title:      userData.Title,
				Department: "Customer Support",
				IsActive:   true,
				UserRole:   spec.userRole,
				CreatedAt:  userData.CreatedAt,
			}
			users = append(users, user)
			g.logger.Debug().
				Str("name", user.FirstName+" "+user.LastName).
				Str("role", spec.userRole).
				Msg("Generated user")
		}
	}

	return users, nil
}

// assignIDsAndHierarchy assigns Salesforce IDs and sets up manager relationships.
func (g *UserGenerator) assignIDsAndHierarchy(users []User, managerCount int) []string {
	var managerIDs []string

	// First pass: assign IDs and collect manager IDs
	for i := range users {
		users[i].ID = SalesforceID("User")
		if users[i].UserRole == "Manager" || strings.Contains(strings.ToLower(users[i].Title), "manager") {
			managerIDs = append(managerIDs, users[i].ID)
		}
	}

	// Second pass: assign managers to non-managers
	if len(managerIDs) > 0 {
		managerIdx := 0
		for i := range users {
			if users[i].UserRole != "Manager" && !strings.Contains(strings.ToLower(users[i].Title), "manager") {
				users[i].ManagerID = managerIDs[managerIdx%len(managerIDs)]
				managerIdx++
			}
		}
	}

	return managerIDs
}

// createJiraUsers creates Jira user records linked to Salesforce users.
func (g *UserGenerator) createJiraUsers(users []User) []JiraUser {
	jiraUsers := make([]JiraUser, len(users))
	for i, u := range users {
		jiraUsers[i] = JiraUser{
			AccountID:   JiraAccountID(), // 24-char hex
			DisplayName: u.FirstName + " " + u.LastName,
			Email:       u.Email,
			AccountType: "atlassian",
			Active:      u.IsActive,
			SFUserID:    u.ID,
		}
	}
	return jiraUsers
}

// insertUsers inserts users and Jira users into the database.
func (g *UserGenerator) insertUsers(users []User, jiraUsers []JiraUser, managerIDs []string) error {
	tx, err := g.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert managers first (they have no manager_id dependencies)
	userStmt, err := tx.Prepare(`INSERT INTO users
		(id, first_name, last_name, email, username, title, department, is_active, manager_id, user_role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare user statement: %w", err)
	}
	defer userStmt.Close()

	// Insert managers first
	for _, u := range users {
		if u.ManagerID == "" {
			if err := g.insertUser(userStmt, u); err != nil {
				return err
			}
		}
	}

	// Insert non-managers
	for _, u := range users {
		if u.ManagerID != "" {
			if err := g.insertUser(userStmt, u); err != nil {
				return err
			}
		}
	}

	// Insert Jira users
	jiraStmt, err := tx.Prepare(`INSERT INTO jira_users
		(account_id, display_name, email, account_type, active, sf_user_id)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare jira user statement: %w", err)
	}
	defer jiraStmt.Close()

	for _, ju := range jiraUsers {
		active := 0
		if ju.Active {
			active = 1
		}
		if _, err := jiraStmt.Exec(ju.AccountID, ju.DisplayName, ju.Email, ju.AccountType, active, ju.SFUserID); err != nil {
			return fmt.Errorf("insert jira user %s: %w", ju.DisplayName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// insertUser inserts a single user record.
func (g *UserGenerator) insertUser(stmt *sql.Stmt, u User) error {
	active := 0
	if u.IsActive {
		active = 1
	}
	var managerID interface{}
	if u.ManagerID != "" {
		managerID = u.ManagerID
	}
	_, err := stmt.Exec(
		u.ID, u.FirstName, u.LastName, u.Email, u.Username,
		u.Title, u.Department, active, managerID, u.UserRole, u.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user %s: %w", u.Email, err)
	}
	return nil
}

// extractJSON extracts JSON from a response that may contain markdown or other text.
// It handles both arrays [...] and objects {...}.
func extractJSON(s string) string {
	// Try array first
	startArr := strings.Index(s, "[")
	endArr := strings.LastIndex(s, "]")
	if startArr >= 0 && endArr > startArr {
		return s[startArr : endArr+1]
	}

	// Try object
	startObj := strings.Index(s, "{")
	endObj := strings.LastIndex(s, "}")
	if startObj >= 0 && endObj > startObj {
		return s[startObj : endObj+1]
	}

	return s
}

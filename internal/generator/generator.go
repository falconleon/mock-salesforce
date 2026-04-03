// Package generator provides per-object-type data generators.
//
// Each generator populates its corresponding SQLite table(s) using
// an LLM to produce realistic content. Generators are run in phases
// to respect referential dependencies:
//
//	Phase 1: accounts, contacts, users, jira_users
//	Phase 2: cases
//	Phase 3: email_messages, case_comments, feed_items
//	Phase 4: jira_issues, jira_comments
package generator

import (
	"database/sql"

	"github.com/rs/zerolog"
)

// LLM is the interface for generating content via an LLM.
// This is implemented by the z.ai endpoint module.
type LLM interface {
	// Generate sends a prompt and returns the LLM response text.
	Generate(prompt string) (string, error)
}

// Context holds shared dependencies for all generators.
type Context struct {
	DB     *sql.DB
	LLM    LLM
	Logger zerolog.Logger
}

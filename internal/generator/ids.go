package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

// Salesforce ID prefixes by object type.
var sfPrefixes = map[string]string{
	"Account":      "001",
	"Contact":      "003",
	"User":         "005",
	"Case":         "500",
	"EmailMessage": "02s",
	"CaseComment":  "00a",
	"FeedItem":     "0D5",
}

// SalesforceID generates an 18-character Salesforce-style ID.
func SalesforceID(objectType string) string {
	prefix := sfPrefixes[objectType]
	if prefix == "" {
		prefix = "000"
	}

	suffix := make([]byte, 8)
	rand.Read(suffix)
	hex := hex.EncodeToString(suffix)

	// 18 chars total: 3-char prefix + 12-char random + 3-char suffix
	return prefix + hex[:12] + "AAV"
}

// JIRA-style sequential ID generation.
var jiraIDCounter atomic.Int64

func init() {
	jiraIDCounter.Store(10000)
}

// JiraID generates a sequential JIRA numeric ID.
func JiraID() string {
	return fmt.Sprintf("%d", jiraIDCounter.Add(1))
}

// JiraAccountID generates a 24-character hex account ID.
func JiraAccountID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CaseNumber generates a sequential case number string.
var caseNumberCounter atomic.Int64

func init() {
	caseNumberCounter.Store(123456)
}

// NextCaseNumber returns the next case number like "00123457".
func NextCaseNumber() string {
	return fmt.Sprintf("%08d", caseNumberCounter.Add(1))
}

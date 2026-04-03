package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/falconleon/mock-salesforce/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// UIHandler serves browsable HTML pages for the mock Salesforce UI.
type UIHandler struct {
	store          store.Store
	basePath       string
	caseListTpl    *template.Template
	caseTpl        *template.Template
	accountTpl     *template.Template
	accountListTpl *template.Template
}

// NewUIHandler creates a UIHandler with parsed templates and template functions.
func NewUIHandler(s store.Store, basePath string) *UIHandler {
	funcMap := template.FuncMap{
		"statusClass": func(s string) string {
			switch s {
			case "Closed", "Resolved":
				return "badge-green"
			case "In Progress", "Working":
				return "badge-blue"
			case "Escalated":
				return "badge-red"
			case "On Hold":
				return "badge-yellow"
			default:
				return "badge-grey"
			}
		},
		"priorityClass": func(s string) string {
			switch s {
			case "P0":
				return "badge-red"
			case "P1":
				return "badge-orange"
			case "P2":
				return "badge-yellow"
			default:
				return "badge-grey"
			}
		},
		"fieldStr": func(r store.Record, key string) string {
			if v, ok := r[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		},
		"basePath": func() string { return basePath },
	}
	// Parse each page template separately with layout to avoid {{define "content"}} collisions.
	caseListTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/case_list.html"))
	caseTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/case.html"))
	accountTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/account.html"))
	accountListTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/account_list.html"))
	return &UIHandler{
		store:          s,
		basePath:       basePath,
		caseListTpl:    caseListTpl,
		caseTpl:        caseTpl,
		accountTpl:     accountTpl,
		accountListTpl: accountListTpl,
	}
}

// CaseList renders the cases list page.
func (h *UIHandler) CaseList(w http.ResponseWriter, r *http.Request) {
	cases, _ := h.store.Query("Case", func(r map[string]any) bool { return true })

	// Sort by CaseNumber
	sort.Slice(cases, func(i, j int) bool {
		a, _ := cases[i]["CaseNumber"].(string)
		b, _ := cases[j]["CaseNumber"].(string)
		return a < b
	})

	// Resolve account and contact names
	for _, c := range cases {
		if accID, ok := c["AccountId"].(string); ok && accID != "" {
			acc, err := h.store.Get("Account", accID)
			if err == nil {
				c["_AccountName"] = acc["Name"]
			}
		}
		if ctcID, ok := c["ContactId"].(string); ok && ctcID != "" {
			ctc, err := h.store.Get("Contact", ctcID)
			if err == nil {
				firstName, _ := ctc["FirstName"].(string)
				lastName, _ := ctc["LastName"].(string)
				c["_ContactName"] = firstName + " " + lastName
			}
		}
	}

	h.caseListTpl.ExecuteTemplate(w, "case_list.html", map[string]any{
		"Cases":    cases,
		"Title":    "Salesforce — Cases",
		"Total":    len(cases),
		"BasePath": h.basePath,
	})
}

// CaseDetail renders the case detail page.
func (h *UIHandler) CaseDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Try by Id first, then by CaseNumber
	caseRec, err := h.store.Get("Case", id)
	if err != nil {
		// Try searching by CaseNumber
		cases, _ := h.store.Query("Case", func(r map[string]any) bool {
			return r["CaseNumber"] == id
		})
		if len(cases) > 0 {
			caseRec = cases[0]
		} else {
			http.Error(w, "Case not found", 404)
			return
		}
	}

	caseID, _ := caseRec["Id"].(string)

	// Get related data
	emails, _ := h.store.GetByIndex("EmailMessage", "ParentId", caseID)
	comments, _ := h.store.GetByIndex("CaseComment", "ParentId", caseID)
	feedItems, _ := h.store.GetByIndex("FeedItem", "ParentId", caseID)

	// Resolve account and contact
	if accID, ok := caseRec["AccountId"].(string); ok && accID != "" {
		acc, err := h.store.Get("Account", accID)
		if err == nil {
			caseRec["_Account"] = acc
		}
	}
	if ctcID, ok := caseRec["ContactId"].(string); ok && ctcID != "" {
		ctc, err := h.store.Get("Contact", ctcID)
		if err == nil {
			caseRec["_Contact"] = ctc
		}
	}

	h.caseTpl.ExecuteTemplate(w, "case.html", map[string]any{
		"Case":      caseRec,
		"Emails":    emails,
		"Comments":  comments,
		"FeedItems": feedItems,
		"Title":     caseRec["CaseNumber"],
		"BasePath":  h.basePath,
	})
}

// AccountList renders the accounts list page.
func (h *UIHandler) AccountList(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.store.Query("Account", func(r map[string]any) bool { return true })

	sort.Slice(accounts, func(i, j int) bool {
		return fmt.Sprint(accounts[i]["Name"]) < fmt.Sprint(accounts[j]["Name"])
	})

	h.accountListTpl.ExecuteTemplate(w, "account_list.html", map[string]any{
		"Accounts": accounts,
		"Title":    "Salesforce — Accounts",
		"Total":    len(accounts),
		"BasePath": h.basePath,
	})
}

// AccountDetail renders the account detail page with related cases.
func (h *UIHandler) AccountDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	account, err := h.store.Get("Account", id)
	if err != nil {
		http.Error(w, "Account not found", 404)
		return
	}

	// Get related cases
	cases, _ := h.store.GetByIndex("Case", "AccountId", id)

	h.accountTpl.ExecuteTemplate(w, "account.html", map[string]any{
		"Account":  account,
		"Cases":    cases,
		"Title":    account["Name"],
		"BasePath": h.basePath,
	})
}

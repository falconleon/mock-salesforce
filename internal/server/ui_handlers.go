package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/falconleon/mock-salesforce/internal/store"
)

//go:embed templates/*.html templates/partials/*.html
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
	homeTpl        *template.Template
	contactTpl     *template.Template
	userTpl        *template.Template
	casePartialTpl *template.Template
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
		"userName": func(userID string) string {
			if userID == "" {
				return ""
			}
			u, err := s.Get("User", userID)
			if err != nil {
				return userID
			}
			if name, ok := u["Name"].(string); ok && name != "" {
				return name
			}
			first, _ := u["FirstName"].(string)
			last, _ := u["LastName"].(string)
			n := first + " " + last
			if n == " " {
				return userID
			}
			return n
		},
		"humanBytes": func(v any) string {
			var n int64
			switch x := v.(type) {
			case int:
				n = int64(x)
			case int64:
				n = x
			case float64:
				n = int64(x)
			default:
				return ""
			}
			const k = 1024
			if n < k {
				return fmt.Sprintf("%d B", n)
			}
			if n < k*k {
				return fmt.Sprintf("%.1f KB", float64(n)/k)
			}
			return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
		},
		"basePath": func() string { return basePath },
	}
	// Parse each page template separately with layout to avoid {{define "content"}} collisions.
	caseListTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/case_list.html"))
	caseTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/case.html"))
	accountTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/account.html"))
	accountListTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/account_list.html"))
	homeTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/home.html"))
	contactTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/contact.html"))
	userTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/user.html"))
	casePartialTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/partials/*.html"))
	return &UIHandler{
		store:          s,
		basePath:       basePath,
		caseListTpl:    caseListTpl,
		caseTpl:        caseTpl,
		accountTpl:     accountTpl,
		accountListTpl: accountListTpl,
		homeTpl:        homeTpl,
		contactTpl:     contactTpl,
		userTpl:        userTpl,
		casePartialTpl: casePartialTpl,
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

// resolveCase looks up a case by Id, falling back to CaseNumber.
func (h *UIHandler) resolveCase(id string) (store.Record, error) {
	caseRec, err := h.store.Get("Case", id)
	if err == nil {
		return caseRec, nil
	}
	cases, _ := h.store.Query("Case", func(r map[string]any) bool {
		return r["CaseNumber"] == id
	})
	if len(cases) > 0 {
		return cases[0], nil
	}
	return nil, err
}

// caseEmails returns email messages for a case, sorted by MessageDate ascending.
func (h *UIHandler) caseEmails(caseID string) []store.Record {
	emails, _ := h.store.GetByIndex("EmailMessage", "ParentId", caseID)
	sort.SliceStable(emails, func(i, j int) bool {
		a, _ := emails[i]["MessageDate"].(string)
		b, _ := emails[j]["MessageDate"].(string)
		return a < b
	})
	return emails
}

// caseComments returns case comments sorted by CreatedDate ascending.
func (h *UIHandler) caseComments(caseID string) []store.Record {
	comments, _ := h.store.GetByIndex("CaseComment", "ParentId", caseID)
	sort.SliceStable(comments, func(i, j int) bool {
		a, _ := comments[i]["CreatedDate"].(string)
		b, _ := comments[j]["CreatedDate"].(string)
		return a < b
	})
	return comments
}

// caseFeedItems returns feed items for a case with their comments attached as _Comments.
func (h *UIHandler) caseFeedItems(caseID string) []store.Record {
	items, _ := h.store.GetByIndex("FeedItem", "ParentId", caseID)
	sort.SliceStable(items, func(i, j int) bool {
		a, _ := items[i]["CreatedDate"].(string)
		b, _ := items[j]["CreatedDate"].(string)
		return a < b
	})
	for _, item := range items {
		fid, _ := item["Id"].(string)
		if fid == "" {
			continue
		}
		fc, _ := h.store.GetByIndex("FeedComment", "FeedItemId", fid)
		sort.SliceStable(fc, func(i, j int) bool {
			a, _ := fc[i]["CreatedDate"].(string)
			b, _ := fc[j]["CreatedDate"].(string)
			return a < b
		})
		if len(fc) > 0 {
			item["_Comments"] = fc
		}
	}
	return items
}

// caseTasks returns tasks linked to the case via WhatId.
func (h *UIHandler) caseTasks(caseID string) []store.Record {
	tasks, _ := h.store.GetByIndex("Task", "WhatId", caseID)
	sort.SliceStable(tasks, func(i, j int) bool {
		a, _ := tasks[i]["ActivityDate"].(string)
		b, _ := tasks[j]["ActivityDate"].(string)
		return a < b
	})
	return tasks
}

// caseEvents returns events linked to the case via WhatId.
func (h *UIHandler) caseEvents(caseID string) []store.Record {
	events, _ := h.store.GetByIndex("Event", "WhatId", caseID)
	sort.SliceStable(events, func(i, j int) bool {
		a, _ := events[i]["ActivityDateTime"].(string)
		b, _ := events[j]["ActivityDateTime"].(string)
		return a < b
	})
	return events
}

// caseFiles returns ContentDocument records linked to the case via ContentDocumentLink.
// ContentDocumentLink is not currently indexed by the memory store, so this falls
// back to a filter scan over the small junction table.
func (h *UIHandler) caseFiles(caseID string) []store.Record {
	links, _ := h.store.Query("ContentDocumentLink", func(r map[string]any) bool {
		return r["LinkedEntityId"] == caseID
	})
	files := make([]store.Record, 0, len(links))
	for _, link := range links {
		docID, _ := link["ContentDocumentId"].(string)
		if docID == "" {
			continue
		}
		if doc, err := h.store.Get("ContentDocument", docID); err == nil {
			files = append(files, doc)
		}
	}
	sort.SliceStable(files, func(i, j int) bool {
		a, _ := files[i]["CreatedDate"].(string)
		b, _ := files[j]["CreatedDate"].(string)
		return a < b
	})
	return files
}

// CaseDetail renders the case detail page.
func (h *UIHandler) CaseDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caseRec, err := h.resolveCase(id)
	if err != nil {
		http.Error(w, "Case not found", 404)
		return
	}

	caseID, _ := caseRec["Id"].(string)

	emails := h.caseEmails(caseID)
	comments := h.caseComments(caseID)
	feedItems := h.caseFeedItems(caseID)
	tasks := h.caseTasks(caseID)
	events := h.caseEvents(caseID)
	files := h.caseFiles(caseID)

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
		"Case":            caseRec,
		"Emails":          emails,
		"Comments":        comments,
		"FeedItems":       feedItems,
		"Tasks":           tasks,
		"Events":          events,
		"Files":           files,
		"ActivitiesCount": len(tasks) + len(events),
		"Title":           caseRec["CaseNumber"],
		"BasePath":        h.basePath,
	})
}

// CaseEmailsPartial renders the Emails tab body for HTMX swaps.
func (h *UIHandler) CaseEmailsPartial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caseRec, err := h.resolveCase(id)
	if err != nil {
		http.Error(w, "Case not found", 404)
		return
	}
	caseID, _ := caseRec["Id"].(string)
	h.casePartialTpl.ExecuteTemplate(w, "case_emails", map[string]any{
		"Emails": h.caseEmails(caseID),
	})
}

// CaseCommentsPartial renders the Comments tab body for HTMX swaps.
func (h *UIHandler) CaseCommentsPartial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caseRec, err := h.resolveCase(id)
	if err != nil {
		http.Error(w, "Case not found", 404)
		return
	}
	caseID, _ := caseRec["Id"].(string)
	h.casePartialTpl.ExecuteTemplate(w, "case_comments", map[string]any{
		"Comments": h.caseComments(caseID),
	})
}

// CaseFeedPartial renders the Feed tab body for HTMX swaps.
func (h *UIHandler) CaseFeedPartial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caseRec, err := h.resolveCase(id)
	if err != nil {
		http.Error(w, "Case not found", 404)
		return
	}
	caseID, _ := caseRec["Id"].(string)
	h.casePartialTpl.ExecuteTemplate(w, "case_feed", map[string]any{
		"FeedItems": h.caseFeedItems(caseID),
	})
}

// CaseActivitiesPartial renders the Activities tab body for HTMX swaps.
func (h *UIHandler) CaseActivitiesPartial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caseRec, err := h.resolveCase(id)
	if err != nil {
		http.Error(w, "Case not found", 404)
		return
	}
	caseID, _ := caseRec["Id"].(string)
	h.casePartialTpl.ExecuteTemplate(w, "case_activities", map[string]any{
		"Tasks":  h.caseTasks(caseID),
		"Events": h.caseEvents(caseID),
	})
}

// CaseFilesPartial renders the Files tab body for HTMX swaps.
func (h *UIHandler) CaseFilesPartial(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	caseRec, err := h.resolveCase(id)
	if err != nil {
		http.Error(w, "Case not found", 404)
		return
	}
	caseID, _ := caseRec["Id"].(string)
	h.casePartialTpl.ExecuteTemplate(w, "case_files", map[string]any{
		"Files": h.caseFiles(caseID),
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

// AccountDetail renders the account detail page with related cases and contacts.
func (h *UIHandler) AccountDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	account, err := h.store.Get("Account", id)
	if err != nil {
		http.Error(w, "Account not found", 404)
		return
	}

	// Get related cases
	cases, _ := h.store.GetByIndex("Case", "AccountId", id)

	// Get related contacts
	contacts, _ := h.store.GetByIndex("Contact", "AccountId", id)
	sort.Slice(contacts, func(i, j int) bool {
		return fmt.Sprint(contacts[i]["Name"]) < fmt.Sprint(contacts[j]["Name"])
	})

	h.accountTpl.ExecuteTemplate(w, "account.html", map[string]any{
		"Account":  account,
		"Cases":    cases,
		"Contacts": contacts,
		"Title":    account["Name"],
		"BasePath": h.basePath,
	})
}

// Home renders the customer (Account) list as the application home page.
func (h *UIHandler) Home(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.store.Query("Account", func(r map[string]any) bool { return true })

	sort.Slice(accounts, func(i, j int) bool {
		return fmt.Sprint(accounts[i]["Name"]) < fmt.Sprint(accounts[j]["Name"])
	})

	h.homeTpl.ExecuteTemplate(w, "home.html", map[string]any{
		"Accounts": accounts,
		"Title":    "Home — Customers",
		"Total":    len(accounts),
		"BasePath": h.basePath,
	})
}

// ContactDetail renders the contact detail page with the related account.
func (h *UIHandler) ContactDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	contact, err := h.store.Get("Contact", id)
	if err != nil {
		http.Error(w, "Contact not found", 404)
		return
	}

	if accID, ok := contact["AccountId"].(string); ok && accID != "" {
		acc, err := h.store.Get("Account", accID)
		if err == nil {
			contact["_Account"] = acc
		}
	}

	// Cases where this contact is named
	cases, _ := h.store.Query("Case", func(r map[string]any) bool {
		return r["ContactId"] == id
	})

	name, _ := contact["Name"].(string)
	if name == "" {
		first, _ := contact["FirstName"].(string)
		last, _ := contact["LastName"].(string)
		name = first + " " + last
	}

	h.contactTpl.ExecuteTemplate(w, "contact.html", map[string]any{
		"Contact":  contact,
		"Cases":    cases,
		"Title":    name,
		"BasePath": h.basePath,
	})
}

// UserDetail renders the user detail page (internal users — owners/agents).
func (h *UIHandler) UserDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, err := h.store.Get("User", id)
	if err != nil {
		http.Error(w, "User not found", 404)
		return
	}

	// Cases owned by this user
	ownedCases, _ := h.store.Query("Case", func(r map[string]any) bool {
		return r["OwnerId"] == id
	})

	name, _ := user["Name"].(string)
	if name == "" {
		first, _ := user["FirstName"].(string)
		last, _ := user["LastName"].(string)
		name = first + " " + last
	}

	h.userTpl.ExecuteTemplate(w, "user.html", map[string]any{
		"User":       user,
		"OwnedCases": ownedCases,
		"Title":      name,
		"BasePath":   h.basePath,
	})
}

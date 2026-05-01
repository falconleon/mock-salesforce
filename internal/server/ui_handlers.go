package server

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sort"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// caseTabs lists the related-list tabs rendered on the case detail page,
// in display order. The first entry is the default when no ?tab= is given.
var caseTabs = []string{"emails", "comments", "feed", "activities", "files"}

// resolveCaseTab normalises the ?tab= query parameter against caseTabs,
// returning the default tab when the value is empty or unrecognised.
func resolveCaseTab(v string) string {
	for _, t := range caseTabs {
		if t == v {
			return t
		}
	}
	return caseTabs[0]
}

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// UIHandler serves browsable HTML pages for the mock Salesforce UI.
type UIHandler struct {
	store          store.Store
	basePath       string
	sessionSecret  string
	caseListTpl    *template.Template
	caseTpl        *template.Template
	accountTpl     *template.Template
	accountListTpl *template.Template
	homeTpl        *template.Template
	contactTpl     *template.Template
	userTpl        *template.Template
	casePartialTpl *template.Template
	logger         zerolog.Logger
}

// NewUIHandler creates a UIHandler with parsed templates and template functions.
// sessionSecret is used to decode the sf_session cookie for nav user display.
func NewUIHandler(s store.Store, basePath string, sessionSecret string) *UIHandler {
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
		"intField": func(r store.Record, key string) int {
			v, ok := r[key]
			if !ok {
				return 0
			}
			switch x := v.(type) {
			case int:
				return x
			case int64:
				return int(x)
			case float64:
				return int(x)
			}
			return 0
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
		sessionSecret:  sessionSecret,
		caseListTpl:    caseListTpl,
		caseTpl:        caseTpl,
		accountTpl:     accountTpl,
		accountListTpl: accountListTpl,
		homeTpl:        homeTpl,
		contactTpl:     contactTpl,
		userTpl:        userTpl,
		casePartialTpl: casePartialTpl,
		logger:         zerolog.Nop(),
	}
}

// WithLogger attaches a logger used to record template execution failures.
func (h *UIHandler) WithLogger(logger zerolog.Logger) *UIHandler {
	h.logger = logger
	return h
}

// logTemplateErr emits a structured error log for a failed template render.
func (h *UIHandler) logTemplateErr(err error, name, path string) {
	if err == nil {
		return
	}
	h.logger.Error().Err(err).Str("template", name).Str("path", path).Msg("template execution failed")
}

// currentUser returns the authenticated user's email from the session
// cookie, or "" if no valid session is present.
func (h *UIHandler) currentUser(r *http.Request) string {
	if h.sessionSecret == "" {
		return ""
	}
	email, _ := middleware.ValidateSession(r, h.sessionSecret)
	return email
}

// isOpenCaseStatus reports whether a Case Status string represents an
// open (not yet resolved) case. Closed and Resolved are terminal.
func isOpenCaseStatus(status string) bool {
	switch status {
	case "Closed", "Resolved":
		return false
	}
	return true
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

	err := h.caseListTpl.ExecuteTemplate(w, "case_list.html", map[string]any{
		"Cases":       cases,
		"Title":       "Salesforce — Cases",
		"Total":       len(cases),
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
	})
	h.logTemplateErr(err, "case_list.html", r.URL.Path)
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

	activeTab := resolveCaseTab(r.URL.Query().Get("tab"))
	var tabBuf bytes.Buffer
	var tabErr error
	var tabName string
	switch activeTab {
	case "comments":
		tabName = "case_comments"
		tabErr = h.casePartialTpl.ExecuteTemplate(&tabBuf, tabName, map[string]any{"Comments": comments})
	case "feed":
		tabName = "case_feed"
		tabErr = h.casePartialTpl.ExecuteTemplate(&tabBuf, tabName, map[string]any{"FeedItems": feedItems})
	case "activities":
		tabName = "case_activities"
		tabErr = h.casePartialTpl.ExecuteTemplate(&tabBuf, tabName, map[string]any{"Tasks": tasks, "Events": events})
	case "files":
		tabName = "case_files"
		tabErr = h.casePartialTpl.ExecuteTemplate(&tabBuf, tabName, map[string]any{"Files": files})
	default:
		tabName = "case_emails"
		tabErr = h.casePartialTpl.ExecuteTemplate(&tabBuf, tabName, map[string]any{"Emails": emails})
	}
	h.logTemplateErr(tabErr, tabName, r.URL.Path)

	pageErr := h.caseTpl.ExecuteTemplate(w, "case.html", map[string]any{
		"Case":            caseRec,
		"Emails":          emails,
		"Comments":        comments,
		"FeedItems":       feedItems,
		"Tasks":           tasks,
		"Events":          events,
		"Files":           files,
		"ActivitiesCount": len(tasks) + len(events),
		"ActiveTab":       activeTab,
		"InitialTabHTML":  template.HTML(tabBuf.String()),
		"Title":           caseRec["CaseNumber"],
		"BasePath":        h.basePath,
	})
	h.logTemplateErr(pageErr, "case.html", r.URL.Path)
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
	tplErr := h.casePartialTpl.ExecuteTemplate(w, "case_emails", map[string]any{
		"Emails": h.caseEmails(caseID),
	})
	h.logTemplateErr(tplErr, "case_emails", r.URL.Path)
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
	tplErr := h.casePartialTpl.ExecuteTemplate(w, "case_comments", map[string]any{
		"Comments": h.caseComments(caseID),
	})
	h.logTemplateErr(tplErr, "case_comments", r.URL.Path)
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
	tplErr := h.casePartialTpl.ExecuteTemplate(w, "case_feed", map[string]any{
		"FeedItems": h.caseFeedItems(caseID),
	})
	h.logTemplateErr(tplErr, "case_feed", r.URL.Path)
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
	tplErr := h.casePartialTpl.ExecuteTemplate(w, "case_activities", map[string]any{
		"Tasks":  h.caseTasks(caseID),
		"Events": h.caseEvents(caseID),
	})
	h.logTemplateErr(tplErr, "case_activities", r.URL.Path)
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
	tplErr := h.casePartialTpl.ExecuteTemplate(w, "case_files", map[string]any{
		"Files": h.caseFiles(caseID),
	})
	h.logTemplateErr(tplErr, "case_files", r.URL.Path)
}

// AccountList renders the accounts list page.
func (h *UIHandler) AccountList(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.store.Query("Account", func(r map[string]any) bool { return true })

	sort.Slice(accounts, func(i, j int) bool {
		return fmt.Sprint(accounts[i]["Name"]) < fmt.Sprint(accounts[j]["Name"])
	})

	err := h.accountListTpl.ExecuteTemplate(w, "account_list.html", map[string]any{
		"Accounts":    accounts,
		"Title":       "Salesforce — Accounts",
		"Total":       len(accounts),
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
	})
	h.logTemplateErr(err, "account_list.html", r.URL.Path)
}

// AccountDetail renders the account detail page with related cases and contacts.
func (h *UIHandler) AccountDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	account, err := h.store.Get("Account", id)
	if err != nil {
		http.Error(w, "Account not found", 404)
		return
	}

	// Get related cases (indexed by AccountId).
	cases, _ := h.store.GetByIndex("Case", "AccountId", id)
	sort.Slice(cases, func(i, j int) bool {
		a, _ := cases[i]["CaseNumber"].(string)
		b, _ := cases[j]["CaseNumber"].(string)
		return a < b
	})

	// Get related contacts
	contacts, _ := h.store.GetByIndex("Contact", "AccountId", id)
	sort.Slice(contacts, func(i, j int) bool {
		return fmt.Sprint(contacts[i]["Name"]) < fmt.Sprint(contacts[j]["Name"])
	})

	tplErr := h.accountTpl.ExecuteTemplate(w, "account.html", map[string]any{
		"Account":     account,
		"Cases":       cases,
		"Contacts":    contacts,
		"Title":       account["Name"],
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
	})
	h.logTemplateErr(tplErr, "account.html", r.URL.Path)
}

// Home renders the customer (Account) list as the application home page.
// Each account is annotated with `_OpenCases`, the count of related Cases
// whose Status is not Closed/Resolved, so the home table can show an
// at-a-glance open-case load per customer.
func (h *UIHandler) Home(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.store.Query("Account", func(r map[string]any) bool { return true })

	sort.Slice(accounts, func(i, j int) bool {
		return fmt.Sprint(accounts[i]["Name"]) < fmt.Sprint(accounts[j]["Name"])
	})

	for _, acc := range accounts {
		accID, _ := acc["Id"].(string)
		if accID == "" {
			acc["_OpenCases"] = 0
			continue
		}
		cases, _ := h.store.GetByIndex("Case", "AccountId", accID)
		open := 0
		for _, c := range cases {
			status, _ := c["Status"].(string)
			if isOpenCaseStatus(status) {
				open++
			}
		}
		acc["_OpenCases"] = open
	}

	err := h.homeTpl.ExecuteTemplate(w, "home.html", map[string]any{
		"Accounts":    accounts,
		"Title":       "Home — Customers",
		"Total":       len(accounts),
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
	})
	h.logTemplateErr(err, "home.html", r.URL.Path)
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

	tplErr := h.contactTpl.ExecuteTemplate(w, "contact.html", map[string]any{
		"Contact":     contact,
		"Cases":       cases,
		"Title":       name,
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
	})
	h.logTemplateErr(tplErr, "contact.html", r.URL.Path)
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

	tplErr := h.userTpl.ExecuteTemplate(w, "user.html", map[string]any{
		"User":        user,
		"OwnedCases":  ownedCases,
		"Title":       name,
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
	})
	h.logTemplateErr(tplErr, "user.html", r.URL.Path)
}

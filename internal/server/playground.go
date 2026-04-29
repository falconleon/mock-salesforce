package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/soql"
	"github.com/falconleon/mock-salesforce/internal/store"
)

// playgroundExamples are seeded into the sidebar of the SOQL playground.
// They exercise representative shapes of the parser/executor: simple
// projection, ORDER BY, COUNT, and GROUP BY.
var playgroundExamples = []string{
	"SELECT Id, Name, Industry FROM Account LIMIT 10",
	"SELECT Id, CaseNumber, Subject, Status, Priority FROM Case ORDER BY CaseNumber LIMIT 20",
	"SELECT COUNT(Id) FROM Case",
	"SELECT Status, COUNT(Id) FROM Case GROUP BY Status",
}

// PlaygroundHandler renders the SOQL playground page and executes
// queries submitted from it via the internal/soql executor.
type PlaygroundHandler struct {
	store         store.Store
	basePath      string
	sessionSecret string
	pageTpl       *template.Template
	resultsTpl    *template.Template
}

// NewPlaygroundHandler constructs a PlaygroundHandler with parsed templates.
func NewPlaygroundHandler(s store.Store, basePath, sessionSecret string) *PlaygroundHandler {
	funcs := template.FuncMap{
		"basePath": func() string { return basePath },
	}
	pageTpl := template.Must(template.New("").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/playground.html"))
	resultsTpl := template.Must(template.New("").Funcs(funcs).ParseFS(templateFS, "templates/partials/playground_results.html"))
	return &PlaygroundHandler{
		store:         s,
		basePath:      basePath,
		sessionSecret: sessionSecret,
		pageTpl:       pageTpl,
		resultsTpl:    resultsTpl,
	}
}

// Page renders the SOQL playground page shell (form + examples + empty results).
func (h *PlaygroundHandler) Page(w http.ResponseWriter, r *http.Request) {
	currentUser := ""
	if h.sessionSecret != "" {
		if email, ok := middleware.ValidateSession(r, h.sessionSecret); ok {
			currentUser = email
		}
	}
	_ = h.pageTpl.ExecuteTemplate(w, "playground.html", map[string]any{
		"Title":       "SOQL Playground",
		"BasePath":    h.basePath,
		"CurrentUser": currentUser,
		"Query":       r.URL.Query().Get("q"),
		"Examples":    playgroundExamples,
	})
}

// Run executes a SOQL query and renders the results partial.
func (h *PlaygroundHandler) Run(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, "MALFORMED_QUERY", err.Error(), 0)
		return
	}
	q := r.FormValue("q")
	if q == "" {
		h.renderError(w, "MALFORMED_QUERY", "Missing query", 0)
		return
	}

	start := time.Now()
	stmt, err := soql.NewParser(q).Parse()
	if err != nil {
		h.renderError(w, "MALFORMED_QUERY", err.Error(), time.Since(start))
		return
	}
	result, err := soql.NewExecutor(h.store).Execute(stmt)
	if err != nil {
		h.renderError(w, "QUERY_ERROR", err.Error(), time.Since(start))
		return
	}

	headers, rows := projectPlaygroundRows(stmt.Fields, result.Records)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.resultsTpl.ExecuteTemplate(w, "playground_results", map[string]any{
		"TotalSize": result.TotalSize,
		"Elapsed":   time.Since(start).String(),
		"Headers":   headers,
		"Rows":      rows,
	})
}

func (h *PlaygroundHandler) renderError(w http.ResponseWriter, code, msg string, elapsed time.Duration) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.resultsTpl.ExecuteTemplate(w, "playground_results", map[string]any{
		"Error":     msg,
		"ErrorCode": code,
		"Elapsed":   elapsed.String(),
	})
}

// projectPlaygroundRows derives header labels and stringified row cells from
// a SOQL query's projected fields and result records. For aggregate fields
// without an alias the canonical "AGG(field)" form is used as the lookup key,
// which the executor populates alongside the synthetic "exprN" alias.
func projectPlaygroundRows(fields []soql.Field, records []store.Record) ([]string, [][]string) {
	type colKey struct {
		Relation  string
		Name      string
		LookupKey string
	}
	headers := make([]string, 0, len(fields))
	keys := make([]colKey, 0, len(fields))
	for _, f := range fields {
		header := f.Alias
		if header == "" {
			if f.Aggregate != "" {
				header = f.AggregateKey()
			} else {
				header = f.FullName()
			}
		}
		headers = append(headers, header)
		key := f.Alias
		if key == "" {
			if f.Aggregate != "" {
				key = f.AggregateKey()
			} else {
				key = f.Name
			}
		}
		keys = append(keys, colKey{Relation: f.Relation, Name: f.Name, LookupKey: key})
	}

	rows := make([][]string, 0, len(records))
	for _, rec := range records {
		row := make([]string, len(keys))
		for i, k := range keys {
			row[i] = stringifyCell(lookupCell(rec, k.Relation, k.Name, k.LookupKey))
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func lookupCell(r store.Record, relation, name, key string) any {
	if relation != "" {
		if rel, ok := r[relation].(map[string]any); ok {
			return rel[name]
		}
		return nil
	}
	if v, ok := r[key]; ok {
		return v
	}
	return r[name]
}

func stringifyCell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

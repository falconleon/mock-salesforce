package server

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"

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
	logger        zerolog.Logger
}

// NewPlaygroundHandler constructs a PlaygroundHandler with parsed templates.
// The handler logs to a no-op logger by default; use WithLogger to attach one.
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
		logger:        zerolog.Nop(),
	}
}

// WithLogger attaches a logger used to record template execution failures.
func (h *PlaygroundHandler) WithLogger(logger zerolog.Logger) *PlaygroundHandler {
	h.logger = logger
	return h
}

// Page renders the SOQL playground page shell (form + examples + empty results).
func (h *PlaygroundHandler) Page(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, r.URL.Query().Get("q"), template.HTML(""))
}

// Run executes a SOQL query and renders the results. HTMX requests
// (HX-Request: true) receive just the playground_results partial for
// in-page swap; plain form submissions get the full playground page
// with the results rendered inline so the form works without JavaScript.
func (h *PlaygroundHandler) Run(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, "MALFORMED_QUERY", err.Error(), 0)
		return
	}
	q := r.FormValue("q")
	if q == "" {
		h.renderError(w, r, "MALFORMED_QUERY", "Missing query", 0)
		return
	}

	start := time.Now()
	stmt, err := soql.NewParser(q).Parse()
	if err != nil {
		h.renderError(w, r, "MALFORMED_QUERY", err.Error(), time.Since(start))
		return
	}
	result, err := soql.NewExecutor(h.store).Execute(stmt)
	if err != nil {
		h.renderError(w, r, "QUERY_ERROR", err.Error(), time.Since(start))
		return
	}

	headers, rows := projectPlaygroundRows(stmt.Fields, result.Records)
	h.renderResults(w, r, q, map[string]any{
		"TotalSize": result.TotalSize,
		"Elapsed":   time.Since(start).String(),
		"Headers":   headers,
		"Rows":      rows,
	})
}

func (h *PlaygroundHandler) renderError(w http.ResponseWriter, r *http.Request, code, msg string, elapsed time.Duration) {
	h.renderResults(w, r, r.FormValue("q"), map[string]any{
		"Error":     msg,
		"ErrorCode": code,
		"Elapsed":   elapsed.String(),
	})
}

// isHTMXRequest reports whether the request originated from an HTMX swap
// rather than a plain browser form submission or navigation.
func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// renderResults emits the playground_results partial. For HTMX swaps it
// writes the partial directly; otherwise it re-renders the full
// playground page with the results embedded so the page is usable
// without JavaScript.
func (h *PlaygroundHandler) renderResults(w http.ResponseWriter, r *http.Request, q string, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if isHTMXRequest(r) {
		if err := h.resultsTpl.ExecuteTemplate(w, "playground_results", data); err != nil {
			h.logger.Error().Err(err).Str("template", "playground_results").Str("path", r.URL.Path).Msg("template execution failed")
		}
		return
	}
	var buf bytes.Buffer
	if err := h.resultsTpl.ExecuteTemplate(&buf, "playground_results", data); err != nil {
		h.logger.Error().Err(err).Str("template", "playground_results").Str("path", r.URL.Path).Msg("template execution failed")
	}
	h.renderPage(w, r, q, template.HTML(buf.String()))
}

// renderPage writes the full playground page with an optional pre-rendered
// results block embedded in the results card.
func (h *PlaygroundHandler) renderPage(w http.ResponseWriter, r *http.Request, q string, results template.HTML) {
	currentUser := ""
	if h.sessionSecret != "" {
		if email, ok := middleware.ValidateSession(r, h.sessionSecret); ok {
			currentUser = email
		}
	}
	if err := h.pageTpl.ExecuteTemplate(w, "playground.html", map[string]any{
		"Title":       "SOQL Playground",
		"BasePath":    h.basePath,
		"CurrentUser": currentUser,
		"Query":       q,
		"Examples":    playgroundExamples,
		"Results":     results,
	}); err != nil {
		h.logger.Error().Err(err).Str("template", "playground.html").Str("path", r.URL.Path).Msg("template execution failed")
	}
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

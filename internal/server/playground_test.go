package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestPlaygroundHandler(t *testing.T) *PlaygroundHandler {
	t.Helper()
	return NewPlaygroundHandler(uiTestStore(t), "", "")
}

func TestPlaygroundPageRenders(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/playground", nil)
	rr := httptest.NewRecorder()

	h.Page(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"SOQL Playground",
		"playground-form",
		`name="q"`,
		"/playground/run",
		"Examples",
		"SELECT Id, Name, Industry FROM Account",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page body missing %q\n%s", want, body)
		}
	}
}

func TestPlaygroundPagePrefillsQueryParam(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/playground?q=SELECT+Id+FROM+Account", nil)
	rr := httptest.NewRecorder()

	h.Page(rr, req)

	if !strings.Contains(rr.Body.String(), "SELECT Id FROM Account") {
		t.Errorf("expected prefilled query in textarea; body=%s", rr.Body.String())
	}
}

func runPlayground(t *testing.T, h *PlaygroundHandler, query string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("q", query)
	req := httptest.NewRequest(http.MethodPost, "/playground/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.Run(rr, req)
	return rr
}

func TestPlaygroundRunReturnsTable(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "SELECT Id, Name FROM Account")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"2 rows",
		"<th>Id</th>",
		"<th>Name</th>",
		"Acme Corp",
		"Beta Industries",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("results body missing %q\n%s", want, body)
		}
	}
}

func TestPlaygroundRunRendersAggregateColumn(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "SELECT COUNT(Id) FROM Case")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"<th>COUNT(Id)</th>", "<td>1</td>"} {
		if !strings.Contains(body, want) {
			t.Errorf("aggregate body missing %q\n%s", want, body)
		}
	}
}

func TestPlaygroundRunRendersGroupByRows(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "SELECT Status, COUNT(Id) FROM Case GROUP BY Status")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"<th>Status</th>",
		"<th>COUNT(Id)</th>",
		"Working",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("group-by body missing %q\n%s", want, body)
		}
	}
}

func TestPlaygroundRunRendersParseError(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "this is not soql")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (error rendered as partial)", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{"playground-error", "MALFORMED_QUERY"} {
		if !strings.Contains(body, want) {
			t.Errorf("error body missing %q\n%s", want, body)
		}
	}
}

func TestPlaygroundRunRequiresQuery(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "")

	if !strings.Contains(rr.Body.String(), "Missing query") {
		t.Errorf("expected 'Missing query' error; body=%s", rr.Body.String())
	}
}

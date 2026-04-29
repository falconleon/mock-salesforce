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

// runPlaygroundHTMX simulates a POST originating from an HTMX swap by
// setting the HX-Request header, which selects the partial-only response.
func runPlaygroundHTMX(t *testing.T, h *PlaygroundHandler, query string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("q", query)
	req := httptest.NewRequest(http.MethodPost, "/playground/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	h.Run(rr, req)
	return rr
}

func TestPlaygroundPageFormHasMethodAndAction(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/playground", nil)
	rr := httptest.NewRecorder()

	h.Page(rr, req)

	body := rr.Body.String()
	for _, want := range []string{
		`method="post"`,
		`action="/playground/run"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page body missing %q (no-JS form fallback); body=%s", want, body)
		}
	}
}

func TestPlaygroundPageExampleLinksHaveHref(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/playground", nil)
	rr := httptest.NewRecorder()

	h.Page(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "sfPlaygroundLoad") {
		t.Errorf("expected example links to be plain href anchors, found onclick handler; body=%s", body)
	}
	// First example query should appear in an href so the example link
	// works without JavaScript via the ?q= prefill on /playground.
	if !strings.Contains(body, `href="/playground?q=SELECT`) {
		t.Errorf("expected example link with href=/playground?q=...; body=%s", body)
	}
}

func TestPlaygroundRunRendersFullPageForNonHTMX(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "SELECT Id, Name FROM Account")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	// Full-page response must include the page chrome AND the results.
	for _, want := range []string{
		"<!DOCTYPE html>",
		"playground-form",
		"Examples",
		"2 rows",
		"Acme Corp",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("non-HTMX run body missing %q\n%s", want, body)
		}
	}
	// Submitted query should be re-populated in the textarea so the user
	// can refine it without retyping.
	if !strings.Contains(body, "SELECT Id, Name FROM Account") {
		t.Errorf("expected submitted query echoed into textarea; body=%s", body)
	}
}

func TestPlaygroundRunRendersPartialForHTMX(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlaygroundHTMX(t, h, "SELECT Id, Name FROM Account")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	// HTMX response must be the bare partial — no page chrome.
	for _, unwanted := range []string{"<!DOCTYPE html>", "playground-form", "<body"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("HTMX run body should not contain page chrome %q\n%s", unwanted, body)
		}
	}
	for _, want := range []string{"2 rows", "Acme Corp"} {
		if !strings.Contains(body, want) {
			t.Errorf("HTMX run body missing %q\n%s", want, body)
		}
	}
}

func TestPlaygroundRunRendersErrorAsFullPageForNonHTMX(t *testing.T) {
	h := newTestPlaygroundHandler(t)
	rr := runPlayground(t, h, "this is not soql")

	body := rr.Body.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"playground-form",
		"playground-error",
		"MALFORMED_QUERY",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("non-HTMX error body missing %q\n%s", want, body)
		}
	}
	// The bad query must be echoed back so the user can edit it.
	if !strings.Contains(body, "this is not soql") {
		t.Errorf("expected submitted (invalid) query echoed back; body=%s", body)
	}
}

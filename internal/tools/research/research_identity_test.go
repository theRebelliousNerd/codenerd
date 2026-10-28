package research

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/tools"
)

// Finding two of Docs/audits/research-observation-identity-and-admission.md:
// a page with no recognized result cards became "No results found" -- an
// unknown promoted to negative evidence.
func TestDuckDuckGoReportsNoResults_OnlyForTheProvidersOwnEmptyState(t *testing.T) {
	if !duckDuckGoReportsNoResults(`<html><body><div id="links" class="results"><div class="no-results">No results.</div></div></body></html>`) {
		t.Fatal("the provider's empty-results state was not recognized")
	}
	for _, page := range []string{
		`<html><body></body></html>`,
		`<html><body><form id="challenge-form">Please confirm you are not a robot</form></body></html>`,
		`<html><body><div class="result-list-v2">new layout</div></body></html>`,
		`<html><body><div class="no-results-banner">ad</div></body></html>`,
	} {
		if duckDuckGoReportsNoResults(page) {
			t.Errorf("%s was read as a confirmed empty search", page)
		}
	}
}

// Finding three: relative links were emitted raw; they resolve against the
// document's base, and credentials never travel in a link.
func TestHTMLToMarkdown_ResolvesRelativeLinksAgainstTheDocumentBase(t *testing.T) {
	md, err := htmlToMarkdown(`<html><body><a href="migration.html">Migrate</a> <a href="/root.html">Root</a> <a href="https://u:p@other.test/x">Abs</a></body></html>`,
		"https://docs.example.test/v2/guide/index.html", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"](https://docs.example.test/v2/guide/migration.html)",
		"](https://docs.example.test/root.html)",
		"](https://other.test/x)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestDocumentNotice_NamesARedirectedDocumentOnly(t *testing.T) {
	if got := documentNotice("http://127.0.0.1:8080", "http://127.0.0.1:8080/"); got != "" {
		t.Errorf("same document reported as a redirect: %q", got)
	}
	if got := documentNotice("https://x.test/old", "https://x.test/v2/new"); !strings.Contains(got, "[document: https://x.test/v2/new (requested https://x.test/old)]") {
		t.Errorf("redirect not named: %q", got)
	}
}

// Through the real browser: a redirected fetch names the document it ended on
// and resolves its relative link against it; an unrecognized search page is
// an error, a recognized empty one is a scoped "no results".
func TestRegisteredResearchTools_ReportTheDocumentAndTheSearchStateTheyObserved(t *testing.T) {
	withBoundResearchRuntime(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/old":
			http.Redirect(w, r, "/v2/guide/", http.StatusFound)
		case "/v2/guide/":
			_, _ = fmt.Fprint(w, `<html><body><p>Guide</p><a href="migration.html">Migrate</a></body></html>`)
		case "/html/":
			if r.URL.Query().Get("q") == "empty" {
				_, _ = fmt.Fprint(w, `<html><body><div id="links" class="results"><div class="no-results">No results.</div></div></body></html>`)
				return
			}
			_, _ = fmt.Fprint(w, `<html><body><p>Please confirm you are not a robot</p></body></html>`)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)
	previous := duckDuckGoSearchURL
	duckDuckGoSearchURL = server.URL + "/html/?q=%s"
	t.Cleanup(func() { duckDuckGoSearchURL = previous })

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatal(err)
	}
	fetched, err := registry.Execute(context.Background(), "web_fetch", map[string]any{"url": server.URL + "/old"})
	if err != nil || fetched == nil || !fetched.IsSuccess() {
		t.Fatalf("web_fetch: %+v %v", fetched, err)
	}
	if !strings.Contains(fetched.Result, "[document: "+server.URL+"/v2/guide/ (requested "+server.URL+"/old)]") {
		t.Errorf("redirected document not named:\n%s", fetched.Result)
	}
	if !strings.Contains(fetched.Result, "]("+server.URL+"/v2/guide/migration.html)") {
		t.Errorf("relative link not resolved against the document:\n%s", fetched.Result)
	}

	challenged, err := registry.Execute(context.Background(), "web_search", map[string]any{"query": "challenged"})
	if err == nil && challenged != nil && challenged.IsSuccess() {
		t.Fatalf("an unrecognized page was reported as a search answer: %s", challenged.Result)
	}
	empty, err := registry.Execute(context.Background(), "web_search", map[string]any{"query": "empty"})
	if err != nil || empty == nil || !empty.IsSuccess() || !strings.Contains(empty.Result, "DuckDuckGo reported no results") {
		t.Fatalf("a recognized empty page: %+v %v", empty, err)
	}
}

// Finding four: every paged fetch navigates again, so an offset could
// continue a different version of the page. The continuation names the
// snapshot it continues and is refused when the document changed.
func TestWebFetch_AContinuationOfAChangedDocumentIsRefused(t *testing.T) {
	withBoundResearchRuntime(t)
	var body atomic.Value
	body.Store(strings.Repeat("a", 100))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, body.Load().(string))
	}))
	t.Cleanup(server.Close)

	first, err := executeWebFetch(context.Background(), map[string]any{"url": server.URL, "max_length": 40})
	if err != nil {
		t.Fatal(err)
	}
	_, hint, ok := strings.Cut(first, "and snapshot=")
	if !ok {
		t.Fatalf("paging notice names no snapshot: %q", first)
	}
	snapshot := strings.TrimSuffix(strings.TrimSpace(hint), "]")
	if _, err := executeWebFetch(context.Background(), map[string]any{"url": server.URL, "max_length": 40, "offset": 40, "snapshot": snapshot}); err != nil {
		t.Fatalf("continuing the unchanged document: %v", err)
	}
	body.Store("inserted " + body.Load().(string))
	_, err = executeWebFetch(context.Background(), map[string]any{"url": server.URL, "max_length": 40, "offset": 40, "snapshot": snapshot})
	if err == nil || !strings.Contains(err.Error(), "changed since snapshot") {
		t.Fatalf("continuing a changed document was not refused: %v", err)
	}
}

package research

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func TestParseDuckDuckGoResults_RedirectPreservesDestinationQueryAndFragment(t *testing.T) {
	destination := "https://example.test/page?a=1&b=two#section"
	redirect := "//duckduckgo.com/l/?uddg=" + url.QueryEscape(destination) + "&rut=tracking-token"
	page := fmt.Sprintf(`<div class="result results_links"><a class="result__a" href="%s">Fixture</a></div>`, redirect)

	results, err := parseDuckDuckGoResults(page, 0)
	if err != nil {
		t.Fatalf("parseDuckDuckGoResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %+v, want one result", results)
	}
	if results[0].URL != destination {
		t.Fatalf("destination URL = %q, want %q", results[0].URL, destination)
	}
}

func TestParseDuckDuckGoResults_MissingOrMalformedRedirectDestinationIsSkipped(t *testing.T) {
	for name, href := range map[string]string{
		"missing uddg":   "//duckduckgo.com/l/?rut=tracking-token",
		"empty uddg":     "//duckduckgo.com/l/?uddg=&rut=tracking-token",
		"invalid escape": "//duckduckgo.com/l/?uddg=%ZZ&rut=tracking-token",
	} {
		t.Run(name, func(t *testing.T) {
			page := fmt.Sprintf(`<div class="result results_links"><a class="result__a" href="%s">Fixture</a></div>`, href)
			results, err := parseDuckDuckGoResults(page, 0)
			if err != nil {
				t.Fatalf("parseDuckDuckGoResults: %v", err)
			}
			if len(results) != 0 {
				t.Fatalf("results = %+v, want malformed redirect skipped", results)
			}
		})
	}
}

func TestRegisteredWebSearch_PreservesRedirectDestinationQueryAndFragment(t *testing.T) {
	withBoundResearchRuntime(t)
	destination := "https://example.test/page?a=1&b=two#section"
	redirect := "//duckduckgo.com/l/?uddg=" + url.QueryEscape(destination) + "&rut=tracking-token"
	redirect = strings.ReplaceAll(redirect, "&", "&amp;")
	page := fmt.Sprintf(`<html><body><div class="result results_links"><a class="result__a" href="%s">Fixture result</a></div></body></html>`, redirect)
	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/html/" {
			// Chrome may request a favicon after the search document. It is not
			// part of the provider fixture contract.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("search request method = %q, want GET", r.Method)
		}
		if got := r.URL.Query().Get("q"); got != "registry search" {
			t.Errorf("search query = %q, want %q", got, "registry search")
		}
		select {
		case requests <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, page)
	}))
	t.Cleanup(server.Close)

	previousSearchURL := duckDuckGoSearchURL
	duckDuckGoSearchURL = server.URL + "/html/?q=%s"
	t.Cleanup(func() { duckDuckGoSearchURL = previousSearchURL })

	registry := tools.NewRegistry()
	if err := RegisterAll(registry); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	beforeTabs := researchTabCount(t)
	result, err := registry.Execute(context.Background(), "web_search", map[string]any{
		"query": "registry search",
	})
	if err != nil {
		t.Fatalf("Registry.Execute(web_search): %v", err)
	}
	if result == nil || !result.IsSuccess() {
		t.Fatalf("registered web_search result = %+v, want success", result)
	}
	if !strings.Contains(result.Result, "**URL:** "+destination+"\n") {
		t.Fatalf("registered search output did not preserve complete destination URL %q: %s", destination, result.Result)
	}
	if strings.Contains(result.Result, "rut=tracking-token") {
		t.Fatalf("registered search output leaked redirect tracking parameter: %s", result.Result)
	}
	select {
	case <-requests:
	default:
		t.Fatal("registered web_search did not reach the local HTTP fixture")
	}
	if got := researchTabCount(t); got != beforeTabs {
		t.Fatalf("research tabs after registered search = %d, baseline %d", got, beforeTabs)
	}
}

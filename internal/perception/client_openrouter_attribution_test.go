package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"codenerd/internal/config"
)

// OpenRouter's attribution headers were always sent, an empty HTTP-Referer
// included, and the site URL could not be configured. They now come from
// openrouter.site_url and openrouter.app_title through the factory, and an
// unset one is not sent.
func TestOpenRouter_SendsOnlyTheConfiguredAttribution(t *testing.T) {
	var mu sync.Mutex
	var headers []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers = append(headers, r.Header.Clone())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	call := func(opts *config.OpenRouterProviderConfig) http.Header {
		t.Helper()
		client, err := NewClientFromConfig(&ProviderConfig{
			Engine: "api", Provider: ProviderOpenRouter, APIKey: "k", BaseURL: srv.URL, Model: "vendor/model", OpenRouter: opts,
		})
		if err != nil {
			t.Fatalf("NewClientFromConfig: %v", err)
		}
		if _, err := client.CompleteWithSystem(context.Background(), "sys", "hello"); err != nil {
			t.Fatalf("CompleteWithSystem: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		return headers[len(headers)-1]
	}

	set := call(&config.OpenRouterProviderConfig{SiteURL: "https://example.dev", AppTitle: "Example App"})
	if set.Get("HTTP-Referer") != "https://example.dev" || set.Get("X-Title") != "Example App" {
		t.Fatalf("configured attribution not sent: Referer=%q Title=%q", set.Get("HTTP-Referer"), set.Get("X-Title"))
	}
	unset := call(nil)
	if _, ok := unset["Http-Referer"]; ok {
		t.Fatalf("an unset site URL sent a Referer header: %q", unset.Values("HTTP-Referer"))
	}
	if _, ok := unset["X-Title"]; ok {
		t.Fatalf("an unset app title sent an X-Title header: %q", unset.Values("X-Title"))
	}
}

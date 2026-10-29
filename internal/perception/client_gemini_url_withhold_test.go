package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The provider cap stays on the request. SetURLContextURLs cannot return the
// tail — GroundingController fixes that method as void — and lastRequest /
// lastGroundingSources are a rate-limit clock and response citations. The
// caller reads the withheld URLs off the client.
func TestGroundingGeminiURLContext_When25URLs_Sends20AndCallerReadsWithheld(t *testing.T) {
	const extra = 5
	total := maxGeminiURLContextURLs + extra
	urls := make([]string, total)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://docs.example/%02d", i)
	}

	var (
		gotBody   GeminiRequest
		decodeErr error
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decodeErr = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	c.SetEnableURLContext(true)
	c.SetURLContextURLs(urls)

	withheld := c.GetWithheldURLContextURLs()
	if len(withheld) != extra {
		t.Fatalf("withheld before send = %d, want %d", len(withheld), extra)
	}
	for i := range withheld {
		if withheld[i] != urls[maxGeminiURLContextURLs+i] {
			t.Fatalf("withheld[%d] = %q, want %q", i, withheld[i], urls[maxGeminiURLContextURLs+i])
		}
	}
	withheld[0] = "MUTATED"
	if again := c.GetWithheldURLContextURLs(); again[0] == "MUTATED" {
		t.Fatal("caller mutated the withheld record")
	}

	if _, err := c.Complete(context.Background(), "hi"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if decodeErr != nil {
		t.Fatalf("decode request: %v", decodeErr)
	}
	sent := urlContextURLsSent(gotBody)
	if len(sent) != maxGeminiURLContextURLs {
		t.Fatalf("sent %d URL-context URLs, want %d", len(sent), maxGeminiURLContextURLs)
	}
	for i := range sent {
		if sent[i] != urls[i] {
			t.Fatalf("sent[%d] = %q, want %q", i, sent[i], urls[i])
		}
	}
	withheld = c.GetWithheldURLContextURLs()
	if len(withheld) != extra || withheld[0] != urls[maxGeminiURLContextURLs] {
		t.Fatalf("withheld after send = %v, want the %d not sent", withheld, extra)
	}

	// A later list that fits clears the record, and the next request sends it all.
	c.SetURLContextURLs(urls[:2])
	if left := c.GetWithheldURLContextURLs(); len(left) != 0 {
		t.Fatalf("withheld after a fitting list = %v, want none", left)
	}
	if _, err := c.Complete(context.Background(), "hi"); err != nil {
		t.Fatalf("Complete fitting list: %v", err)
	}
	if decodeErr != nil {
		t.Fatalf("decode fitting request: %v", decodeErr)
	}
	sent = urlContextURLsSent(gotBody)
	if len(sent) != 2 || sent[0] != urls[0] || sent[1] != urls[1] {
		t.Fatalf("sent after a fitting list = %v, want the two URLs", sent)
	}
}

// A list from GeminiConfig never passes through SetURLContextURLs, so the
// tail has to be readable off the constructed client, and the request still
// sends only the provider prefix.
func TestGroundingGeminiURLContext_ConfigListRecordsWithheldBeforeSend(t *testing.T) {
	const extra = 5
	total := maxGeminiURLContextURLs + extra
	urls := make([]string, total)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://docs.example/cfg/%02d", i)
	}

	c := NewGeminiClientWithConfig(GeminiConfig{
		APIKey:           "k",
		Model:            "gemini-2.5-flash",
		EnableURLContext: true,
		URLContextURLs:   urls,
	})
	withheld := c.GetWithheldURLContextURLs()
	if len(withheld) != extra || withheld[extra-1] != urls[total-1] {
		t.Fatalf("withheld before any request = %v, want the last %d", withheld, extra)
	}

	built := c.buildBuiltInTools()
	if len(built) != 1 || built[0].URLContext == nil {
		t.Fatalf("built tools = %+v, want one URL-context tool", built)
	}
	sent := built[0].URLContext.URLs
	if len(sent) != maxGeminiURLContextURLs || sent[0] != urls[0] {
		t.Fatalf("sent = %d URLs starting %q, want %d starting %q", len(sent), firstOrEmpty(sent), maxGeminiURLContextURLs, urls[0])
	}
	if again := c.GetWithheldURLContextURLs(); len(again) != extra || again[0] != urls[maxGeminiURLContextURLs] {
		t.Fatalf("withheld after build = %v, want the %d not sent", again, extra)
	}
}

func urlContextURLsSent(req GeminiRequest) []string {
	var sent []string
	for _, tool := range req.Tools {
		if tool.URLContext != nil {
			sent = append(sent, tool.URLContext.URLs...)
		}
	}
	return sent
}

func firstOrEmpty(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

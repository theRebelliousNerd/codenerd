package sessionlog

import (
	"net/http"
	"strings"
	"testing"
)

func TestRedactHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer abc")
	h.Set("X-Goog-Api-Key", "abc")
	h.Set("OpenAI-Organization", "org")
	h.Set("Chatgpt-Account-Id", "acct")
	h.Set("X-Custom-Token", "abc")
	h.Set("Content-Type", "application/json")
	h.Set("Anthropic-Beta", "x")
	got := RedactHeaders(h)
	for _, k := range []string{"Authorization", "X-Goog-Api-Key", "Chatgpt-Account-Id", "X-Custom-Token"} {
		if got[k][0] != Redacted {
			t.Errorf("%s = %v, want redacted", k, got[k])
		}
	}
	if got["Content-Type"][0] != "application/json" || got["Anthropic-Beta"][0] != "x" || got["Openai-Organization"][0] != "org" {
		t.Errorf("non-credential headers must be kept: %v", got)
	}
	if RedactHeaders(nil) != nil {
		t.Error("nil headers stay nil")
	}
}

func TestRedactURL(t *testing.T) {
	cases := map[string]string{
		"https://x.test/v1?key=SECRET&alt=sse":    "alt=sse",
		"https://user:pw@x.test/v1":               "x.test/v1",
		"https://x.test/v1?access_token=SECRET":   "access_token=REDACTED",
		"https://x.test/v1?model=a&prompt=hi%20u": "prompt=hi%20u",
	}
	for in, mustHave := range cases {
		got := RedactURL(in)
		if strings.Contains(got, "SECRET") || strings.Contains(got, "pw@") {
			t.Errorf("RedactURL(%q) = %q leaks", in, got)
		}
		if !strings.Contains(got, mustHave) {
			t.Errorf("RedactURL(%q) = %q, want it to keep %q", in, got, mustHave)
		}
	}
	if got := RedactURL("://bad url"); got != Redacted {
		t.Errorf("an unparseable URL must not be echoed, got %q", got)
	}
}

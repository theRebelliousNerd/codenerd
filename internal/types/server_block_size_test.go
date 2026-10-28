package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func fetchResult(t *testing.T, source map[string]any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"type":        "web_fetch_tool_result",
		"tool_use_id": "srvtoolu_1",
		"content": map[string]any{
			"type":   "web_fetch_result",
			"url":    "https://example.com/a",
			"source": source,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestServerBlockChars(t *testing.T) {
	small := fetchResult(t, map[string]any{"type": "text", "media_type": "text/plain", "data": "tiny"})
	large := fetchResult(t, map[string]any{"type": "text", "media_type": "text/plain", "data": strings.Repeat("page ", 4000)})
	if got, want := ServerBlockChars(small), len(small); got != want {
		t.Errorf("a text block is counted as the text it is: got %d, want %d", got, want)
	}
	if ServerBlockChars(large) <= ServerBlockChars(small)+19000 {
		t.Errorf("a larger fetched page must count larger: small=%d large=%d", ServerBlockChars(small), ServerBlockChars(large))
	}

	encoded := strings.Repeat("A", 4000)
	pdf := fetchResult(t, map[string]any{"type": "base64", "media_type": "application/pdf", "data": encoded})
	if got, want := ServerBlockChars(pdf), len(pdf)-1000; got != want {
		t.Errorf("a base64 body is charged at three quarters of its encoded length: got %d, want %d", got, want)
	}

	search, err := json.Marshal(map[string]any{
		"type": "web_search_tool_result",
		"content": []any{map[string]any{
			"type": "web_search_result", "url": "https://e.com", "title": "t",
			"encrypted_content": encoded,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ServerBlockChars(search); got >= len(search) || got < len(search)-1000 {
		t.Errorf("encrypted search content is charged at its decoded bound: got %d for %d raw bytes", got, len(search))
	}
}

func TestServerBlockCharsInvalidJSONIsChargedWhole(t *testing.T) {
	raw := []byte(`{"type":"web_fetch_tool_result", "content": ` + strings.Repeat("x", 500))
	if got := ServerBlockChars(raw); got != len(raw) {
		t.Errorf("unclassifiable raw must be charged whole, not as nothing: got %d, want %d", got, len(raw))
	}
}

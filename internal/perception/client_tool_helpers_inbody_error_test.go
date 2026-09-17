package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAPIErrorCode pins APIErrorCode.UnmarshalJSON (F-OR-3):
// a JSON number becomes its decimal text, a JSON string is kept, and JSON
// null decodes as empty (matching the old string semantics for absent codes).
func TestAPIErrorCode(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantCode   string
		wantStatus int
		wantOK     bool
	}{
		{"numeric code", `{"code":502}`, "502", 502, true},
		{"string numeric code", `{"code":"502"}`, "502", 502, true},
		{"string non-numeric code", `{"code":"rate_limited"}`, "rate_limited", 0, false},
		{"null code", `{"code":null}`, "", 0, false},
		{"missing code", `{}`, "", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got APIErrorCode
			if err := json.Unmarshal([]byte(tc.body), &got); err != nil {
				t.Fatalf("Unmarshal(%s) = %v, want nil", tc.body, err)
			}
			if got != APIErrorCode(tc.wantCode) {
				t.Fatalf("code = %q, want %q", got, tc.wantCode)
			}
			status, ok := got.HTTPStatus()
			if ok != tc.wantOK || status != tc.wantStatus {
				t.Fatalf("HTTPStatus() = (%d, %v), want (%d, %v)", status, ok, tc.wantStatus, tc.wantOK)
			}
		})
	}
}


// TestExecuteOpenAIRequestInBodyTransientError pins F-OR-3: OpenRouter
// reports upstream failures as HTTP 200 + {"error":{"code":<number>}}, so
// an in-body 502/429/408-style code must be retried exactly like the same
// HTTP status, and the second attempt's completion must be returned.
// The first retry sleeps 1s (backoff = 1<<0 seconds in ExecuteOpenAIRequest).
func TestExecuteOpenAIRequestInBodyTransientError(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = w.Write([]byte(`{"error":{"code":502,"message":"Provider returned error"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	resp, err := ExecuteOpenAIRequest(context.Background(), srv.Client(), srv.URL, "key", OpenAIRequest{})
	if err != nil {
		t.Fatalf("ExecuteOpenAIRequest() error = %v, want nil", err)
	}
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp = %+v, want a completion with nil Error", resp)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected choices: %+v", resp.Choices)
	}
	if requests != 2 {
		t.Fatalf("server saw %d requests, want 2", requests)
	}
}

// TestExecuteOpenAIRequestInBodyPermanentErrors pins the non-transient
// side of F-OR-3: a string code ("invalid_api_key") or an in-body 400 must
// surface the provider's code and message as an error without any retry.
func TestExecuteOpenAIRequestInBodyPermanentErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantSub []string
	}{
		{"string non-numeric code", `{"error":{"code":"invalid_api_key","message":"bad key"}}`, []string{"invalid_api_key", "bad key"}},
		{"numeric non-transient code", `{"error":{"code":400,"message":"bad request"}}`, []string{"400", "bad request"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := ExecuteOpenAIRequest(context.Background(), srv.Client(), srv.URL, "key", OpenAIRequest{})
			if err == nil {
				t.Fatalf("ExecuteOpenAIRequest() = nil error, want one containing %v", tc.wantSub)
			}
			for _, sub := range tc.wantSub {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q does not contain %q", err.Error(), sub)
				}
			}
			if requests != 1 {
				t.Errorf("server saw %d requests, want 1 (not retried)", requests)
			}
		})
	}
}

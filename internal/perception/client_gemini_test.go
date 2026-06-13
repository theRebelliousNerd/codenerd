package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsTransientGeminiStatus is a table test for the retry-classification
// helper. The 5xx family (500/502/503/504) is retryable; everything else —
// including 429 (handled separately with its own message) and the 4xx client
// errors — is NOT classified transient here.
func TestIsTransientGeminiStatus(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{http.StatusInternalServerError, true}, // 500
		{http.StatusBadGateway, true},          // 502
		{http.StatusServiceUnavailable, true},  // 503 — the Gemini "high demand" case
		{http.StatusGatewayTimeout, true},      // 504
		{http.StatusOK, false},                 // 200
		{http.StatusTooManyRequests, false},    // 429 handled separately, not here
		{http.StatusBadRequest, false},         // 400
		{http.StatusUnauthorized, false},       // 401
		{http.StatusNotFound, false},           // 404
		{http.StatusNotImplemented, false},     // 501 (not in the retry set)
	}
	for _, tc := range cases {
		if got := isTransientGeminiStatus(tc.code); got != tc.want {
			t.Errorf("isTransientGeminiStatus(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

// TestGeminiClient_CompleteWithSystem_RetriesOn503 proves the regression fix:
// a 503 UNAVAILABLE (the "model is experiencing high demand" transient failure
// that collapsed a real user turn into a misleading clarification) is now
// retried with backoff instead of hard-returned. The mock serves one 503 then a
// valid 200, and the client must transparently recover and return the response.
// Backoff before retry 1 is 1s (time.Duration(1<<0)*time.Second), so this stays
// fast.
func TestGeminiClient_CompleteWithSystem_RetriesOn503(t *testing.T) {
	var calls int32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			// First attempt: simulate Gemini 503 UNAVAILABLE.
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": {"code": 503, "status": "UNAVAILABLE", "message": "The model is overloaded. Please try again later."}}`))
			return
		}
		// Retry: succeed with a normal completion.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"recovered after 503"}]}}]}`))
	}))
	defer mockServer.Close()

	client := &GeminiClient{
		apiKey:  "test-api-key",
		baseURL: mockServer.URL,
		model:   "gemini-3.5-flash",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	resp, err := client.CompleteWithSystem(context.Background(), "system", "user")
	require.NoError(t, err, "503-then-200 should retry and succeed, not hard-fail")
	assert.Equal(t, "recovered after 503", resp)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "expected exactly one 503 then one successful retry")
}

// TestGeminiClient_CompleteWithSchema_RetriesOn503 mirrors the above for the
// schema path — the exact call that failed in the live perception classification
// (ParseIntentWithGCD routes through structured-output classification).
func TestGeminiClient_CompleteWithSchema_RetriesOn503(t *testing.T) {
	var calls int32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": {"code": 503, "status": "UNAVAILABLE", "message": "high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"ok\":true}"}]}}]}`))
	}))
	defer mockServer.Close()

	client := &GeminiClient{
		apiKey:  "test-api-key",
		baseURL: mockServer.URL,
		model:   "gemini-3.5-flash",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	resp, err := client.CompleteWithSchema(context.Background(), "system", "user", `{"type":"object"}`)
	require.NoError(t, err, "503-then-200 on schema path should retry and succeed")
	assert.Equal(t, `{"ok":true}`, resp)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestGeminiClient_CountTokens(t *testing.T) {
	// Create a mock HTTP server that simulates the Gemini CountTokens API
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "countTokens")
		assert.Equal(t, "test-api-key", r.URL.Query().Get("key"))

		// Verify the request body parses correctly
		var reqBody GeminiRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Verify contents
		require.NotNil(t, reqBody.SystemInstruction)
		assert.Equal(t, "system prompt text", reqBody.SystemInstruction.Parts[0].Text)

		require.Len(t, reqBody.Contents, 1)
		assert.Equal(t, "user prompt text", reqBody.Contents[0].Parts[0].Text)

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"totalTokens": 42}`))
	}))
	defer mockServer.Close()

	// Initialize GeminiClient with the mock server's URL
	client := &GeminiClient{
		apiKey:                "test-api-key",
		baseURL:               mockServer.URL, // Replace default base URL with mock server
		model:                 "gemini-3.1-flash-lite",
		maxOutputTokens:       8192,
		maxOutputTokensConfig: true,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	// Execute CountTokens
	tokens, err := client.CountTokens(context.Background(), "system prompt text", "user prompt text")

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, 42, tokens)
}

func TestGeminiClient_CountTokens_APIError(t *testing.T) {
	// Mock server returning an error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal Server Error"}}`))
	}))
	defer mockServer.Close()

	client := &GeminiClient{
		apiKey:  "test-api-key",
		baseURL: mockServer.URL,
		model:   "gemini-3.1-flash-lite",
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}

	tokens, err := client.CountTokens(context.Background(), "system", "user")

	require.Error(t, err)
	assert.Equal(t, 0, tokens)
	assert.Contains(t, err.Error(), "status 500")
}

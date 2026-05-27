package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

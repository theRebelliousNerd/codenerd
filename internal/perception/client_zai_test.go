package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestZAIClient_Complete_Success(t *testing.T) {
	// Mock ZAI API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Expected test-key authorization")
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "chatcmpl-123",
			"choices": [
				{
					"message": {
						"content": "Hello, world!"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	// Create client and override baseURL (field accessible in same package)
	client := NewZAIClient("test-key")
	client.baseURL = server.URL

	ctx := context.Background()
	resp, err := client.Complete(ctx, "Hello")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if resp != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got %q", resp)
	}
}

func TestZAIClient_CompleteWithStructured_RetryAndBackoff(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Simulate rate limit
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "{\"success\": true}"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewZAIClient("test-key")
	client.baseURL = server.URL

	// Speed up retries by overriding client's retry delays
	client.retryBackoffBase = 1 * time.Millisecond
	client.retryBackoffMax = 5 * time.Millisecond

	ctx := context.Background()
	resp, err := client.CompleteWithStructuredOutput(ctx, "sys", "user", false)
	if err != nil {
		t.Fatalf("CompleteWithStructuredOutput failed: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts (2 retries), got %d", attempts)
	}
	if resp != `{"success": true}` {
		t.Errorf("Unexpected response: %s", resp)
	}
}

func TestZAIClient_SetModel(t *testing.T) {
	client := NewZAIClient("test-key")

	// Default model should be set
	if client.GetModel() == "" {
		t.Error("Expected default model to be set")
	}

	// SetModel should change the model
	client.SetModel("glm-4.9")
	if client.GetModel() != "glm-4.9" {
		t.Errorf("Expected model glm-4.9, got %s", client.GetModel())
	}
}

func TestZAIClient_DisableSemaphore(t *testing.T) {
	client := NewZAIClient("test-key")

	// Initially semaphore should be enabled (not nil)
	if client.semDisabled {
		t.Error("Expected semaphore to be enabled initially")
	}

	// Disable semaphore
	client.DisableSemaphore()
	if !client.semDisabled {
		t.Error("Expected semaphore to be disabled after call")
	}
	if client.sem != nil {
		t.Error("Expected sem to be nil after DisableSemaphore")
	}
}

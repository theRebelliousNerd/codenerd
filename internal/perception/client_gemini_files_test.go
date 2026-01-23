package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGeminiClient_UploadFile(t *testing.T) {
	// Mock server for Resumable Upload Protocol
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Initial Request
		if r.Method == "POST" && r.URL.Path == "/upload/v1beta/files" {
			if r.Header.Get("X-Goog-Upload-Protocol") != "resumable" {
				t.Errorf("Expected resumable protocol header")
			}
			// Return upload URL
			w.Header().Set("X-Goog-Upload-URL", "http://"+r.Host+"/upload_session")
			w.WriteHeader(http.StatusOK)
			return
		}

		// 2. Upload Bytes
		if r.Method == "POST" && r.URL.Path == "/upload_session" {
			if r.Header.Get("X-Goog-Upload-Command") != "upload, finalize" {
				t.Errorf("Expected upload command")
			}
			// Return success JSON
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"file": {"uri": "files/123456789"}}`))
			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	client := &GeminiClient{
		apiKey:     "test-key",
		baseURL:    ts.URL + "/v1beta",
		httpClient: ts.Client(),
	}

	// Create dummy file
	tmpFile := t.TempDir() + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	uri, err := client.UploadFile(context.Background(), tmpFile, "text/plain")
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if uri != "files/123456789" {
		t.Errorf("Expected URI 'files/123456789', got %s", uri)
	}
}

func TestGeminiClient_CreateCachedContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/cachedContents" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name": "cachedContents/abcdef", "model": "models/gemini-pro"}`))
	}))
	defer ts.Close()

	client := &GeminiClient{
		apiKey:     "test-key",
		baseURL:    ts.URL + "/v1beta",
		httpClient: ts.Client(),
		model:      "gemini-pro",
	}

	name, err := client.CreateCachedContent(context.Background(), []string{"files/123"}, 300)
	if err != nil {
		t.Fatalf("CreateCachedContent failed: %v", err)
	}
	if name != "cachedContents/abcdef" {
		t.Errorf("Expected cache name 'cachedContents/abcdef', got %s", name)
	}
}

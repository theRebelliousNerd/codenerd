package research

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// MockTransport allows intercepting requests for testing
type MockTransport struct {
	Handlers map[string]func(*http.Request) (*http.Response, error)
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	for prefix, handler := range m.Handlers {
		if strings.HasPrefix(url, prefix) {
			return handler(req)
		}
	}
	// Debug output for troubleshooting
	// fmt.Printf("MockTransport 404 for: %s\n", url)
	// println("MockTransport 404 for:", url)

	// Default 404
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		Handlers: make(map[string]func(*http.Request) (*http.Response, error)),
	}
}

func (m *MockTransport) RegisterResponder(urlPrefix string, body string, status int) {
	m.Handlers[urlPrefix] = func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Header:     make(http.Header),
		}, nil
	}
}

func TestExecuteContext7_Success_LlmsTxt(t *testing.T) {
	mock := NewMockTransport()

	// Mock llms.txt - Note: Code expects "- Path: Title" format
	mock.RegisterResponder("https://raw.githubusercontent.com/owner/repo/main/llms.txt",
		"- docs/intro.md: Introduction", 200)

	// Mock referenced doc - needs > 50 chars to pass validation
	mock.RegisterResponder("https://raw.githubusercontent.com/owner/repo/main/docs/intro.md",
		"# Introduction\nHello World. This is a longer document to satisfy the length requirement of the parser. It needs to be at least 50 characters long.", 200)

	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mock
	defer func() { http.DefaultClient.Transport = oldTransport }()

	tool := Context7Tool()
	res, err := tool.Execute(context.Background(), map[string]any{
		"topic": "test-topic",
		"repo":  "owner/repo",
	})
	if err != nil {
		t.Fatalf("executeContext7 failed: %v", err)
	}

	if !strings.Contains(res, "# Documentation for test-topic") {
		t.Errorf("Expected header, got: %s", res)
	}
	if !strings.Contains(res, "# Introduction") {
		t.Errorf("Expected doc content, got: %s", res)
	}
}

func TestExecuteContext7_InferRepo(t *testing.T) {
	// Test internal logic of inferRepo via execute (integration) or direct function call if exported
	// inferRepo is private (lowercase), so we test it via Execute or helper test

	mock := NewMockTransport()
	// Mangle maps to google/mangle
	mock.RegisterResponder("https://raw.githubusercontent.com/google/mangle/main/llms.txt",
		"- docs/intro.md: Introduction", 200)
	// Mock content > 50 chars
	mock.RegisterResponder("https://raw.githubusercontent.com/google/mangle/main/docs/intro.md",
		"# Mangle Intro\ncontent that is sufficiently long to pass the 50 character limit imposed by the context7 parser. Otherwise it gets ignored.", 200)

	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mock
	defer func() { http.DefaultClient.Transport = oldTransport }()

	tool := Context7Tool()
	res, err := tool.Execute(context.Background(), map[string]any{
		"topic": "mangle",
	})
	if err != nil {
		t.Fatalf("executeContext7 failed: %v", err)
	}

	// If it tried to fetch google/mangle, it hit our mock.
	// If it didn't infer, it would fail or try something else.
	if !strings.Contains(res, "google/mangle") && !strings.Contains(res, "Documentation for mangle") {
		// Note: The output might not explicitly say google/mangle if it just dumps content.
		// But if content is "content", we look for that.
		if !strings.Contains(res, "content") {
			t.Errorf("Expected content from inferred repo, got: %s", res)
		}
	}
}

func TestInferRepo_Logic(t *testing.T) {
	// Since inferRepo is in the same package, we can test it directly here
	tests := []struct {
		topic    string
		expected string
	}{
		{"react", "facebook/react"},
		{"go-rod", "go-rod/rod"},
		{"owner/repo", "owner/repo"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		got := inferRepo(tt.topic)
		if got != tt.expected {
			t.Errorf("inferRepo(%q) = %q, want %q", tt.topic, got, tt.expected)
		}
	}
}

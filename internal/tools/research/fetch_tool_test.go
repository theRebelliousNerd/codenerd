package research

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchTool_Execute_Success(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><body><h1>Hello World</h1><p>Test content.</p></body></html>`)
	}))
	defer ts.Close()

	tool := WebFetchTool()
	args := map[string]any{
		"url": ts.URL,
	}

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("executeWebFetch failed: %v", err)
	}

	if !strings.Contains(result, "# Hello World") {
		t.Errorf("Expected markdown header, got: %s", result)
	}
	if !strings.Contains(result, "Test content") {
		t.Errorf("Expected content, got: %s", result)
	}
}

func TestWebFetchTool_Execute_PlainText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, `Just plain text.`)
	}))
	defer ts.Close()

	tool := WebFetchTool()
	result, err := tool.Execute(context.Background(), map[string]any{"url": ts.URL})
	if err != nil {
		t.Fatalf("executeWebFetch failed: %v", err)
	}

	if !strings.Contains(result, "Just plain text") {
		t.Errorf("Expected plain text, got: %s", result)
	}
}

func TestWebFetchTool_Execute_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	tool := WebFetchTool()
	_, err := tool.Execute(context.Background(), map[string]any{"url": ts.URL})
	if err == nil {
		t.Error("Expected error for 404")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("Expected 404 error, got: %v", err)
	}
}

func TestHtmlToMarkdown_Conversion(t *testing.T) {
	html := `
		<html>
			<head><title>Page Title</title></head>
			<body>
				<h1>Header 1</h1>
				<p>Paragraph with <a href="http://example.com">link</a>.</p>
				<ul>
					<li>Item 1</li>
					<li>Item 2</li>
				</ul>
			</body>
		</html>`

	md, err := htmlToMarkdown(html, "http://base.com", true)
	if err != nil {
		t.Fatalf("htmlToMarkdown failed: %v", err)
	}

	expectedParts := []string{
		"# Page Title",
		"# Header 1",
		"Paragraph with [link ](http://example.com).", // Note: converter adds space
		"- Item 1",
		"- Item 2",
	}

	for _, part := range expectedParts {
		if !strings.Contains(md, part) {
			t.Errorf("Markdown missing expected part: %q\nGot:\n%s", part, md)
		}
	}
}

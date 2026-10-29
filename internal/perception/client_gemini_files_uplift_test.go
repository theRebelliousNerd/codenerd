package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The resumable upload protocol end to end: session start with the right
// headers, raw bytes to the session URL, and the file URI back.
func TestGeminiUploadFile_ResumableProtocol(t *testing.T) {
	var gotSessionHeaders http.Header
	var gotBytes []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/files" &&
			r.Header.Get("X-Goog-Upload-Command") == "start":
			gotSessionHeaders = r.Header.Clone()
			w.Header().Set("X-Goog-Upload-URL", "http://"+r.Host+"/upload/session-1")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/upload/session-1":
			gotBytes, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"file":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/abc","name":"files/abc"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(path, []byte("hello files"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri, err := geminiTestClient(srv.URL).UploadFile(context.Background(), path, "text/plain")
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if uri != "https://generativelanguage.googleapis.com/v1beta/files/abc" {
		t.Errorf("uri = %q, want the file URI", uri)
	}
	if got := gotSessionHeaders.Get("X-Goog-Upload-Protocol"); got != "resumable" {
		t.Errorf("upload protocol = %q, want resumable", got)
	}
	if got := gotSessionHeaders.Get("X-Goog-Upload-Header-Content-Length"); got != "11" {
		t.Errorf("upload content length = %q, want 11", got)
	}
	if got := gotSessionHeaders.Get("X-Goog-Upload-Header-Content-Type"); got != "text/plain" {
		t.Errorf("upload content type = %q, want text/plain", got)
	}
	if string(gotBytes) != "hello files" {
		t.Errorf("uploaded bytes = %q, want the file content", gotBytes)
	}
}

// GetFile accepts resource names, bare IDs, and the full URIs UploadFile
// returns — and honors the client's base URL instead of hardcoded prod.
func TestGeminiGetFile_FormsAndBaseURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"files/abc","displayName":"f","mimeType":"text/plain"}`))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	ctx := context.Background()
	for _, input := range []string{
		"files/abc",
		"abc",
		"https://generativelanguage.googleapis.com/v1beta/files/abc",
	} {
		if _, err := c.GetFile(ctx, input); err != nil {
			t.Errorf("GetFile(%q): %v", input, err)
			continue
		}
		if gotPath != "/files/abc" {
			t.Errorf("GetFile(%q) hit %q, want /files/abc", input, gotPath)
		}
	}
}

// DeleteFile normalizes URIs to resource names: the production caller
// passes UploadFile's URI, which used to become "files/https://..." and
// 404 while the error was ignored — leaking a file per strategic-doc run.
func TestGeminiDeleteFile_NormalizesURI(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	err := geminiTestClient(srv.URL).DeleteFile(context.Background(),
		"https://generativelanguage.googleapis.com/v1beta/files/abc")
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/files/abc" {
		t.Errorf("request = %s %s, want DELETE /files/abc", gotMethod, gotPath)
	}
}

// The cache admin methods honor the client's base URL (they hardcoded
// prod, which made them untestable and proxy-breaking).
func TestGeminiCacheAdmin_UsesBaseURL(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"cachedContents/xyz","cachedContents":[{"name":"cachedContents/xyz"}]}`))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	ctx := context.Background()
	if _, err := c.GetCachedContent(ctx, "cachedContents/xyz"); err != nil {
		t.Fatalf("GetCachedContent: %v", err)
	}
	if err := c.DeleteCachedContent(ctx, "cachedContents/xyz"); err != nil {
		t.Fatalf("DeleteCachedContent: %v", err)
	}
	names, err := c.ListCachedContent(ctx)
	if err != nil {
		t.Fatalf("ListCachedContent: %v", err)
	}
	if len(names) != 1 || names[0] != "cachedContents/xyz" {
		t.Errorf("cached contents = %v, want [cachedContents/xyz]", names)
	}
	want := []string{"GET /cachedContents/xyz", "DELETE /cachedContents/xyz", "GET /cachedContents"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

// Creating a cache normalizes the model to the API's models/ form,
// formats the TTL, and references the uploaded files.
func TestGeminiCreateCachedContent_RequestShape(t *testing.T) {
	captured := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("request is not JSON: %v", err)
		}
		captured <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"cachedContents/xyz"}`))
	}))
	defer srv.Close()

	name, err := geminiTestClient(srv.URL).CreateCachedContent(context.Background(),
		[]string{"https://generativelanguage.googleapis.com/v1beta/files/abc"}, 300)
	if err != nil {
		t.Fatalf("CreateCachedContent: %v", err)
	}
	if name != "cachedContents/xyz" {
		t.Errorf("name = %q, want cachedContents/xyz", name)
	}
	req := <-captured
	if req["model"] != "models/gemini-2.5-flash" {
		t.Errorf("model = %v, want the normalized models/ form", req["model"])
	}
	if req["ttl"] != "300s" {
		t.Errorf("ttl = %v, want 300s", req["ttl"])
	}
	contents, _ := req["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents = %v, want one file-bearing content", req["contents"])
	}
	parts, _ := contents[0].(map[string]any)["parts"].([]any)
	fileData, _ := parts[0].(map[string]any)["fileData"].(map[string]any)
	if fileData["fileUri"] != "https://generativelanguage.googleapis.com/v1beta/files/abc" {
		t.Errorf("fileUri = %v, want the uploaded file", fileData["fileUri"])
	}
}

// ListFiles returns the URIs the API reports.
func TestGeminiListFiles_ParsesURIs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":[{"uri":"u1"},{"uri":"u2"}]}`))
	}))
	defer srv.Close()

	uris, err := geminiTestClient(srv.URL).ListFiles(context.Background())
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if strings.Join(uris, ",") != "u1,u2" {
		t.Errorf("uris = %v, want [u1 u2]", uris)
	}
}

package sessionlog

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const secretKey = "sk-ant-SECRET-0123456789"

func TestWireCaptureRecordsTheExactRequestAndResponseWithoutCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"claude-x"`) {
			t.Errorf("the server must still receive the body, got %q", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=topsecret")
		_, _ = w.Write([]byte(`{"id":"msg_1","content":[{"type":"text","text":"line a\nline b"}],"usage":{"input_tokens":3}}`))
	}))
	defer srv.Close()

	s, _ := installStore(t, "session-1", true)
	client := WrapClient(&http.Client{})
	ctx, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session", Provider: "anthropic"})

	reqBody := `{"model":"claude-x","system":[{"type":"text","text":"sys1\nsys2","cache_control":{"type":"ephemeral"}}],"max_tokens":16}`
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/v1/messages?key="+secretKey+"&beta=1", strings.NewReader(reqBody))
	req.Header.Set("x-api-key", secretKey)
	req.Header.Set("Authorization", "Bearer "+secretKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(got), "line a") {
		t.Fatalf("the caller must still get the response, got %q", got)
	}
	call.End(Result{})

	var ex Exchange
	path := sessionPath(s, "session-1", "chat", "0001-session.json")
	readJSON(t, path, &ex)
	if len(ex.Wire) != 1 {
		t.Fatalf("wire = %d round trips, want 1", len(ex.Wire))
	}
	w := ex.Wire[0]
	if w.Attempt != 1 || w.Transport != "http" || w.Request.Method != "POST" {
		t.Fatalf("wire header: %+v", w)
	}
	if strings.Contains(w.Request.URL, secretKey) || !strings.Contains(w.Request.URL, "beta=1") {
		t.Fatalf("URL key must be redacted and the rest kept: %s", w.Request.URL)
	}
	if w.Request.Headers["X-Api-Key"][0] != Redacted || w.Request.Headers["Authorization"][0] != Redacted {
		t.Fatalf("credential headers must be redacted: %v", w.Request.Headers)
	}
	if w.Request.Headers["Anthropic-Version"][0] != "2023-06-01" {
		t.Fatalf("other headers must be kept: %v", w.Request.Headers)
	}
	if w.Response.Headers["Set-Cookie"][0] != Redacted {
		t.Fatalf("response cookies must be redacted: %v", w.Response.Headers)
	}
	raw := readFile(t, path)
	if strings.Contains(raw, secretKey) || strings.Contains(raw, "topsecret") {
		t.Fatalf("a credential reached the exchange file:\n%s", raw)
	}

	// The body round-trips to the bytes that were sent.
	tree, _ := DecodeOrdered([]byte(readJSONBody(t, raw, "request")))
	restored, err := RestoreTree(tree, w.Request.LinesPaths)
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := marshalPlain(restored); string(out) != reqBody {
		t.Fatalf("request body not restored exactly\n got: %s\nwant: %s", out, reqBody)
	}
	if w.Response.Status != 200 || w.Response.BodyBytes == 0 {
		t.Fatalf("response: %+v", w.Response)
	}
	respTree, _ := DecodeOrdered([]byte(readJSONBody(t, raw, "response")))
	respBack, err := RestoreTree(respTree, w.Response.LinesPaths)
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := marshalPlain(respBack); !strings.Contains(string(out), `"text":"line a\nline b"`) {
		t.Fatalf("response body not restored: %s", out)
	}
}

// readJSONBody extracts the "body" of wire[0].request or .response from file
// text, as JSON.
func readJSONBody(t *testing.T, file, which string) string {
	t.Helper()
	tree, err := DecodeOrdered([]byte(file))
	if err != nil {
		t.Fatal(err)
	}
	wire := tree.(*Object).Values["wire"].([]any)[0].(*Object)
	section := wire.Values[which].(*Object)
	out, err := marshalPlain(section.Values["body"])
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestWireCaptureRecordsEveryRetryAndTheTransportError(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	s, _ := installStore(t, "session-1", true)
	client := WrapClient(&http.Client{})
	ctx, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader(`{"n":1}`))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	dead, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1/", nil)
	_, _ = client.Do(dead)
	call.End(Result{})
	var ex Exchange
	readJSON(t, sessionPath(s, "session-1", "chat", "0001-session.json"), &ex)
	if len(ex.Wire) != 3 || ex.Wire[0].Response.Status != 429 || ex.Wire[1].Response.Status != 200 || ex.Wire[2].Error == "" {
		t.Fatalf("wire = %+v", ex.Wire)
	}
	for i, w := range ex.Wire {
		if w.Attempt != i+1 {
			t.Errorf("attempt %d numbered %d", i+1, w.Attempt)
		}
	}
	if ex.LogicalRequest != nil {
		t.Error("a call with wire capture must not also carry the logical request")
	}
}

func TestEventStreamBodyIsNamedNotSilentlyDropped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"delta\":\"hi\"}\n\n"))
	}))
	defer srv.Close()
	s, _ := installStore(t, "session-1", true)
	client := WrapClient(&http.Client{})
	ctx, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "hi") {
		t.Fatalf("the stream must reach the caller untouched, got %q", body)
	}
	call.End(Result{Response: &Response{Text: []string{"hi"}, Streamed: true}})
	var ex Exchange
	readJSON(t, sessionPath(s, "session-1", "chat", "0001-session.json"), &ex)
	if got := ex.Wire[0].Response; got.Body != nil || !strings.Contains(got.BodyNote, "event stream") {
		t.Fatalf("a streamed body must be named as omitted, got %+v", got)
	}
}

func TestBinaryBodyIsNamedNotSilentlyDropped(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	client := WrapClient(&http.Client{})
	ctx, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "upload"})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, strings.NewReader("\xff\xfe\x00\x01"))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	call.End(Result{})
	var ex Exchange
	readJSON(t, sessionPath(s, "session-1", "chat", "0001-upload.json"), &ex)
	if r := ex.Wire[0].Request; r.Body != nil || !strings.Contains(r.BodyNote, "binary") || r.BodyBytes != 4 {
		t.Fatalf("binary body must be named with its size: %+v", r)
	}
}

func TestNoCapturePassesThroughUntouched(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	}))
	defer srv.Close()
	client := WrapClient(&http.Client{})
	resp, err := client.Post(srv.URL, "text/plain", strings.NewReader("echo"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if b, _ := io.ReadAll(resp.Body); string(b) != "echo" {
		t.Fatalf("got %q", b)
	}
}

func TestWrapClientDoesNotMutateTheCallersClient(t *testing.T) {
	orig := &http.Client{Timeout: time.Second}
	wrapped := WrapClient(orig)
	if orig.Transport != nil {
		t.Fatal("WrapClient must not change the client it was given")
	}
	if wrapped.Timeout != time.Second {
		t.Fatal("WrapClient must keep the client's settings")
	}
	if WrapClient(wrapped) != wrapped {
		t.Fatal("wrapping twice must be a no-op")
	}
}

func TestExecRunRecordsArgvStdinAndOutputButNotTheEnvironment(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	ctx, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session", Provider: "claude-cli"})
	RecordExec(ctx, ExecRun{
		Command: "claude", Args: []string{"-p", "--system-prompt", "be brief\nand kind"},
		Stdin: "question\nline two", Stdout: `{"result":"ok"}`, Stderr: "warn", ExitCode: 0,
		Started: time.Now(), Duration: 5 * time.Millisecond,
	})
	RecordExec(ctx, ExecRun{Command: "claude", Err: errors.New("exit status 1"), ExitCode: 1})
	call.End(Result{})
	var ex Exchange
	path := sessionPath(s, "session-1", "chat", "0001-session.json")
	readJSON(t, path, &ex)
	if len(ex.Wire) != 2 || ex.Wire[0].Transport != "exec" || ex.Wire[0].Request.URL != "claude" {
		t.Fatalf("wire = %+v", ex.Wire)
	}
	raw := readFile(t, path)
	tree, _ := DecodeOrdered([]byte(readJSONBody(t, raw, "request")))
	restored, err := RestoreTree(tree, ex.Wire[0].Request.LinesPaths)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := marshalPlain(restored)
	if !strings.Contains(string(out), `"stdin":"question\nline two"`) || !strings.Contains(string(out), `"be brief\nand kind"`) {
		t.Fatalf("argv/stdin not restored: %s", out)
	}
	if ex.Wire[1].Error != "exit status 1" || ex.Wire[1].Response.Status != 1 {
		t.Fatalf("a failed run must carry its error and exit code: %+v", ex.Wire[1])
	}
	// No capture on ctx: nothing recorded, nothing panics.
	RecordExec(context.Background(), ExecRun{Command: "x"})
}

func TestExitCodeOfACommandThatNeverRan(t *testing.T) {
	if ExitCode(nil) != -1 || ExitCode(exec.Command("true")) != -1 {
		t.Fatal("a command that never ran has exit code -1")
	}
}

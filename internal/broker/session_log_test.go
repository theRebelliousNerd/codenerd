package broker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

type loggedBroker struct {
	client    types.LLMClient
	workspace string
	debug     *atomic.Bool
}

func newLoggedBroker(t *testing.T, underlying types.LLMClient, window int) loggedBroker {
	t.Helper()
	ws := t.TempDir()
	var debug atomic.Bool
	debug.Store(true)
	sessionlog.Install(sessionlog.NewStore(sessionlog.Options{Workspace: ws, Session: "session-1", Enabled: debug.Load}))
	t.Cleanup(func() { sessionlog.Install(nil) })
	meter := testMeter(window, 100)
	wrapped, err := Wrap(underlying, meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"}))
	if err != nil {
		t.Fatal(err)
	}
	return loggedBroker{client: wrapped, workspace: ws, debug: &debug}
}

func (b loggedBroker) file(t *testing.T, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{b.workspace, ".nerd", "sessions", "session-1"}, parts...)...)
}

func (b loggedBroker) exchange(t *testing.T, folder, name string) sessionlog.Exchange {
	t.Helper()
	data, err := os.ReadFile(b.file(t, folder, name))
	if err != nil {
		entries, _ := os.ReadDir(b.file(t, folder))
		t.Fatalf("%s/%s: %v (folder holds %v)", folder, name, err, entries)
	}
	var ex sessionlog.Exchange
	if err := json.Unmarshal(data, &ex); err != nil {
		t.Fatalf("%s is not valid JSON: %v", name, err)
	}
	return ex
}

func (b loggedBroker) count(t *testing.T, folder string) int {
	t.Helper()
	entries, _ := os.ReadDir(b.file(t, folder))
	return len(entries)
}

func TestEveryMethodWritesAnExchangeFileWithResponseAndUsage(t *testing.T) {
	all := newFakeAll()
	all.respText = "answer one\nanswer two"
	b := newLoggedBroker(t, all, 100000)
	ctx := WithPurpose(sessionlog.WithChat(context.Background()), PurposeSession)

	if _, err := b.client.CompleteWithSystem(ctx, "sys\nprompt", "question"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.client.CompleteWithTools(ctx, "sys", "q", []types.ToolDefinition{{Name: "read_file"}}); err != nil {
		t.Fatal(err)
	}
	tr := b.client.(types.ToolResultsProvider)
	history := []types.Message{{Role: "user", Text: "go"}, {Role: "assistant", Text: "calling", ToolCalls: []types.ToolCall{{ID: "c1", Name: "read_file", Input: map[string]any{"path": "a.go"}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "c1", Content: "package a\nfunc A() {}"}}}}
	if _, err := tr.CompleteWithToolResults(ctx, "sys", history, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.client.Complete(ctx, "bare prompt"); err != nil {
		t.Fatal(err)
	}

	if n := b.count(t, "chat"); n != 4 {
		t.Fatalf("chat/ holds %d files, want 4", n)
	}
	ex := b.exchange(t, "chat", "0001-session.json")
	if ex.Method != "CompleteWithSystem" || ex.Provider != "fake" || ex.Model != "fake-model" || ex.Outcome != sessionlog.OutcomeOK {
		t.Fatalf("header: %+v", ex)
	}
	if sessionlog.Join(ex.Response.Text) != "answer one\nanswer two" {
		t.Fatalf("response: %+v", ex.Response)
	}
	if ex.Usage.InputTokens != 100 || ex.Usage.OutputTokens != 50 || ex.Usage.Status != "reported" || ex.Usage.Price == nil {
		t.Fatalf("usage: %+v", ex.Usage)
	}
	if ex.Usage.Price.Priced {
		t.Fatalf("an unpriced model must say so, not claim a cost: %+v", ex.Usage.Price)
	}
	if ex.Admission == nil {
		t.Fatal("the admission decision (estimate, window, headroom) belongs in the file")
	}
	if sessionlog.Join(ex.LogicalRequest.System) != "sys\nprompt" || sessionlog.Join(ex.LogicalRequest.User) != "question" {
		t.Fatalf("logical request: %+v", ex.LogicalRequest)
	}
	third := b.exchange(t, "chat", "0003-session.json")
	if third.Method != "CompleteWithToolResults" || len(third.LogicalRequest.Messages) != 3 {
		t.Fatalf("tool-results exchange: %+v", third)
	}
	if got := third.LogicalRequest.Messages[2].ToolResults; got == nil {
		t.Fatal("tool results sent to the model must be in the file")
	}
}

func TestDebugOffWritesNoExchangeFilesButTheAgentKeepsItsTranscript(t *testing.T) {
	all := newFakeAll()
	all.respText = "the answer"
	b := newLoggedBroker(t, all, 100000)
	b.debug.Store(false)

	ctx := sessionlog.WithAgent(WithPurpose(context.Background(), PurposeSubagent), "coder", "coder-7")
	if _, err := b.client.CompleteWithSystem(ctx, "sys", "q"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b.file(t, "coder-7", "0001-subagent.json")); !os.IsNotExist(err) {
		t.Fatalf("debug off must write no exchange file: %v", err)
	}
	data, err := os.ReadFile(b.file(t, "coder-7", sessionlog.TranscriptFile))
	if err != nil {
		t.Fatalf("the transcript is written whether or not debug is on: %v", err)
	}
	var doc struct {
		Events []struct {
			Kind  string   `json:"kind"`
			Model string   `json:"model"`
			Text  []string `json:"text"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || len(doc.Events) != 1 {
		t.Fatalf("transcript: %v %s", err, data)
	}
	if e := doc.Events[0]; e.Kind != sessionlog.EventAssistant || e.Model != "fake-model" || sessionlog.Join(e.Text) != "the answer" {
		t.Fatalf("assistant event: %+v", e)
	}
}

func TestStreamedCallsAreAssembledIntoTheirFinalForm(t *testing.T) {
	all := newFakeAll()
	all.streamChunks = []string{"hel", "lo\nwor", "ld"}
	b := newLoggedBroker(t, all, 100000)
	ctx := sessionlog.WithChat(context.Background())

	content, done := b.client.CompleteWithStreaming(ctx, "sys", "q", false)
	var got strings.Builder
	for chunk := range content {
		got.WriteString(chunk)
	}
	res := <-done
	if got.String() != "hello\nworld" || res.Err != nil {
		t.Fatalf("the caller must still receive the stream untouched: %q %v", got.String(), res.Err)
	}

	ex := b.exchange(t, "chat", "0001-unattributed.json")
	if !ex.Response.Streamed || sessionlog.Join(ex.Response.Text) != "hello\nworld" || ex.Response.StopReason != "end_turn" {
		t.Fatalf("assembled stream: %+v", ex.Response)
	}
	if ex.Usage.InputTokens != 100 {
		t.Fatalf("a stream's usage must be settled into the file: %+v", ex.Usage)
	}

	// The thoughts stream keeps its reasoning apart from the answer.
	th := b.client.(types.ThoughtStreamer)
	c2, t2, d2 := th.CompleteWithStreamingAndThoughts(ctx, "sys", "q", true)
	for range c2 {
	}
	for range t2 {
	}
	<-d2
	ex = b.exchange(t, "chat", "0002-unattributed.json")
	if sessionlog.Join(ex.Response.Thoughts) != "thinking" || sessionlog.Join(ex.Response.Text) != "ok" {
		t.Fatalf("thought stream: %+v", ex.Response)
	}
}

func TestARefusedRequestLeavesAFileSayingWhy(t *testing.T) {
	b := newLoggedBroker(t, newFakeAll(), 10) // a window nothing fits
	_, err := b.client.CompleteWithSystem(WithPurpose(sessionlog.WithChat(context.Background()), PurposeSession), strings.Repeat("x ", 5000), "q")
	if _, refused := IsAdmissionError(err); !refused {
		t.Fatalf("expected a broker refusal, got %v", err)
	}
	ex := b.exchange(t, "chat", "0001-session.json")
	if ex.Outcome != sessionlog.OutcomeRefused || !strings.Contains(ex.Error, "refused") || ex.Admission == nil {
		t.Fatalf("refusal: %+v", ex)
	}
	if len(ex.Wire) != 0 || ex.LogicalRequest == nil {
		t.Fatal("a refused request reached no provider: no wire, only the logical request")
	}
}

func TestAProviderErrorIsRecordedAndStillReturned(t *testing.T) {
	all := newFakeAll()
	all.respErr = io.ErrUnexpectedEOF
	b := newLoggedBroker(t, all, 100000)
	if _, err := b.client.CompleteWithSystem(sessionlog.WithChat(context.Background()), "s", "q"); err != io.ErrUnexpectedEOF {
		t.Fatalf("the call's own error must reach the caller, got %v", err)
	}
	ex := b.exchange(t, "chat", "0001-unattributed.json")
	if ex.Outcome != sessionlog.OutcomeError || !strings.Contains(ex.Error, "unexpected EOF") {
		t.Fatalf("error exchange: %+v", ex)
	}
}

// httpClient is a client that makes a real HTTP call through the wire
// transport, as every provider client does.
type httpClient struct {
	*fakeClient
	url string
	hc  *http.Client
}

func (c *httpClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, strings.NewReader(`{"system":"`+sys+`","prompt":"`+user+`"}`))
	req.Header.Set("Authorization", "Bearer sk-live-0123456789")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.report(ctx)
	return string(body), nil
}

func TestTheWireRequestAndResponseOfTheProviderCallAreCapturedWithoutCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reply":"pong"}`))
	}))
	defer srv.Close()
	client := &httpClient{fakeClient: newFakeClient(), url: srv.URL, hc: sessionlog.WrapClient(&http.Client{})}
	b := newLoggedBroker(t, client, 100000)

	if _, err := b.client.CompleteWithSystem(sessionlog.WithChat(context.Background()), "be brief", "ping"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.file(t, "chat", "0001-unattributed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sk-live-0123456789") {
		t.Fatalf("a credential reached the exchange file:\n%s", raw)
	}
	var ex sessionlog.Exchange
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatal(err)
	}
	if len(ex.Wire) != 1 || ex.Wire[0].Request.Headers["Authorization"][0] != sessionlog.Redacted || ex.Wire[0].Response.Status != 200 {
		t.Fatalf("wire: %+v", ex.Wire)
	}
	if ex.LogicalRequest != nil {
		t.Fatal("with the wire captured the logical request is redundant and must be left out")
	}
}

func TestExchangeLoggingIsRaceFreeUnderConcurrentCalls(t *testing.T) {
	b := newLoggedBroker(t, newFakeAll(), 1000000)
	done := make(chan struct{}, 40)
	for i := 0; i < 40; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			ctx := WithPurpose(sessionlog.WithAgent(context.Background(), "coder", "coder-1"), PurposeSubagent)
			_, _ = b.client.CompleteWithSystem(ctx, "s", "q")
		}()
	}
	for i := 0; i < 40; i++ {
		<-done
	}
	if n := b.count(t, "coder-1"); n != 41 { // 40 exchanges + transcript.json
		t.Fatalf("coder-1/ holds %d files, want 40 exchanges and a transcript", n)
	}
	_ = usage.Report{}
}

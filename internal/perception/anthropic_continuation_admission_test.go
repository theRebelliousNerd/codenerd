package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

const continuationModel = "claude-opus-5-5"

// pausedFetchReply is a turn the endpoint paused after a web_fetch returned a
// page of pageChars characters of plain text.
func pausedFetchReply(pageChars int) string {
	page := strings.Repeat("the fetched page says things ", pageChars/29+1)[:pageChars]
	return fmt.Sprintf(`{"type":"message","role":"assistant","model":%q,"stop_reason":"pause_turn",
"usage":{"input_tokens":10,"output_tokens":5,"server_tool_use":{"web_fetch_requests":1}},
"content":[{"type":"server_tool_use","id":"srvtoolu_1","name":"web_fetch","input":{"url":"https://example.com/a"}},
{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_1","content":{"type":"web_fetch_result","url":"https://example.com/a","content":{"type":"document","source":{"type":"text","media_type":"text/plain","data":%q}}}}]}`,
		continuationModel, page)
}

const continuationDone = `{"type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"end_turn",
"usage":{"input_tokens":30,"output_tokens":7},"content":[{"type":"text","text":"answer"}]}`

func newContinuationClient(t *testing.T, base string, window int, options config.AnthropicProviderConfig) *AnthropicClient {
	t.Helper()
	c := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "test-key", BaseURL: base, Model: continuationModel, MaxInputTokens: window, Options: options})
	if c.configErr != nil {
		t.Fatalf("client config: %v", c.configErr)
	}
	return c
}

func (ws *webToolServer) sent() int {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return len(ws.requests)
}

// resumptionTokens is what admission counts for the first resumption of a
// paused reply: the neutral transcript with the paused content as the
// assistant's turn, plus the server tool declarations.
func resumptionTokens(t *testing.T, c *AnthropicClient, system, user, pausedReply string) int {
	t.Helper()
	var paused AnthropicResponse
	if err := json.Unmarshal([]byte(pausedReply), &paused); err != nil {
		t.Fatal(err)
	}
	history := append(userTurn(user), types.NewAssistantMessage(blocksFromAnthropicContent(paused.Content)...))
	tokens, err := prompt.EstimateRequestTokens(system, history, nil)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := serverDeclarationTokens(c.serverTools(continuationModel))
	if err != nil {
		t.Fatal(err)
	}
	return tokens + declared
}

// A resumption is a larger request than the one that was admitted: the paused
// content carries the fetched page. Under a window the first request fits and
// the resumption does not, the first is the only request the endpoint sees and
// the refusal says which request was not sent.
func TestAnthropicExchange_ResumptionIsAdmittedAgainstTheWindow(t *testing.T) {
	ws := newWebToolServer(t, pausedFetchReply(40_000), continuationDone)
	c := newContinuationClient(t, ws.server.URL, 2_000, config.AnthropicProviderConfig{WebFetch: true})
	_, err := c.CompleteWithSystem(context.Background(), "sys", "q")
	if !errors.Is(err, ErrInputWindowExceeded) {
		t.Fatalf("err = %v, want ErrInputWindowExceeded for the resumption", err)
	}
	if !strings.Contains(err.Error(), "continuation was not sent") {
		t.Errorf("the refusal does not say the continuation was not sent: %v", err)
	}
	if got := ws.sent(); got != 1 {
		t.Fatalf("the endpoint saw %d requests, want only the first", got)
	}
}

// A resumption that exactly fits is sent: the contract is the window, not a
// margin, and the same measure admits the first request and its continuation.
// One token less window refuses it.
func TestAnthropicExchange_BoundaryFittingResumptionIsSent(t *testing.T) {
	reply := pausedFetchReply(4_000)
	probe := newContinuationClient(t, "http://unused.invalid", 0, config.AnthropicProviderConfig{WebFetch: true})
	need := resumptionTokens(t, probe, "sys", "q", reply)

	fits := newWebToolServer(t, reply, continuationDone)
	c := newContinuationClient(t, fits.server.URL, need, config.AnthropicProviderConfig{WebFetch: true})
	text, err := c.CompleteWithSystem(context.Background(), "sys", "q")
	if err != nil {
		t.Fatalf("a resumption of exactly %d tokens under a window of %d was refused: %v", need, need, err)
	}
	if text != "answer" || fits.sent() != 2 {
		t.Fatalf("text = %q after %d requests, want the resumed answer after 2", text, fits.sent())
	}

	over := newWebToolServer(t, reply, continuationDone)
	c = newContinuationClient(t, over.server.URL, need-1, config.AnthropicProviderConfig{WebFetch: true})
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "q"); !errors.Is(err, ErrInputWindowExceeded) {
		t.Fatalf("err = %v, want a resumption one token over the window refused", err)
	}
	if got := over.sent(); got != 1 {
		t.Errorf("the endpoint saw %d requests, want 1", got)
	}
}

// cancelAfterFirstResponse makes the client's transport cancel ctx once the
// first response has been read and closed, so the cancellation arrives between
// the paused response and its resumption.
type cancelAfterFirstResponse struct {
	base   http.RoundTripper
	cancel context.CancelFunc
	once   sync.Once
}

type cancelOnClose struct {
	io.ReadCloser
	fire func()
}

func (b cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.fire()
	return err
}

func (r *cancelAfterFirstResponse) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = cancelOnClose{ReadCloser: resp.Body, fire: func() { r.once.Do(r.cancel) }}
	return resp, nil
}

// A context cancelled while the paused turn is in hand sends nothing more.
func TestAnthropicExchange_CancelledContextSendsNoResumption(t *testing.T) {
	ws := newWebToolServer(t, pausedFetchReply(4_000), continuationDone)
	c := newContinuationClient(t, ws.server.URL, 0, config.AnthropicProviderConfig{WebFetch: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.httpClient.Transport = &cancelAfterFirstResponse{base: c.httpClient.Transport, cancel: cancel}

	_, err := c.CompleteWithSystem(ctx, "sys", "q")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := ws.sent(); got != 1 {
		t.Fatalf("the endpoint saw %d requests, want only the first", got)
	}
}

// With web tools off a request carries no server declaration and is admitted
// on the transcript alone, byte for byte as before: a window that is exactly
// the plain request admits it, and the same window refuses the request once
// the web tools are on, because their declarations are counted.
func TestAnthropicExchange_DeclarationsAreCountedOnlyWhenDeclared(t *testing.T) {
	plain, err := prompt.EstimateRequestTokens("sys", userTurn("q"), nil)
	if err != nil {
		t.Fatal(err)
	}

	off := newWebToolServer(t, continuationDone)
	c := newContinuationClient(t, off.server.URL, plain, config.AnthropicProviderConfig{})
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "q"); err != nil {
		t.Fatalf("web tools off: a request exactly the window was refused: %v", err)
	}
	if got := serverToolsIn(off.request(t, 0)); len(got) != 0 {
		t.Errorf("web tools off declared %v", got)
	}

	on := newWebToolServer(t, continuationDone)
	c = newContinuationClient(t, on.server.URL, plain, config.AnthropicProviderConfig{WebSearch: true, WebFetch: true})
	_, err = c.CompleteWithSystem(context.Background(), "sys", "q")
	if !errors.Is(err, ErrInputWindowExceeded) {
		t.Fatalf("web tools on: err = %v, want the declarations to push the request over the window", err)
	}
	for _, want := range []string{"server tool declarations", "not counted"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	if got := on.sent(); got != 0 {
		t.Errorf("the endpoint saw %d requests, want none", got)
	}

	// Window = transcript + declarations admits it: the declarations are what
	// the count was short by, nothing more.
	declared, err := serverDeclarationTokens(c.serverTools(continuationModel))
	if err != nil || declared == 0 {
		t.Fatalf("declaration tokens = %d, %v", declared, err)
	}
	fit := newWebToolServer(t, continuationDone)
	c = newContinuationClient(t, fit.server.URL, plain+declared, config.AnthropicProviderConfig{WebSearch: true, WebFetch: true})
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "q"); err != nil {
		t.Fatalf("a request exactly transcript+declarations was refused: %v", err)
	}
}

// A streamed request declares no server tool, so it is not charged for any.
func TestAnthropicStream_AdmissionDoesNotCountUndeclaredServerTools(t *testing.T) {
	plain, err := prompt.EstimateRequestTokens("sys", userTurn("q"), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := newContinuationClient(t, "http://unused.invalid", plain, config.AnthropicProviderConfig{WebSearch: true, WebFetch: true})
	if err := c.admit("sys", userTurn("q"), nil, nil); err != nil {
		t.Fatalf("a streamed request exactly the window was refused: %v", err)
	}
	if err := c.admitInput("sys", userTurn("q"), nil); !errors.Is(err, ErrInputWindowExceeded) {
		t.Fatalf("a request that declares web tools was not charged for them: %v", err)
	}
}

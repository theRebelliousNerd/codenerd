package perception

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// inferenceCounter is an Anthropic endpoint that counts what reaches the
// inference route and answers it, so a refusal can be proven to send nothing.
type inferenceCounter struct {
	inference atomic.Int32
	server    *httptest.Server
}

func newInferenceCounter(t *testing.T) *inferenceCounter {
	t.Helper()
	ic := &inferenceCounter{}
	ic.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/messages/count_tokens") {
			io.WriteString(w, `{"input_tokens":4}`)
			return
		}
		ic.inference.Add(1)
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.Contains(string(body), `"stream":true`):
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, anthropicSSE("end_turn"))
		case strings.Contains(string(body), `"json_schema"`):
			io.WriteString(w, anthropicJSONReply)
		default:
			io.WriteString(w, anthropicChatOK)
		}
	}))
	t.Cleanup(ic.server.Close)
	return ic
}

// anthropicSurface drives one of the client's request entry points.
type anthropicSurface struct {
	name string
	call func(ctx context.Context, c LLMClient, system, user string) error
}

func anthropicSurfaces() []anthropicSurface {
	return []anthropicSurface{
		{"complete", func(ctx context.Context, c LLMClient, _, user string) error {
			_, err := c.Complete(ctx, user)
			return err
		}},
		{"system", func(ctx context.Context, c LLMClient, system, user string) error {
			_, err := c.CompleteWithSystem(ctx, system, user)
			return err
		}},
		{"tools", func(ctx context.Context, c LLMClient, system, user string) error {
			_, err := c.CompleteWithTools(ctx, system, user, nil)
			return err
		}},
		{"history", func(ctx context.Context, c LLMClient, system, user string) error {
			trp, ok := c.(types.ToolResultsProvider)
			if !ok {
				return fmt.Errorf("%T lost native tool continuation", c)
			}
			_, err := trp.CompleteWithToolResults(ctx, system, []types.Message{{Role: "user", Text: user}}, nil)
			return err
		}},
		{"stream", func(ctx context.Context, c LLMClient, system, user string) error {
			content, errs := c.CompleteWithStreaming(ctx, system, user, false)
			for range content {
			}
			var last error
			for err := range errs {
				if err != nil {
					last = err
				}
			}
			return last
		}},
		{"schema", func(ctx context.Context, c LLMClient, system, user string) error {
			schema, ok := core.AsSchemaCapable(c)
			if !ok {
				return fmt.Errorf("%T lost structured output", c)
			}
			_, err := schema.CompleteWithSchema(ctx, system, user, anthropicClosedSchema)
			return err
		}},
	}
}

// The client's backstop: every entry point refuses a request larger than the
// slot's max_input_tokens before anything is sent, names the cap, and a
// request within the cap is sent as before.
func TestAnthropicInputWindow_RefusesOverCapRequestsBeforeSending(t *testing.T) {
	const window = 50 // tokens; 200 bytes at the estimator's 4 bytes a token
	over := strings.Repeat("x", 2000)
	under := "short"

	for _, surface := range anthropicSurfaces() {
		t.Run(surface.name, func(t *testing.T) {
			ic := newInferenceCounter(t)
			client := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "test-key", BaseURL: ic.server.URL, Model: "claude-haiku-5-5", MaxInputTokens: window})
			if got := client.InputWindowTokens(); got != window {
				t.Fatalf("InputWindowTokens() = %d, want %d", got, window)
			}

			err := surface.call(context.Background(), client, "sys", over)
			if !errors.Is(err, ErrInputWindowExceeded) {
				t.Fatalf("over-cap request: err = %v, want ErrInputWindowExceeded", err)
			}
			if !strings.Contains(err.Error(), "max_input_tokens is 50") {
				t.Errorf("the refusal does not name the cap: %v", err)
			}
			if code := providerFailureStatus(err); code != 0 {
				t.Errorf("the refusal reads as provider status %d; a credential pool would replace it", code)
			}
			if hits := ic.inference.Load(); hits != 0 {
				t.Fatalf("over-cap request reached the endpoint %d time(s); it must be refused before sending", hits)
			}

			if err := surface.call(context.Background(), client, "sys", under); err != nil {
				t.Fatalf("under-cap request: %v", err)
			}
			if hits := ic.inference.Load(); hits != 1 {
				t.Fatalf("under-cap request reached the endpoint %d time(s), want 1", hits)
			}
		})
	}
}

// The refusal measures the request as the session executor does: a request
// the executor's measure puts exactly at the cap is sent, one token more is
// not.
func TestAnthropicInputWindow_MeasuresAsTheExecutorDoes(t *testing.T) {
	history := []types.Message{{Role: "user", Text: strings.Repeat("y", 1000)}}
	used, err := prompt.EstimateRequestTokens("sys", history, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		window  int
		refused bool
	}{
		{name: "exactly at the cap is sent", window: used},
		{name: "one token over is refused", window: used - 1, refused: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := newInferenceCounter(t)
			client := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "test-key", BaseURL: ic.server.URL, Model: "claude-haiku-5-5", MaxInputTokens: tt.window})
			_, err := client.CompleteWithToolResults(context.Background(), "sys", history, nil)
			if got := errors.Is(err, ErrInputWindowExceeded); got != tt.refused {
				t.Fatalf("refused = %v (err %v), want %v", got, err, tt.refused)
			}
			if !tt.refused && err != nil {
				t.Fatalf("CompleteWithToolResults: %v", err)
			}
		})
	}
}

func TestAnthropicInputWindow_NegativeCapIsAConfigError(t *testing.T) {
	client := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", Model: "claude-haiku-5-5", MaxInputTokens: -1})
	if client.configErr == nil || !strings.Contains(client.configErr.Error(), "max_input_tokens") {
		t.Fatalf("configErr = %v, want a max_input_tokens error", client.configErr)
	}
}

// The slot's cap reaches the client the factory builds for it, pooled or not,
// and the client the executor holds reports it through every wrapper. A slot
// without a cap reports none.
func TestSlotClients_ThreadMaxInputTokens(t *testing.T) {
	seq := poolTestSequence.Add(1)
	pools := map[string][]config.ProviderAPIKey{"anthropic": {
		{Name: "alpha", APIKey: fmt.Sprintf("test-secret-%d-a", seq)},
		{Name: "beta", APIKey: fmt.Sprintf("test-secret-%d-b", seq)},
	}}
	tests := []struct {
		name  string
		cfg   *config.UserConfig
		build func(*config.UserConfig) (LLMClient, error)
		want  int
	}{
		{
			name: "worker with a cap",
			cfg: &config.UserConfig{AnthropicAPIKey: "k",
				Worker: &config.WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: 100000}},
			build: NewWorkerClientFromUserConfig,
			want:  100000,
		},
		{
			name: "meta worker with a cap",
			cfg: &config.UserConfig{MetaAPIKey: "mt",
				Worker: &config.WorkerLLMConfig{Provider: "meta", Model: metaContributorModel, MaxInputTokens: 100000}},
			build: NewWorkerClientFromUserConfig,
			want:  100000,
		},
		{
			name: "planner without a cap",
			cfg: &config.UserConfig{AnthropicAPIKey: "k",
				Planner: &config.PlannerLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5"}},
			build: NewPlannerClientFromUserConfig,
			want:  0,
		},
		{
			name: "pooled worker with a cap",
			cfg: &config.UserConfig{APIKeyPools: pools,
				Worker: &config.WorkerLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", MaxInputTokens: 100000}},
			build: NewWorkerClientFromUserConfig,
			want:  100000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := tt.build(tt.cfg)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if got := broker.InputWindow(client); got != tt.want {
				t.Fatalf("broker.InputWindow(slot client) = %d, want %d", got, tt.want)
			}
		})
	}
}

// A cap the slot's client cannot enforce is refused at construction: built
// anyway, the slot would run uncapped under a config that says otherwise.
func TestSlotClients_RefuseACapTheClientCannotEnforce(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.UserConfig
	}{
		{
			name: "openai worker",
			cfg: &config.UserConfig{OpenAIAPIKey: "ok",
				Worker: &config.WorkerLLMConfig{Provider: "openai", Model: "gpt-test", MaxInputTokens: 100000}},
		},
		{
			name: "ollama worker",
			cfg: &config.UserConfig{
				Worker: &config.WorkerLLMConfig{Provider: "ollama", Model: "gemma4:12b", MaxInputTokens: 100000}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWorkerClientFromUserConfig(tt.cfg)
			if err == nil || !strings.Contains(err.Error(), "worker.max_input_tokens") {
				t.Fatalf("NewWorkerClientFromUserConfig = (%T, %v), want an error naming worker.max_input_tokens", client, err)
			}
			if client != nil {
				t.Errorf("a refused slot returned a client: %T", client)
			}
		})
	}
}

// Through a credential pool the refusal is the client's own error: no
// inference request is sent and it is not turned into a credential failure.
// Every credential carries the cap, whichever the router selects: the calls
// below alternate between the two, and each reports and enforces it.
func TestPooledAnthropic_EveryCredentialRefusesOverCapWithoutSending(t *testing.T) {
	ic := newInferenceCounter(t)
	cfg := testPoolConfig(t, ic.server.URL)
	cfg.MaxInputTokens = 50
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if got := broker.InputWindow(client); got != 50 {
			t.Fatalf("call %d: broker.InputWindow(pooled) = %d, want 50", i, got)
		}
		_, err = client.CompleteWithSystem(ctx, "sys", strings.Repeat("x", 2000))
		if !errors.Is(err, ErrInputWindowExceeded) {
			t.Fatalf("call %d: err = %v, want ErrInputWindowExceeded", i, err)
		}
		var pooled *pooledCallError
		if errors.As(err, &pooled) {
			t.Fatalf("call %d: the pool replaced the refusal with a credential failure: %v", i, err)
		}
		if _, err := client.CompleteWithSystem(ctx, "sys", "short"); err != nil {
			t.Fatalf("call %d: under-cap request: %v", i, err)
		}
	}
	if hits := ic.inference.Load(); hits != 4 {
		t.Fatalf("inference saw %d request(s), want only the 4 under-cap ones", hits)
	}
}

// The Meta client is the same backstop: every entry point refuses a request
// larger than the slot's max_input_tokens before anything is sent, and a
// request within it is sent.
func TestMetaInputWindow_RefusesOverCapRequestsBeforeSending(t *testing.T) {
	const window = 50
	over := strings.Repeat("x", 2000)
	surfaces := []struct {
		name string
		call func(ctx context.Context, c *OpenAICompatClient, user string) error
	}{
		{"system", func(ctx context.Context, c *OpenAICompatClient, user string) error {
			_, err := c.CompleteWithSystem(ctx, "sys", user)
			return err
		}},
		{"tools", func(ctx context.Context, c *OpenAICompatClient, user string) error {
			_, err := c.CompleteWithTools(ctx, "sys", user, nil)
			return err
		}},
		{"history", func(ctx context.Context, c *OpenAICompatClient, user string) error {
			_, err := c.CompleteWithToolResults(ctx, "sys", []types.Message{{Role: "user", Text: user}}, nil)
			return err
		}},
		{"stream", func(ctx context.Context, c *OpenAICompatClient, user string) error {
			content, errs := c.CompleteWithStreaming(ctx, "sys", user, false)
			for range content {
			}
			var last error
			for err := range errs {
				if err != nil {
					last = err
				}
			}
			return last
		}},
		{"grounded search", func(ctx context.Context, c *OpenAICompatClient, user string) error {
			_, err := c.GroundedWebSearch(ctx, user)
			return err
		}},
	}
	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
			}))
			defer srv.Close()
			c := newTestCompatClient(t, ProviderMeta, srv.URL)
			c.maxInputTokens = window
			if got := c.InputWindowTokens(); got != window {
				t.Fatalf("InputWindowTokens() = %d, want %d", got, window)
			}
			err := surface.call(context.Background(), c, over)
			if !errors.Is(err, ErrInputWindowExceeded) {
				t.Fatalf("over-cap request: err = %v, want ErrInputWindowExceeded", err)
			}
			if n := hits.Load(); n != 0 {
				t.Fatalf("over-cap request reached the endpoint %d time(s)", n)
			}
		})
	}

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.maxInputTokens = window
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "short"); err != nil {
		t.Fatalf("under-cap request: %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("under-cap request reached the endpoint %d time(s), want 1", n)
	}
}

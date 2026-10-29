package init

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// identifiedClient is a provider client that reports its identity and,
// optionally, an input cap, and records every request it is sent.
type identifiedClient struct {
	provider, model string
	window          int
	reply           func(system, user string) (string, error)

	mu    sync.Mutex
	calls []string
}

func (c *identifiedClient) ModelIdentity() (string, string) { return c.provider, c.model }

func (c *identifiedClient) InputWindowTokens() int { return c.window }

func (c *identifiedClient) record(system, user string) (string, error) {
	c.mu.Lock()
	c.calls = append(c.calls, system+"\n----\n"+user)
	c.mu.Unlock()
	if c.reply != nil {
		return c.reply(system, user)
	}
	return "ok", nil
}

func (c *identifiedClient) Complete(_ context.Context, prompt string) (string, error) {
	return c.record("", prompt)
}

func (c *identifiedClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	return c.record(system, user)
}

func (c *identifiedClient) CompleteWithTools(_ context.Context, system, user string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	text, err := c.record(system, user)
	return &types.LLMToolResponse{Text: text}, err
}

func (c *identifiedClient) CompleteWithStreaming(_ context.Context, system, user string, _ bool) (<-chan string, <-chan types.StreamResult) {
	text, err := c.record(system, user)
	if err != nil {
		return types.FailedStream(err)
	}
	return types.StreamWhole(context.Background(), func(context.Context) (string, error) { return text, nil })
}

func (c *identifiedClient) requests() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

// scheduledLayer stands in for a scheduling or tracing decorator: it reports
// nothing of its own and is walked through.
type scheduledLayer struct{ inner perception.LLMClient }

func (s scheduledLayer) Unwrap() types.LLMClient { return s.inner }
func (s scheduledLayer) Complete(ctx context.Context, prompt string) (string, error) {
	return s.inner.Complete(ctx, prompt)
}
func (s scheduledLayer) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return s.inner.CompleteWithSystem(ctx, system, user)
}
func (s scheduledLayer) CompleteWithTools(ctx context.Context, system, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return s.inner.CompleteWithTools(ctx, system, user, tools)
}
func (s scheduledLayer) CompleteWithStreaming(ctx context.Context, system, user string, thinking bool) (<-chan string, <-chan types.StreamResult) {
	return s.inner.CompleteWithStreaming(ctx, system, user, thinking)
}

func TestRoute(t *testing.T) {
	main := &identifiedClient{provider: "meta", model: "worker-model"}
	planner := &identifiedClient{provider: "anthropic", model: "planner-model", window: 64000}

	tests := []struct {
		name       string
		cfg        InitConfig
		slot       llmSlot
		wantClient perception.LLMClient
		wantSlot   llmSlot
		wantModel  string
		wantWindow int
	}{
		{name: "planner slot with a planner", cfg: InitConfig{LLMClient: main, PlannerClient: planner}, slot: slotPlanner, wantClient: planner, wantSlot: slotPlanner, wantModel: "planner-model", wantWindow: 64000},
		{name: "main slot with a planner", cfg: InitConfig{LLMClient: main, PlannerClient: planner}, slot: slotMain, wantClient: main, wantSlot: slotMain, wantModel: "worker-model"},
		{name: "planner slot falls back to main", cfg: InitConfig{LLMClient: main}, slot: slotPlanner, wantClient: main, wantSlot: slotMain, wantModel: "worker-model"},
		{name: "no client at all", cfg: InitConfig{}, slot: slotPlanner, wantSlot: slotMain},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := (&Initializer{config: tt.cfg}).route(tt.slot)
			if route.client != tt.wantClient || route.slot != tt.wantSlot || route.model != tt.wantModel || route.window != tt.wantWindow {
				t.Fatalf("route = {slot %s model %q window %d client %p}, want {slot %s model %q window %d client %p}",
					route.slot, route.model, route.window, route.client, tt.wantSlot, tt.wantModel, tt.wantWindow, tt.wantClient)
			}
		})
	}
}

func TestClientIdentity_WalksDecorators(t *testing.T) {
	base := &identifiedClient{provider: "anthropic", model: "planner-model"}
	provider, model := clientIdentity(scheduledLayer{inner: scheduledLayer{inner: base}})
	if provider != "anthropic" || model != "planner-model" {
		t.Fatalf("clientIdentity = (%q, %q), want the base client's identity through two decorator layers", provider, model)
	}
}

// The handler is given the routed client, so a planner phase cannot reach the
// main client by accident, and the compile names the model that reads it.
func TestWithJITPrompt_SendsToTheRoutedClient(t *testing.T) {
	main := &identifiedClient{provider: "meta", model: "worker-model"}
	planner := &identifiedClient{provider: "anthropic", model: "planner-model"}
	ini := &Initializer{config: InitConfig{LLMClient: main, PlannerClient: planner}}

	for _, slot := range []llmSlot{slotPlanner, slotMain} {
		_, err := ini.withJITPrompt(context.Background(), initCall{slot: slot, phase: "fixture", task: "say " + string(slot), system: "guidance"},
			func(ctx context.Context, client perception.LLMClient, prompt string) (string, error) {
				return client.CompleteWithSystem(ctx, prompt, "say "+string(slot))
			})
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := planner.requests(); len(got) != 1 || !strings.Contains(got[0], "say planner") {
		t.Fatalf("planner requests = %q, want only the planner call", got)
	}
	if got := main.requests(); len(got) != 1 || !strings.Contains(got[0], "say main") {
		t.Fatalf("main requests = %q, want only the main call", got)
	}
	metrics := ini.snapshotLLMMetrics()
	if len(metrics.Clients) != 2 || metrics.Clients[0].Slot != "planner" || metrics.Clients[1].Slot != "main" {
		t.Fatalf("client rows = %+v, want a planner and a main row", metrics.Clients)
	}
}

func TestWithJITPrompt_NoClientIsNotAnAttempt(t *testing.T) {
	ini := &Initializer{}
	_, err := ini.withJITPrompt(context.Background(), initCall{slot: slotPlanner, phase: "north_star", task: "x"},
		func(context.Context, perception.LLMClient, string) (string, error) {
			t.Fatal("handler ran without a client")
			return "", nil
		})
	if err == nil {
		t.Fatal("want an error when no client is configured")
	}
	if metrics := ini.snapshotLLMMetrics(); metrics.Attempts != 0 {
		t.Fatalf("attempts = %d, want 0: no provider was called", metrics.Attempts)
	}
}

// The init compile names the serving model (provider and model are
// fail-closed dimensions) and fits the serving client's input cap.
func TestInitCompilationContext_CompilesForTheRoute(t *testing.T) {
	capped := llmRoute{slot: slotPlanner, provider: "anthropic", model: "planner-model", window: 20000}
	cc := initCompilationContext(capped, "agents", "build the team", nil)
	if cc.Provider != "anthropic" || cc.Model != "planner-model" {
		t.Fatalf("compile identity = (%q, %q), want the planner's", cc.Provider, cc.Model)
	}
	if cc.TokenBudget > 20000 {
		t.Fatalf("TokenBudget = %d, want at most the 20000-token slot cap", cc.TokenBudget)
	}
	if err := cc.Validate(); err != nil {
		t.Fatalf("a slot-capped init compile context must validate (reserved < budget): %v", err)
	}

	uncapped := initCompilationContext(llmRoute{slot: slotMain}, "agents", "build the team", nil)
	if uncapped.TokenBudget < cc.TokenBudget {
		t.Fatalf("uncapped budget %d is below the capped %d", uncapped.TokenBudget, cc.TokenBudget)
	}
	if err := uncapped.Validate(); err != nil {
		t.Fatal(err)
	}
}

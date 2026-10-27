package system

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// slotPolicyEndpoint serves both vendors of the deployment from one address --
// Meta's chat completions and Anthropic's messages -- and records what each
// request carried, so a test can read the request on the wire.
type slotPolicyEndpoint struct {
	mu       sync.Mutex
	requests []slotPolicyRequest
	server   *httptest.Server
}

type slotPolicyRequest struct {
	path string
	body map[string]any
}

const slotPolicyAnthropicReply = `{"type":"message","role":"assistant","model":"claude-opus-5-5","stop_reason":"end_turn",
"usage":{"input_tokens":10,"output_tokens":20,"server_tool_use":{"web_search_requests":1}},
"content":[
 {"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{"query":"go 1.27"}},
 {"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[{"type":"web_search_result","url":"https://go.dev/doc/go1.27","title":"Go 1.27","encrypted_content":"abc"}]},
 {"type":"text","text":"answer","citations":[{"type":"web_search_result_location","url":"https://go.dev/blog/go1.27","title":"Blog","encrypted_index":"x","cited_text":"..."}]}
]}`

func newSlotPolicyEndpoint(t *testing.T) *slotPolicyEndpoint {
	t.Helper()
	ep := &slotPolicyEndpoint{}
	ep.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/count_tokens") {
			io.WriteString(w, `{"input_tokens":4}`)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode %s: %v", r.URL.Path, err)
		}
		ep.mu.Lock()
		ep.requests = append(ep.requests, slotPolicyRequest{path: r.URL.Path, body: body})
		ep.mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/messages"):
			io.WriteString(w, slotPolicyAnthropicReply)
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":500,"total_tokens":1500}}`)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ep.server.Close)
	return ep
}

func (ep *slotPolicyEndpoint) count() int {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return len(ep.requests)
}

func (ep *slotPolicyEndpoint) last(t *testing.T) slotPolicyRequest {
	t.Helper()
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if len(ep.requests) == 0 {
		t.Fatal("no request reached the endpoint")
	}
	return ep.requests[len(ep.requests)-1]
}

func (ep *slotPolicyEndpoint) all() []slotPolicyRequest {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return append([]slotPolicyRequest(nil), ep.requests...)
}

func anthropicEffortOf(body map[string]any) string {
	out, _ := body["output_config"].(map[string]any)
	effort, _ := out["effort"].(string)
	return effort
}

// The deployment's slot policies, booted through the real factory and read on
// the wire: a Meta main and worker at xhigh with the worker capped at 100k input
// tokens, the classifier at Meta's lowest effort, an Anthropic Opus planner at
// its default effort with web search on, a reviewer profile routed to Opus, and
// the Muse Spark contributor price on the usage meter.
func TestBoot_SlotPoliciesReachTheWire(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ep := newSlotPolicyEndpoint(t)
	searchOff := false
	cfg := &config.UserConfig{
		Provider: "meta", Model: "muse-spark-1.3-contributor", MetaAPIKey: "test-meta-key", BaseURL: ep.server.URL,
		ReasoningEffort: "xhigh", ClassificationModel: "muse-spark-1.3-contributor",
		Meta:            &config.MetaProviderConfig{EnableWebSearch: &searchOff},
		AnthropicAPIKey: "test-anthropic-key",
		Worker: &config.WorkerLLMConfig{Provider: "meta", Model: "muse-spark-1.3-contributor", Endpoint: ep.server.URL,
			ReasoningEffort: "xhigh", MaxInputTokens: 100000},
		Planner: &config.PlannerLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", Endpoint: ep.server.URL,
			Anthropic: &config.AnthropicProviderConfig{WebSearch: true}},
		ShardProfiles: map[string]config.ShardProfile{
			"reviewer": {Provider: "anthropic", Model: "claude-opus-5-5"},
			"tester":   {Provider: "anthropic", Model: "claude-opus-5-5", ReasoningEffort: "low"},
		},
	}
	if errs := config.Errors(cfg.Check(nil)); len(errs) != 0 {
		t.Fatalf("the deployment config does not pass its own check: %v", errs)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ws := t.TempDir()
	cortex, err := BootCortexWithConfig(ctx, BootConfig{Workspace: ws, UserConfigOverride: cfg})
	if err != nil {
		t.Fatalf("BootCortexWithConfig: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	if cortex.WorkerLLMClient == nil || cortex.PlannerLLMClient == nil {
		t.Fatalf("boot built no worker (%v) or planner (%v)", cortex.WorkerLLMClient, cortex.PlannerLLMClient)
	}

	t.Run("main effort and the Muse Spark price", func(t *testing.T) {
		metered := usage.NewContext(ctx, cortex.UsageTracker)
		if _, err := cortex.LLMClient.CompleteWithSystem(metered, "sys", "hello"); err != nil {
			t.Fatalf("main: %v", err)
		}
		if got := ep.last(t).body["reasoning_effort"]; got != "xhigh" {
			t.Errorf("main reasoning_effort on the wire = %v, want xhigh", got)
		}
		counts := cortex.UsageTracker.Stats().ByModel["muse-spark-1.3-contributor"]
		want := 1000*0.10/1e6 + 500*0.20/1e6
		if counts.Input != 1000 || math.Abs(counts.Cost-want) > 1e-12 {
			t.Errorf("usage for muse-spark-1.3-contributor = %+v, want 1000 input priced at %g", counts, want)
		}
	})

	t.Run("worker effort and input cap", func(t *testing.T) {
		if got := broker.InputWindow(cortex.WorkerLLMClient); got != 100000 {
			t.Fatalf("the booted worker reports input window %d, want 100000", got)
		}
		// The session executor is handed the worker through the session
		// adapter (initFinalExecutors), and sizes every working request to
		// the window it reports through it.
		if got := broker.InputWindow(&sessionLLMAdapter{client: cortex.WorkerLLMClient, tracker: cortex.UsageTracker}); got != 100000 {
			t.Fatalf("the session executor's client reports input window %d, want 100000", got)
		}
		if _, err := cortex.WorkerLLMClient.CompleteWithSystem(ctx, "sys", "hello"); err != nil {
			t.Fatalf("worker: %v", err)
		}
		if got := ep.last(t).body["reasoning_effort"]; got != "xhigh" {
			t.Errorf("worker reasoning_effort on the wire = %v, want xhigh", got)
		}
		sent := ep.count()
		_, err := cortex.WorkerLLMClient.CompleteWithSystem(ctx, "sys", strings.Repeat("x", 4*100001))
		if !errors.Is(err, perception.ErrInputWindowExceeded) {
			t.Fatalf("an over-cap worker request: err = %v, want ErrInputWindowExceeded", err)
		}
		if ep.count() != sent {
			t.Fatal("an over-cap worker request reached the wire")
		}
	})

	t.Run("planner effort, web search and grounding sources", func(t *testing.T) {
		if _, err := cortex.PlannerLLMClient.CompleteWithSystem(ctx, "sys", "what changed in Go 1.27?"); err != nil {
			t.Fatalf("planner: %v", err)
		}
		req := ep.last(t)
		if got := anthropicEffortOf(req.body); got != config.DefaultPlannerReasoningEffort {
			t.Errorf("planner effort on the wire = %q, want the planner default %q", got, config.DefaultPlannerReasoningEffort)
		}
		var search bool
		tools, _ := req.body["tools"].([]any)
		for _, raw := range tools {
			tool, _ := raw.(map[string]any)
			if tool["type"] == "web_search_20260209" && tool["max_uses"] == float64(config.DefaultAnthropicWebToolMaxUses) {
				search = true
			}
		}
		if !search {
			t.Errorf("the planner request declared no web_search_20260209 with %d uses: %v", config.DefaultAnthropicWebToolMaxUses, tools)
		}
		provider, ok := cortex.PlannerLLMClient.(types.GroundingProvider)
		if !ok {
			t.Fatal("the booted planner is not a GroundingProvider")
		}
		want := []string{"https://go.dev/doc/go1.27", "https://go.dev/blog/go1.27"}
		if got := provider.GetLastGroundingSources(); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("grounding sources through the booted chain = %v, want %v", got, want)
		}
	})

	t.Run("profile routes carry the routing slot's effort", func(t *testing.T) {
		tests := []struct {
			name    string
			client  perception.LLMClient
			profile string
			want    string
		}{
			{name: "the planner routes the reviewer", client: cortex.PlannerLLMClient, profile: "reviewer", want: config.DefaultPlannerReasoningEffort},
			{name: "the worker routes the reviewer", client: cortex.WorkerLLMClient, profile: "reviewer", want: "xhigh"},
			{name: "a profile's own effort wins", client: cortex.WorkerLLMClient, profile: "tester", want: "low"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				routed := config.ShardProfileContext(ctx, cfg.GetShardProfile(tt.profile))
				if _, err := tt.client.CompleteWithSystem(routed, "sys", "review this"); err != nil {
					t.Fatalf("routed call: %v", err)
				}
				req := ep.last(t)
				if !strings.HasSuffix(req.path, "/messages") || req.body["model"] != "claude-opus-5-5" {
					t.Fatalf("the call was not routed to Anthropic Opus: %s %v", req.path, req.body["model"])
				}
				if got := anthropicEffortOf(req.body); got != tt.want {
					t.Errorf("routed effort on the wire = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("the classifier runs at Meta's lowest effort", func(t *testing.T) {
		before := ep.count()
		_, _ = cortex.Transducer.ParseIntent(ctx, "fix the failing test in parser.go")
		var efforts []any
		for _, req := range ep.all()[before:] {
			if strings.HasSuffix(req.path, "/chat/completions") {
				efforts = append(efforts, req.body["reasoning_effort"])
			}
		}
		if len(efforts) == 0 {
			t.Fatal("classification sent no request")
		}
		for _, effort := range efforts {
			if effort != "minimal" {
				t.Errorf("classification reasoning_effort on the wire = %v, want minimal (all: %v)", effort, efforts)
			}
		}
	})
}

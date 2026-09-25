package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// envelopeOnlyClient is what a CLI engine looks like to the executor: it
// speaks Piggyback and has no tool-result channel of its own. Every request
// it receives is recorded, so a test can read what the model was shown.
type envelopeOnlyClient struct {
	*MockLLMClient
	mu       sync.Mutex
	requests []string
	systems  []string
}

func (*envelopeOnlyClient) ShouldUsePiggybackTools() bool { return true }

func newEnvelopeOnlyClient(answer func(round int, user string) string) *envelopeOnlyClient {
	c := &envelopeOnlyClient{}
	c.MockLLMClient = &MockLLMClient{
		CompleteWithSystemFunc: func(_ context.Context, sys, user string) (string, error) {
			c.mu.Lock()
			c.requests = append(c.requests, user)
			c.systems = append(c.systems, sys)
			round := len(c.requests)
			c.mu.Unlock()
			return answer(round, user), nil
		},
	}
	return c
}

func piggybackTestExecutor(t *testing.T, client types.LLMClient) *Executor {
	t.Helper()
	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		llmClient:    client,
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.VerifyBuildAfterEdits = false
	executor.config.VerifyTestsAfterEdits = false
	executor.config.CriticReviewAfterEdits = false
	executor.config.WorkspaceRoot = t.TempDir()
	return executor
}

// An envelope client reads, is shown what it read, and acts on it. Until
// 2026-09-25 the Piggyback path executed the first batch of tool requests and
// returned: the model never saw a result, so the second request here was never
// made. Both requests reuse the id the protocol's own example teaches ("req_1"),
// which is what a model does every round; the second must not be paired to the
// first's result.
func TestPiggybackClientSeesItsToolResultsAndContinues(t *testing.T) {
	const reader, second = "piggyback_channel_reader", "piggyback_channel_second"
	var reads, seconds int
	ensureGeneralProbe(t, reader, func(context.Context, map[string]any) (string, error) {
		reads++
		return "PROBE-CONTENT-42", nil
	})
	ensureGeneralProbe(t, second, func(context.Context, map[string]any) (string, error) {
		seconds++
		return "SECOND-RESULT-7", nil
	})

	client := newEnvelopeOnlyClient(func(round int, user string) string {
		switch round {
		case 1:
			return piggySingleRequest("reading", "req_1", reader, map[string]any{"path": "a.go"})
		case 2:
			return piggySingleRequest("acting on it", "req_1", second, map[string]any{"path": "b.go"})
		default:
			return `{"control_packet":{"tool_requests":[]},"surface_response":"all done"}`
		}
	})
	executor := piggybackTestExecutor(t, client)

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	resp, toolErrs, err := executor.runToolLoop(context.Background(), "system", "inspect a.go then b.go",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{reader, second}}, nil, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors %v)", err, toolErrs)
	}
	if reads != 1 || seconds != 1 {
		t.Fatalf("reader ran %d time(s), second ran %d time(s); want one each -- the model's "+
			"second request was never made or ran twice", reads, seconds)
	}
	if len(client.requests) != 3 {
		t.Fatalf("the client was asked %d time(s), want 3 (request, continue with the read, conclude)", len(client.requests))
	}
	if !strings.Contains(client.requests[1], "PROBE-CONTENT-42") ||
		!strings.Contains(client.requests[1], "tool_result id=req_1 status=ok") {
		t.Fatalf("the second request does not show the model its read's result:\n%s", client.requests[1])
	}
	// The repeated "req_1" is renamed, and the model is shown the new name
	// on both the request and its result.
	third := client.requests[2]
	if strings.Count(third, "tool_result id=req_1 ") != 1 {
		t.Fatalf("the reused id pairs two results in the third request:\n%s", third)
	}
	if !strings.Contains(third, "tool=piggyback_channel_second") || !strings.Contains(third, "SECOND-RESULT-7") {
		t.Fatalf("the third request does not carry the second call and its result:\n%s", third)
	}
	if !strings.Contains(third, "tool_request id=pb_") || !strings.Contains(third, "tool_result id=pb_") {
		t.Fatalf("the reused id was not replaced by a loop-unique one:\n%s", third)
	}
	if got := executor.processPiggybackControlPacket(resp.Text); got != "all done" {
		t.Fatalf("final surface = %q, want the model's conclusion", got)
	}
	if result.ToolCallsExecuted != 2 {
		t.Fatalf("ToolCallsExecuted = %d, want 2", result.ToolCallsExecuted)
	}
	// The catalog rides the system prompt of every request, not only the first.
	for i, sys := range client.systems[:2] {
		if !strings.Contains(sys, reader) {
			t.Errorf("request %d's system prompt carries no tool catalog", i+1)
		}
	}
}

// A model that asks for the same thing every round is ended by the working
// policy, as the native loop's is -- not by a count, and not by running
// forever now that the envelope channel continues. On a read-only turn the
// policy finalizes: the model is asked once more with exploration closed, the
// request offers it no tools (so its catalog is empty), and a repeated request
// is refused rather than run.
func TestPiggybackRepeatedRequestIsEndedByTheWorkingPolicy(t *testing.T) {
	const probe = "piggyback_channel_repeat"
	executions := 0
	ensureGeneralProbe(t, probe, func(context.Context, map[string]any) (string, error) {
		executions++
		return "same answer", nil
	})
	client := newEnvelopeOnlyClient(func(int, string) string {
		return piggySingleRequest("again", "req_1", probe, map[string]any{"path": "same.go"})
	})
	executor := piggybackTestExecutor(t, client)

	// Only a guard for the test process; the loop's own stop is under test.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	_, toolErrs, err := executor.runToolLoop(ctx, "system", "look",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{probe}}, nil, result)
	if ctx.Err() != nil {
		t.Fatalf("the loop ran until the test's guard expired; nothing derived an end: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "repeated_cycle") {
		t.Fatalf("ended with %v, want the working policy's finalize or its repeated_cycle stop", err)
	}
	if err == nil && !slicesContainSubstring(toolErrs, "not offered during forced finalization") {
		t.Fatalf("ended without a policy stop or a forced finalization: tool errors %v, %d request(s)", toolErrs, len(client.requests))
	}
	if executions != result.ToolCallsExecuted {
		t.Fatalf("the probe ran %d time(s) but %d executed call(s) were counted", executions, result.ToolCallsExecuted)
	}
	last := client.systems[len(client.systems)-1]
	if err == nil && strings.Contains(last, probe) {
		t.Fatalf("the forced final request still offered %s in its catalog:\n%s", probe, last)
	}
}

func slicesContainSubstring(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// The catalog a request carries is the one it offers: the commit regime and
// the forced final answer narrow what the model may call, and an envelope
// client reads its tools from the prompt, so a catalog of the whole allowlist
// would offer it tools the round will refuse.
func TestPiggybackCatalogIsTheRequestsOffer(t *testing.T) {
	const kept, dropped = "piggyback_catalog_kept", "piggyback_catalog_dropped"
	for _, name := range []string{kept, dropped} {
		ensureGeneralProbe(t, name, func(context.Context, map[string]any) (string, error) { return "", nil })
	}
	executor := piggybackTestExecutor(t, nil)
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{kept, dropped}}

	catalog := executor.piggybackCatalog(cfg, []types.ToolDefinition{{Name: kept}})
	if !strings.Contains(catalog, kept) || strings.Contains(catalog, dropped) {
		t.Fatalf("catalog for an offer of %s:\n%s", kept, catalog)
	}
	if got := executor.piggybackCatalog(cfg, nil); got != "" {
		t.Fatalf("an empty offer rendered a catalog:\n%s", got)
	}
	if len(cfg.AllowedTools) != 2 {
		t.Fatalf("narrowing mutated the persona's allowlist: %v", cfg.AllowedTools)
	}
}

// Reasoning cannot be replayed on this channel, and folded into the prose it
// would read as something the model said.
func TestRenderPiggybackConversationDropsReasoning(t *testing.T) {
	turn := types.NewAssistantMessage(
		types.ThinkingBlock("SECRET-REASONING", "sig"),
		types.TextBlock("visible prose"),
		types.ToolUseBlock("c1", "read_file", map[string]any{"path": "x.go"}),
	)
	out := renderPiggybackConversation([]types.Message{
		{Role: "user", Text: "the task"},
		turn,
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "c1", Content: "boom", IsError: true}}},
	})
	if strings.Contains(out, "SECRET-REASONING") {
		t.Fatalf("reasoning leaked into the rendered conversation:\n%s", out)
	}
	for _, want := range []string{"user: the task", "assistant: visible prose",
		`assistant tool_request id=c1 tool=read_file args={"path":"x.go"}`, "tool_result id=c1 status=error:\nboom"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered conversation lacks %q:\n%s", want, out)
		}
	}
}

// A round that blocked an unsafe mangle_update and went on to call tools is
// not the turn's last word, so its surface -- where the constitutional
// override puts its notice -- is never shown. The notice is carried to the
// response instead. Before the envelope channel continued, the single round
// was also the last one and showed it; a continuation that dropped it would
// hide from the user that the model tried to write a protected fact.
func TestPiggybackSafetyNoticeOutlivesTheRoundThatRaisedIt(t *testing.T) {
	const probe = "piggyback_channel_notice_probe"
	ensureGeneralProbe(t, probe, func(context.Context, map[string]any) (string, error) { return "ok", nil })

	first := `{"control_packet":{"mangle_updates":["permitted(/piggyback_channel_notice_probe, \"x\", \"{}\")."],` +
		`"tool_requests":[{"id":"req_1","tool_name":"` + probe + `","tool_args":{}}]},"surface_response":"working"}`
	client := newEnvelopeOnlyClient(func(round int, _ string) string {
		if round == 1 {
			return first
		}
		return `{"control_packet":{"tool_requests":[]},"surface_response":"finished"}`
	})
	executor := piggybackTestExecutor(t, client)
	executor.kernel = realKernel(t)

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	resp, toolErrs, err := executor.runToolLoop(context.Background(), "system", "look",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{probe}}, nil, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (%v)", err, toolErrs)
	}
	if len(result.safetyNotices) != 1 {
		t.Fatalf("carried %d safety notice(s), want the one the first round raised: %v", len(result.safetyNotices), result.safetyNotices)
	}
	response := withSafetyNotices(executor.processPiggybackControlPacket(resp.Text), result.safetyNotices)
	if !strings.HasPrefix(response, safetyNoticePrefix) || !strings.HasSuffix(response, "finished") {
		t.Fatalf("the response does not carry the first round's notice ahead of the conclusion:\n%s", response)
	}
	// Once only, however often it is folded in.
	if again := withSafetyNotices(response, result.safetyNotices); again != response {
		t.Fatalf("the notice was repeated:\n%s", again)
	}
}

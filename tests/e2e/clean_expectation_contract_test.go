//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

func sclCompletedHistoryError(history []perception.ConversationTurn, inputs map[string]struct{}, reply string) error {
	if len(history) != 2*len(inputs) {
		return fmt.Errorf("history has %d events, want %d", len(history), 2*len(inputs))
	}
	seen := make(map[string]int, len(inputs))
	replies := 0
	for _, event := range history {
		switch event.Role {
		case "user":
			if _, ok := inputs[event.Content]; !ok {
				return fmt.Errorf("unexpected user input %q", event.Content)
			}
			seen[event.Content]++
		case "assistant":
			if event.Content != reply {
				return fmt.Errorf("unexpected reply %q", event.Content)
			}
			replies++
		default:
			return fmt.Errorf("unexpected role %q", event.Role)
		}
	}
	for input := range inputs {
		if seen[input] != 1 {
			return fmt.Errorf("input %q appears %d times", input, seen[input])
		}
	}
	if replies != len(inputs) {
		return fmt.Errorf("history has %d replies, want %d", replies, len(inputs))
	}
	return nil
}

func sclMandatoryBaselineError(system string) error {
	for _, required := range []string{
		"You are codeNERD, a logic-first coding agent",
		"Constitutional prime directive",
		"Default deny: an action whose safety cannot be determined is blocked",
		"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE",
	} {
		if !strings.Contains(system, required) {
			return fmt.Errorf("embedded mandatory baseline missing %q", required)
		}
	}
	return nil
}

func TestCleanExpectationsHistoryOracle(t *testing.T) {
	inputs := map[string]struct{}{"first": {}, "second": {}}
	valid := []perception.ConversationTurn{{Role: "user", Content: "first"}, {Role: "assistant", Content: "reply"}, {Role: "user", Content: "second"}, {Role: "assistant", Content: "reply"}}
	for _, mode := range []string{"valid", "reordered", "missing", "extra", "duplicate", "foreign", "role", "reply"} {
		t.Run(mode, func(t *testing.T) {
			events := append([]perception.ConversationTurn(nil), valid...)
			switch mode {
			case "reordered":
				events[0], events[1] = events[1], events[0]
			case "missing":
				events = events[:3]
			case "extra":
				events = append(events, valid[0])
			case "duplicate":
				events[2].Content = "first"
			case "foreign":
				events[2].Content = "foreign"
			case "role":
				events[1].Role = "system"
			case "reply":
				events[1].Content = "truncated"
			}
			err := sclCompletedHistoryError(events, inputs, "reply")
			if (mode == "valid" || mode == "reordered") != (err == nil) {
				t.Fatalf("history oracle accepted/rejected the wrong control: mode=%s err=%v", mode, err)
			}
		})
	}
}

func TestCleanExpectationsBaselineOracle(t *testing.T) {
	cc := prompt.NewCompilationContext()
	cc.IntentVerb = "/coder"
	canonical, err := prompt.AssembleEmbeddedBaselinePrompt(cc)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"valid", "empty", "legacy", "unrelated", "identity", "constitution", "deny", "protocol"} {
		t.Run(mode, func(t *testing.T) {
			system := canonical
			switch mode {
			case "empty":
				system = ""
			case "legacy":
				system = "You are an AI assistant helping with software development."
			case "unrelated":
				system = "successful JIT control"
			case "identity":
				system = strings.ReplaceAll(system, "You are codeNERD, a logic-first coding agent", "")
			case "constitution":
				system = strings.ReplaceAll(system, "Constitutional prime directive", "")
			case "deny":
				system = strings.ReplaceAll(system, "Default deny: an action whose safety cannot be determined is blocked", "")
			case "protocol":
				system = strings.ReplaceAll(system, "OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", "")
			}
			err := sclMandatoryBaselineError(system)
			if (mode == "valid") != (err == nil) {
				t.Fatalf("baseline oracle accepted/rejected the wrong control: mode=%s err=%v", mode, err)
			}
		})
	}
}

func TestCleanExpectationsHistoryStorageAndWindow(t *testing.T) {
	for _, mode := range []string{"default", "lowered", "disabled", "tiny_budget"} {
		t.Run(mode, func(t *testing.T) {
			model := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "whole reply"}}
			fixture := newSCLFixture(t, &sclMockTransducer{intentToReturn: "/coder"}, &sclBoundaryCompiler{}, &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{}}, model, nil)
			cfg := session.DefaultExecutorConfig()
			cfg.WorkspaceRoot = fixture.registry.WorkspaceRoot()
			cfg.EnableSafetyGate = false // Preserve the existing inert mechanics fixture.
			if mode == "lowered" {
				cfg.HistoryTurnWindow = 2
			} else if mode == "disabled" {
				cfg.HistoryTurnWindow = 0
			} else if mode == "tiny_budget" {
				cfg.HistoryCharBudget = 1
			}
			fixture.executor.SetConfig(cfg)
			inputs := make(map[string]struct{})
			var seed []perception.ConversationTurn
			for i := 0; i < 30; i++ {
				input := fmt.Sprintf("HISTORY_%02d_QUESTION", i)
				inputs[input] = struct{}{}
				seed = append(seed, perception.ConversationTurn{Role: "user", Content: input}, perception.ConversationTurn{Role: "assistant", Content: "whole reply"})
			}
			fixture.executor.SetHistory(seed)
			seed[0].Content = "caller-local seed mutation"
			const current = "CURRENT_QUESTION"
			inputs[current] = struct{}{}
			result, err := fixture.executor.Process(context.Background(), current)
			if err != nil || result == nil || result.Response != "whole reply" || model.Invocations() != 1 {
				t.Fatalf("history probe failed: result=%+v err=%v calls=%d", result, err, model.Invocations())
			}
			history := fixture.executor.GetHistory()
			if err := sclCompletedHistoryError(history, inputs, "whole reply"); err != nil {
				t.Fatal(err)
			}
			history[0].Role = "system"
			history[0].Content = "caller-local snapshot mutation"
			if err := sclCompletedHistoryError(fixture.executor.GetHistory(), inputs, "whole reply"); err != nil {
				t.Fatalf("GetHistory returned aliased storage: %v", err)
			}
			model.mu.Lock()
			requests := append([]string(nil), model.userPrompts...)
			model.mu.Unlock()
			if len(requests) != 1 || !strings.Contains(requests[0], current) {
				t.Fatalf("missing actual generating request: %v", requests)
			}
			request := requests[0]
			for i := 0; i < 30; i++ {
				input := fmt.Sprintf("HISTORY_%02d_QUESTION", i)
				want := mode != "disabled" && mode != "tiny_budget" && i >= 30-cfg.HistoryTurnWindow/2
				if strings.Contains(request, input) != want {
					t.Fatalf("generation window mismatch: mode=%s input=%s present=%v want=%v", mode, input, strings.Contains(request, input), want)
				}
			}
			if types.IsClamped(request) != (mode != "disabled") {
				t.Fatalf("history eviction notice mismatch: mode=%s request=%q", mode, request)
			}
		})
	}
}

type sclExpectationCompiler struct {
	mode      string
	cancel    context.CancelFunc
	calls     int
	expected  string
	available int
}

func (c *sclExpectationCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	c.calls++
	c.available = cc.AvailableTokens()
	if c.mode == "canceled" {
		c.cancel()
		return nil, ctx.Err()
	}
	if c.mode == "jit_success" {
		c.expected = "successful JIT control"
		return &prompt.CompilationResult{Prompt: c.expected}, nil
	}
	var err error
	c.expected, err = prompt.AssembleEmbeddedBaselinePrompt(cc)
	if err != nil {
		return nil, err
	}
	return nil, errors.New("controlled JIT failure")
}

type sclExpectationFactory struct {
	calls    int
	compiled string
}

func (f *sclExpectationFactory) Generate(_ context.Context, result *prompt.CompilationResult, _ ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	f.calls++
	if result != nil {
		f.compiled = result.Prompt
	}
	return &config.EffectiveAgentRuntimeConfig{}, nil
}

func TestCleanExpectationsBaselineRequest(t *testing.T) {
	for _, mode := range []string{"jit_error", "jit_success", "budget_refusal", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			compiler := &sclExpectationCompiler{mode: mode, cancel: cancel}
			factory := &sclExpectationFactory{}
			model := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "controlled completion"}}
			fixture := newSCLFixture(t, &sclMockTransducer{intentToReturn: "/coder"}, compiler, factory, model, nil)
			if mode == "budget_refusal" {
				cfg := session.DefaultExecutorConfig()
				cfg.WorkspaceRoot = fixture.registry.WorkspaceRoot()
				cfg.EnableSafetyGate = false
				cfg.TokenBudget = 1
				fixture.executor.SetConfig(cfg)
			}
			result, err := fixture.executor.Process(ctx, "observe the current prompt contract")
			if compiler.calls != 1 {
				t.Fatalf("compiler calls=%d, want 1", compiler.calls)
			}
			if mode == "budget_refusal" || mode == "canceled" {
				if err == nil || result == nil || result.Error == nil || factory.calls != 0 || model.Invocations() != 0 {
					t.Fatalf("refusal reached config/provider: mode=%s result=%+v err=%v config=%d model=%d", mode, result, err, factory.calls, model.Invocations())
				}
				if mode == "canceled" && (!errors.Is(err, context.Canceled) || !errors.Is(result.Error, context.Canceled)) {
					t.Fatalf("owner cancellation was not preserved: result=%+v err=%v", result, err)
				}
				if mode == "budget_refusal" && !strings.Contains(err.Error(), "embedded mandatory baseline") {
					t.Fatalf("wrong budget refusal: %v", err)
				}
				return
			}
			prompts := model.SystemPrompts()
			if err != nil || result == nil || result.Response != "controlled completion" || factory.calls != 1 || len(prompts) != 1 || model.Invocations() != 1 {
				t.Fatalf("completion failed: mode=%s result=%+v err=%v config=%d prompts=%d model=%d", mode, result, err, factory.calls, len(prompts), model.Invocations())
			}
			if compiler.expected == "" || factory.compiled != compiler.expected || prompts[0] != compiler.expected {
				t.Fatalf("config/provider did not receive the exact selected prompt: mode=%s", mode)
			}
			if mode == "jit_error" {
				if err := sclMandatoryBaselineError(prompts[0]); err != nil {
					t.Fatal(err)
				}
				if prompt.EstimateTokens(prompts[0]) > compiler.available {
					t.Fatal("admitted baseline exceeds actual prompt budget")
				}
			}
		})
	}
}

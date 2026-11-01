//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type sclBoundaryCompiler struct {
	mu        sync.Mutex
	available []string
}

func (c *sclBoundaryCompiler) Compile(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	c.mu.Lock()
	c.available = slices.Clone(cc.AvailableTools)
	c.mu.Unlock()
	return &prompt.CompilationResult{Prompt: "inert catalog boundary fixture"}, nil
}

type sclBoundaryModel struct {
	*sclMockLLMClient
	mu      sync.Mutex
	offered []string
}

func (m *sclBoundaryModel) CompleteWithTools(ctx context.Context, system, input string, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	for _, d := range definitions {
		m.offered = append(m.offered, d.Name)
	}
	m.mu.Unlock()
	return m.sclMockLLMClient.CompleteWithTools(ctx, system, input, definitions)
}

type sclBoundaryFactory struct {
	delegate *sclMockConfigFactory
	calls    atomic.Int64
	entered  chan struct{}
	release  chan struct{}
}

func (f *sclBoundaryFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	f.calls.Add(1)
	if f.entered != nil {
		select {
		case f.entered <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		select {
		case <-f.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.delegate.Generate(ctx, result, intents...)
}

func TestCleanCatalogFixtureOwnership(t *testing.T) {
	prior := tools.Global()
	priorRoot, priorDummy, priorHanging := prior.WorkspaceRoot(), prior.Get("dummy_tool"), prior.Get("hanging_tool")
	factory := &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"dummy_tool"}}}
	model := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "owned read", ToolCalls: []types.ToolCall{{ID: "owned-read", Name: "dummy_tool"}}}}
	executor := setupExecutor(t, &sclMockTransducer{intentToReturn: "/coder"}, &sclMockJITCompiler{promptToReturn: &prompt.CompilationResult{Prompt: "owned prompt"}}, factory, model)
	if tools.Global() != prior || prior.WorkspaceRoot() != priorRoot || prior.Get("dummy_tool") != priorDummy || prior.Get("hanging_tool") != priorHanging {
		t.Error("fixture construction changed another owner's global registry")
	}
	later := tools.NewRegistry()
	var laterCalls atomic.Int64
	if err := later.Register(&tools.Tool{Name: "dummy_tool", Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) {
		laterCalls.Add(1)
		return "later owner", nil
	}}); err != nil {
		t.Fatal(err)
	}
	restore := tools.SwapGlobal(later)
	t.Cleanup(func() {
		if tools.Global() == later {
			restore()
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for turn := 0; turn < 2; turn++ {
		result, err := executor.Process(ctx, "read with the owned registry")
		if err != nil || result == nil || result.ToolCallsExecuted != 1 || result.SuccessfulToolCalls != 1 || laterCalls.Load() != 0 {
			t.Fatalf("declared read was dropped or redirected: result=%+v err=%v later=%d", result, err, laterCalls.Load())
		}
	}
	if tools.Global() != later || !slices.Equal(factory.configToReturn.AllowedTools, []string{"dummy_tool"}) {
		t.Fatal("fixture execution changed the later registry or factory ceiling")
	}
}

func TestCleanCatalogModelBoundary(t *testing.T) {
	for _, mode := range []string{"declared", "opaque", "empty_catalog", "empty_ceiling", "registered_ungranted", "factory_error", "factory_panic", "nil_config", "real_kernel_refusal"} {
		t.Run(mode, func(t *testing.T) {
			ceiling := []string{"dummy_tool"}
			if mode == "empty_ceiling" {
				ceiling = nil
			}
			factory := &sclBoundaryFactory{delegate: &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"dummy_tool"}}}}
			if mode == "factory_error" {
				factory.delegate.errToReturn = errors.New("controlled config failure")
			} else if mode == "factory_panic" {
				factory.delegate.panicMode = true
			} else if mode == "nil_config" {
				factory.delegate.configToReturn = nil
			}
			name := "dummy_tool"
			if mode == "registered_ungranted" {
				name = mode
			}
			opaque := make(chan int)
			args := map[string]any{}
			if mode == "opaque" {
				args["opaque"] = opaque
			}
			model := &sclBoundaryModel{sclMockLLMClient: &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "inert boundary response", ToolCalls: []types.ToolCall{{ID: "boundary-read", Name: name, Input: args}}}}}
			compiler := &sclBoundaryCompiler{}
			fixture := newSCLFixture(t, &sclMockTransducer{intentToReturn: "/coder"}, compiler, factory, model, ceiling)
			var callbacks atomic.Int64
			fixture.registry.Get("dummy_tool").Execute = func(_ context.Context, input map[string]any) (string, error) {
				callbacks.Add(1)
				if mode == "opaque" && input["opaque"] != opaque {
					t.Error("modular Go dispatch changed the opaque argument")
				}
				return "actual read callback", nil
			}
			if err := fixture.registry.Register(&tools.Tool{Name: "registered_ungranted", Effect: tools.EffectRead, Execute: func(context.Context, map[string]any) (string, error) {
				callbacks.Add(1)
				return "ungranted callback", nil
			}}); err != nil {
				t.Fatal(err)
			}
			if mode == "empty_catalog" {
				fixture.kernel.queryResults["turn_catalog"] = nil
			}
			executor := fixture.executor
			if mode == "real_kernel_refusal" {
				kernel, err := core.NewRealKernel()
				if err != nil {
					t.Fatal(err)
				}
				kernel.SetWorkspace(fixture.registry.WorkspaceRoot())
				executor = session.NewExecutor(kernel, nil, model, compiler, factory, &sclMockTransducer{intentToReturn: "/coder"})
				cfg := session.DefaultExecutorConfig()
				cfg.WorkspaceRoot = fixture.registry.WorkspaceRoot()
				executor.SetConfig(cfg) // Keep production constitutional safety enabled.
				executor.SetModularTools(fixture.registry)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := executor.Process(ctx, "perform the inert boundary read")
			if result == nil {
				t.Fatalf("no measured result: %v", err)
			}
			positive := mode == "declared" || mode == "opaque"
			if positive {
				if err != nil || callbacks.Load() != 1 || result.SuccessfulToolCalls != 1 {
					t.Fatalf("declared actual callback missing: result=%+v err=%v callbacks=%d", result, err, callbacks.Load())
				}
			} else if callbacks.Load() != 0 || result.SuccessfulToolCalls != 0 {
				t.Fatalf("refusal dispatched a callback: mode=%s result=%+v callbacks=%d", mode, result, callbacks.Load())
			}
			wantCompile, wantOffer := []string{"dummy_tool"}, []string{"dummy_tool"}
			if mode == "empty_catalog" || mode == "empty_ceiling" || mode == "real_kernel_refusal" {
				wantCompile, wantOffer = nil, nil
			} else if mode == "factory_error" || mode == "factory_panic" || mode == "nil_config" {
				wantOffer = nil // Generation failure narrows the actual request.
			}
			if !reflect.DeepEqual(compiler.available, wantCompile) || !reflect.DeepEqual(model.offered, wantOffer) || factory.calls.Load() != 1 {
				t.Fatalf("compile/config/provider contract changed: mode=%s compile=%v offer=%v generate=%d", mode, compiler.available, model.offered, factory.calls.Load())
			}
		})
	}
}

func TestCleanCatalogLiveTurnBoundary(t *testing.T) {
	ceiling := []string{"dummy_tool"}
	fixture := newSCLFixture(t, &sclMockTransducer{intentToReturn: "/coder"}, &sclBoundaryCompiler{}, &sclMockConfigFactory{}, &sclMockLLMClient{}, ceiling)
	kernel := fixture.kernel
	ceiling[0] = "registered_ungranted"
	for _, turn := range []string{"/turn_1_1", "/turn_1_2"} {
		if err := kernel.Assert(types.Fact{Predicate: "turn_verb", Args: []any{types.MangleAtom(turn), types.MangleAtom("/coder")}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := kernel.RetractFact(types.Fact{Predicate: "turn_verb", Args: []any{types.MangleAtom("/turn_1_1")}}); err != nil {
		t.Fatal(err)
	}
	for _, turn := range []string{"/turn_1_1", "/turn_9_9"} {
		if names, err := prompt.DeriveTurnTools(kernel, "/coder", turn); err == nil || len(names) != 0 {
			t.Fatalf("absent turn obtained a catalog: %s %v %v", turn, names, err)
		}
	}
	if names, err := prompt.DeriveTurnTools(kernel, "/coder", "/turn_1_2"); err != nil || !slices.Equal(names, []string{"dummy_tool"}) {
		t.Fatalf("live sibling lost its copied ceiling: %v %v", names, err)
	}
}

func TestCleanCatalogConcurrentFactoryOwnership(t *testing.T) {
	const workers = 4
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	factory := &sclBoundaryFactory{delegate: &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"dummy_tool"}}}, entered: make(chan struct{}, workers), release: release}
	model := &sclMockLLMClient{responseToReturn: &types.LLMToolResponse{Text: "parallel read", ToolCalls: []types.ToolCall{{ID: "parallel-read", Name: "dummy_tool"}}}}
	fixture := newSCLFixture(t, &sclMockTransducer{intentToReturn: "/coder"}, &sclBoundaryCompiler{}, factory, model, []string{"dummy_tool"})
	var callbacks atomic.Int64
	fixture.registry.Get("dummy_tool").Execute = func(context.Context, map[string]any) (string, error) {
		callbacks.Add(1)
		return "actual parallel callback", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			result, err := fixture.executor.Process(ctx, "concurrent inert read")
			if err == nil && (result == nil || result.SuccessfulToolCalls != 1) {
				err = errors.New("concurrent declared read did not execute")
			}
			results <- err
		}()
	}
	for i := 0; i < workers; i++ {
		select {
		case <-factory.entered:
		case <-ctx.Done():
			t.Fatal("not every compile reached the unopened config barrier")
		}
	}
	unblock()
	for i := 0; i < workers; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("concurrent reads did not finish within their owner context")
		}
	}
	if callbacks.Load() != workers || factory.calls.Load() != workers || !slices.Equal(factory.delegate.configToReturn.AllowedTools, []string{"dummy_tool"}) {
		t.Fatalf("factory ownership lost callbacks or changed its template: callbacks=%d calls=%d template=%v", callbacks.Load(), factory.calls.Load(), factory.delegate.configToReturn.AllowedTools)
	}
}

func TestCleanCatalogFactorySnapshots(t *testing.T) {
	for _, mode := range []string{"production", "fixture"} {
		t.Run(mode, func(t *testing.T) {
			var factory session.ConfigFactory
			if mode == "production" {
				factory = prompt.NewConfigFactory(prompt.NewDefaultConfigAtomProvider())
			} else {
				factory = &sclMockConfigFactory{configToReturn: &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"dummy_tool"}, Policies: []string{"policy/coder_safety.mg"}}}
			}
			compiled := &prompt.CompilationResult{Prompt: "factory ownership control"}
			first, err := factory.Generate(context.Background(), compiled, "/explain")
			if err != nil {
				t.Fatal(err)
			}
			second, err := factory.Generate(context.Background(), compiled, "/explain")
			if err != nil {
				t.Fatal(err)
			}
			if first == nil || second == nil || first == second {
				t.Fatal("two generations shared one mutable config object")
			}
			wantTools, wantPolicies := slices.Clone(second.AllowedTools), slices.Clone(second.Policies)
			first.IdentityPrompt = "caller-local identity"
			if len(first.AllowedTools) != 0 {
				first.AllowedTools[0] = "caller-local tool"
			}
			if len(first.Policies) == 0 {
				t.Fatal("ownership control requires a real policy slice")
			}
			first.Policies[0] = "caller-local policy"
			third, err := factory.Generate(context.Background(), compiled, "/explain")
			if err != nil || third == first || third == second || !slices.Equal(second.AllowedTools, wantTools) || !slices.Equal(second.Policies, wantPolicies) || !slices.Equal(third.AllowedTools, wantTools) || !slices.Equal(third.Policies, wantPolicies) {
				t.Fatalf("caller mutation escaped its config: second=%+v third=%+v err=%v", second, third, err)
			}
		})
	}
}

func TestCleanCatalogResponseSnapshots(t *testing.T) {
	for _, mode := range []string{"owned", "nil", "error", "canceled", "loop"} {
		t.Run(mode, func(t *testing.T) {
			opaque := make(chan int)
			template := &types.LLMToolResponse{
				Text: "scripted reply", StopReason: "tool_use",
				Usage:          types.UsageMetadata{InputTokens: 7, OutputTokens: 3, TotalTokens: 10, Known: types.UsageTokens},
				ThoughtSummary: "summary", ThoughtSignature: "signature",
				ToolCalls: []types.ToolCall{{ID: "scripted-call", Name: "dummy_tool", Input: map[string]any{"opaque": opaque, "nested": []any{map[string]any{"key": "original"}}}}},
				Blocks:    []types.ContentBlock{{Kind: types.BlockToolUse, ID: "scripted-call", Name: "dummy_tool", Input: map[string]any{"opaque": opaque, "key": "original"}, Raw: []byte("raw")}},
			}
			model := &sclMockLLMClient{responseToReturn: template}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("scripted error")
			switch mode {
			case "nil":
				model.responseToReturn = nil
			case "error":
				model.errToReturn = sentinel
			case "canceled":
				cancel()
			case "loop":
				model.infiniteLoopMode = true
			}
			first, err := model.CompleteWithTools(ctx, "system", "first", nil)
			if mode == "canceled" {
				if first != nil || !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation changed: %+v %v", first, err)
				}
				return
			}
			if mode == "nil" {
				if first != nil || err != nil {
					t.Fatalf("nil response changed: %+v %v", first, err)
				}
				return
			}
			if mode == "error" {
				if !errors.Is(err, sentinel) || !reflect.DeepEqual(first, template) {
					t.Fatalf("response/error pair changed: %+v %v", first, err)
				}
				return
			}
			if err != nil || first == nil {
				t.Fatalf("missing scripted response: %+v %v", first, err)
			}
			second, err := model.CompleteWithTools(ctx, "system", "second", nil)
			if err != nil || second == nil || first == second || first == template || second == template {
				t.Fatalf("native calls shared a mutable response: %p %p %p %v", first, second, template, err)
			}
			if mode == "loop" {
				if first.ToolCalls[0].ID == second.ToolCalls[0].ID || first.ToolCalls[0].Name != "dummy_tool" || second.ToolCalls[0].Name != "dummy_tool" {
					t.Fatal("loop call identity changed")
				}
				return
			}
			if !reflect.DeepEqual(first, template) || !reflect.DeepEqual(second, template) {
				t.Fatal("scripted response content changed")
			}
			first.ControlsApplied, first.Text = true, "caller-local"
			first.ToolCalls[0].Name = "caller-local"
			first.ToolCalls[0].Input["nested"].([]any)[0].(map[string]any)["key"] = "caller-local"
			first.Blocks[0].Input["key"] = "caller-local"
			first.Blocks[0].Raw[0] = 'x'
			third, err := model.CompleteWithTools(ctx, "system", "third", nil)
			if err != nil || third == nil || third == first || third == second || !reflect.DeepEqual(second, template) || !reflect.DeepEqual(third, template) || template.ControlsApplied || template.Text != "scripted reply" || template.ToolCalls[0].Name != "dummy_tool" || template.ToolCalls[0].Input["nested"].([]any)[0].(map[string]any)["key"] != "original" || template.Blocks[0].Input["key"] != "original" || string(template.Blocks[0].Raw) != "raw" {
				t.Fatal("caller mutation escaped its response")
			}
			if second.ToolCalls[0].Input["opaque"] != opaque || third.Blocks[0].Input["opaque"] != opaque {
				t.Fatal("opaque Go argument identity changed")
			}
		})
	}
}

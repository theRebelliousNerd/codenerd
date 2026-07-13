package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

type cancelingPromptVectorSearcher struct {
	started chan struct{}
	once    sync.Once
}

func (s *cancelingPromptVectorSearcher) Search(ctx context.Context, _ string, _ int) ([]prompt.SearchResult, error) {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *cancelingPromptVectorSearcher) EmbedQuery(context.Context, string) ([]float32, error) {
	return nil, nil
}

func newPromptScopeTestKernel(t *testing.T) (*core.CortexKernel, *KernelAdapter) {
	t.Helper()

	cortex := core.NewCortexKernel("cortex")
	shard, err := core.NewKernelShard(core.KernelShardConfig{Domain: "cortex"})
	if err != nil {
		t.Fatalf("NewKernelShard: %v", err)
	}
	if err := cortex.RegisterShard(shard); err != nil {
		t.Fatalf("RegisterShard: %v", err)
	}
	return cortex, NewKernelAdapter(cortex)
}

func promptScopeTestCorpus() *prompt.EmbeddedCorpus {
	atoms := []*prompt.PromptAtom{
		{ID: "identity", Category: prompt.CategoryIdentity, Content: "SCOPE IDENTITY", IsMandatory: true, Priority: 100, TokenCount: 4},
		{ID: "protocol", Category: prompt.CategoryProtocol, Content: "SCOPE PROTOCOL", IsMandatory: true, Priority: 100, TokenCount: 4},
		{ID: "safety", Category: prompt.CategorySafety, Content: "SCOPE SAFETY", IsMandatory: true, Priority: 100, TokenCount: 4},
		{ID: "methodology", Category: prompt.CategoryMethodology, Content: "SCOPE METHOD", IsMandatory: true, Priority: 100, TokenCount: 4},
		{ID: "go-only", Category: prompt.CategoryLanguage, Content: "ONLY_GO_CONTEXT", IsMandatory: true, Priority: 90, TokenCount: 4, Languages: []string{"go"}},
		{ID: "python-only", Category: prompt.CategoryLanguage, Content: "ONLY_PYTHON_CONTEXT", IsMandatory: true, Priority: 90, TokenCount: 4, Languages: []string{"python"}},
		{ID: "retry", Category: prompt.CategoryProtocol, Content: "RETRY_TOOLS\n{{available_tools}}", IsMandatory: true, Priority: 95, TokenCount: 6, WorldStates: []string{"no_tool_call_retry"}},
	}
	return prompt.NewEmbeddedCorpus(atoms)
}

func assertNoLivePromptFacts(t *testing.T, kernel core.Kernel) {
	t.Helper()
	for _, predicate := range []string{
		"compile_context", "current_context", "atom", "prompt_atom",
		"atom_category", "atom_priority", "atom_tag", "is_mandatory",
		"vector_hit", "atom_requires", "atom_conflicts",
	} {
		facts, err := kernel.Query(predicate)
		if err != nil {
			t.Fatalf("Query(%s): %v", predicate, err)
		}
		if len(facts) != 0 {
			t.Fatalf("live kernel retained %d %s facts", len(facts), predicate)
		}
	}
}

func TestKernelAdapter_CompilationScopesIsolateConcurrentPrompts(t *testing.T) {
	live, adapter := newPromptScopeTestKernel(t)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(promptScopeTestCorpus()),
		prompt.WithKernel(adapter),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	type scenario struct {
		language string
		retry    bool
		tool     string
		want     string
		reject   string
	}
	scenarios := []scenario{
		{language: "/go", tool: "go_edit_0", want: "ONLY_GO_CONTEXT", reject: "ONLY_PYTHON_CONTEXT"},
		{language: "/python", retry: true, tool: "python_edit_1", want: "ONLY_PYTHON_CONTEXT", reject: "ONLY_GO_CONTEXT"},
		{language: "/go", retry: true, tool: "go_edit_2", want: "ONLY_GO_CONTEXT", reject: "ONLY_PYTHON_CONTEXT"},
		{language: "/python", tool: "python_edit_3", want: "ONLY_PYTHON_CONTEXT", reject: "ONLY_GO_CONTEXT"},
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(scenarios))
	for _, scenario := range scenarios {
		scenario := scenario
		wg.Add(1)
		go func() {
			defer wg.Done()
			cc := prompt.NewCompilationContext().WithLanguage(scenario.language)
			cc.IntentTarget = scenario.tool
			cc.AvailableTools = []string{scenario.tool}
			cc.PreviousAttemptNoToolCall = scenario.retry
			result, compileErr := compiler.Compile(context.Background(), cc)
			if compileErr != nil {
				errs <- fmt.Errorf("compile %s: %w", scenario.language, compileErr)
				return
			}
			if !strings.Contains(result.Prompt, scenario.want) || strings.Contains(result.Prompt, scenario.reject) {
				errs <- fmt.Errorf("compile %s crossed context boundary: %q", scenario.language, result.Prompt)
				return
			}
			if scenario.retry {
				if !strings.Contains(result.Prompt, "RETRY_TOOLS") || !strings.Contains(result.Prompt, scenario.tool) {
					errs <- fmt.Errorf("retry compile %s omitted exact tool %q", scenario.language, scenario.tool)
				}
			} else if strings.Contains(result.Prompt, "RETRY_TOOLS") {
				errs <- fmt.Errorf("non-retry compile %s included retry atom", scenario.language)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	assertNoLivePromptFacts(t, live)
}

func TestKernelAdapter_CompilationScopeDoesNotLeakOnBudgetError(t *testing.T) {
	live, adapter := newPromptScopeTestKernel(t)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(promptScopeTestCorpus()),
		prompt.WithKernel(adapter),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	cc := prompt.NewCompilationContext().WithLanguage("/go").WithTokenBudget(1000, 600)
	if _, err := compiler.Compile(context.Background(), cc); err == nil {
		t.Fatal("Compile() error = nil, want budget/headroom error after selection")
	}
	assertNoLivePromptFacts(t, live)
}

func TestKernelAdapter_CompilationScopeDoesNotLeakOnCancellation(t *testing.T) {
	live, adapter := newPromptScopeTestKernel(t)
	searcher := &cancelingPromptVectorSearcher{started: make(chan struct{})}
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(promptScopeTestCorpus()),
		prompt.WithKernel(adapter),
		prompt.WithVectorSearcher(searcher),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		cc := prompt.NewCompilationContext().WithLanguage("/go").WithSemanticQuery("cancel me", 5)
		_, compileErr := compiler.Compile(ctx, cc)
		done <- compileErr
	}()

	<-searcher.started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Compile() error = %v, want context.Canceled", err)
	}
	assertNoLivePromptFacts(t, live)
}

func TestKernelAdapter_RetryContextBypassesPreRetryCache(t *testing.T) {
	live, adapter := newPromptScopeTestKernel(t)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(promptScopeTestCorpus()),
		prompt.WithKernel(adapter),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	first := prompt.NewCompilationContext().WithLanguage("/go")
	first.AvailableTools = []string{"read_file"}
	firstResult, err := compiler.Compile(context.Background(), first)
	if err != nil {
		t.Fatalf("initial Compile: %v", err)
	}
	if strings.Contains(firstResult.Prompt, "RETRY_TOOLS") {
		t.Fatal("initial prompt unexpectedly contained retry nudge")
	}

	retry := first.Clone()
	retry.PreviousAttemptNoToolCall = true
	retry.AvailableTools = []string{"write_file", "run_command"}
	retryResult, err := compiler.Compile(context.Background(), retry)
	if err != nil {
		t.Fatalf("retry Compile: %v", err)
	}
	for _, want := range []string{"RETRY_TOOLS", "write_file", "run_command"} {
		if !strings.Contains(retryResult.Prompt, want) {
			t.Fatalf("retry prompt omitted %q: %q", want, retryResult.Prompt)
		}
	}
	if strings.Contains(retryResult.Prompt, "`read_file`") {
		t.Fatalf("retry prompt reused stale pre-retry tool surface: %q", retryResult.Prompt)
	}
	assertNoLivePromptFacts(t, live)
}

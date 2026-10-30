package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// scopeConsultant answers each consultation with a numbered marker and keeps
// the tasks it was asked.
type scopeConsultant struct {
	mu    sync.Mutex
	tasks []string
}

func (c *scopeConsultant) SpawnConsultation(_ context.Context, _ string, task string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tasks = append(c.tasks, task)
	return fmt.Sprintf("SCOPE_ADVICE_%d", len(c.tasks)), nil
}

// An adviser consulted for a.go was selected again for a later step on b.go,
// but consultation was deduplicated by agent and its card dealt only to a.go:
// the b.go step got neither the answer nor a new one. Deduplication is now by
// covered scope, so the b.go step gets a supplement scoped to b.go, and the
// a.go card stays out of it.
func TestPlannedStep_AnAdviserSelectedForANewFileIsAskedAboutIt(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{experts.AgentAtom("UIArchitect")}},
		{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("UIArchitect"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_prefix", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		{Predicate: "dependency_link", Args: []any{"internal/ui/a.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
		{Predicate: "dependency_link", Args: []any{"internal/ui/b.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	e := NewExecutor(k, nil, nil, &MockJITCompiler{CompileFunc: func(_ context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		return &prompt.CompilationResult{Prompt: "step prompt"}, nil
	}}, nil, nil)
	e.config.WorkspaceRoot = t.TempDir()
	consultant := &scopeConsultant{}
	e.SetExpertConsultant(consultant)

	sel := e.selectExperts("/fix", "internal/ui/a.go")
	if len(sel.Needs) != 1 || sel.Needs[0].Spawn != experts.SpawnAdvise {
		t.Fatalf("fixture: want one adviser for a.go, got %+v", sel.Needs)
	}
	result := &ExecutionResult{}
	e.withExpertConsultations(t.Context(), "turn prompt", perception.Intent{Verb: "/fix", Target: "internal/ui/a.go"}, "fix the ui", sel, result, nil)

	stepCtx := prompt.NewCompilationContext()
	stepCtx.IntentVerb, stepCtx.IntentTarget = "/fix", "internal/ui/b.go"
	stepPrompt := e.stepSystemPrompt(t.Context(), "turn prompt", stepCtx, nil, "fix the ui", result)

	if len(consultant.tasks) != 2 || !strings.Contains(consultant.tasks[1], "Files in your domain: internal/ui/b.go") {
		t.Fatalf("the adviser was not asked about b.go: %q", consultant.tasks)
	}
	if strings.Contains(consultant.tasks[1], "internal/ui/a.go,") || strings.Contains(consultant.tasks[1], "domain: internal/ui/a.go") {
		t.Fatalf("the supplement re-asked about the covered file: %q", consultant.tasks[1])
	}
	if !strings.Contains(stepPrompt, "SCOPE_ADVICE_2") {
		t.Fatalf("the b.go step lacks its supplement:\n%s", stepPrompt)
	}
	if strings.Contains(stepPrompt, "SCOPE_ADVICE_1") {
		t.Fatalf("the a.go card leaked into the b.go step:\n%s", stepPrompt)
	}

	// A third step on a.go is dealt the first card and asks no one.
	stepCtx.IntentTarget = "internal/ui/a.go"
	again := e.stepSystemPrompt(t.Context(), "turn prompt", stepCtx, nil, "fix the ui", result)
	if len(consultant.tasks) != 2 || !strings.Contains(again, "SCOPE_ADVICE_1") || strings.Contains(again, "SCOPE_ADVICE_2") {
		t.Fatalf("the a.go step was not dealt its own card alone (%d asks):\n%s", len(consultant.tasks), again)
	}
}

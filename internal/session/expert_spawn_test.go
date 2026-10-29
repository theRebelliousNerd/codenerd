package session

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// systemCapturingProvider answers every call with a final text and keeps the
// system prompts it was sent.
type systemCapturingProvider struct {
	*MockLLMClient
	mu      sync.Mutex
	systems []string
}

func (p *systemCapturingProvider) CompleteWithToolResults(_ context.Context, system string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.mu.Lock()
	p.systems = append(p.systems, system)
	p.mu.Unlock()
	return &types.LLMToolResponse{Text: "done"}, nil
}

func (p *systemCapturingProvider) firstSystem(t *testing.T) string {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.systems) == 0 {
		t.Fatal("the model was never called")
	}
	return p.systems[0]
}

// recordingConsultant stands in for JITExecutor.SpawnConsultation and keeps
// what each consulted agent was asked.
type recordingConsultant struct {
	mu    sync.Mutex
	asked map[string]string
}

func (c *recordingConsultant) SpawnConsultation(_ context.Context, agent, task string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.asked == nil {
		c.asked = map[string]string{}
	}
	c.asked[agent] = task
	return "TEA_EXPERT_PLAN: keep Update pure, return commands", nil
}

// spawnRosterKernel is a kernel whose roster holds the Bubble Tea expert (an
// executor in policy/shards.mg) and a file in the world model importing it.
func spawnRosterKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{experts.AgentAtom("BubbleTeaExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("BubbleTeaExpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_prefix", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		{Predicate: "dependency_link", Args: []any{"cmd/nerd/chat/view.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	return k
}

func spawnTurn(t *testing.T, verb string, consultant ExpertConsultant) *systemCapturingProvider {
	t.Helper()
	k := spawnRosterKernel(t)
	client := &systemCapturingProvider{MockLLMClient: &MockLLMClient{}}
	executor := NewExecutor(k, &testExecutiveStore{}, client, &MockJITCompiler{}, journeyConfigFactory(), journeyIntent(verb, "/query"))
	cfg := journeyConfig(t)
	cfg.WorkspaceRoot = t.TempDir()
	executor.SetConfig(cfg)
	if consultant != nil {
		executor.SetExpertConsultant(consultant)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := executor.CloneForTask().ProcessWithIntent(ctx, "explain how the chat view renders",
		&perception.Intent{Verb: verb, Category: "/query", Target: "cmd/nerd/chat/view.go"}); err != nil {
		t.Logf("turn error (not under test): %v", err)
	}
	return client
}

// Spawn second on the executor path: a turn whose file imports the Bubble Tea
// expert's exact subject consults that expert (expert_spawn /lead) through
// the consultation spawner before the model is called, and the model reads
// the expert's answer in its system prompt. The clone a `nerd fix` runs on
// carries the spawner. Before expert_spawn the executor never consulted a
// shard agent; only two chat paths did.
func TestTurnConsultsTheShardAgentTheKernelSpawns(t *testing.T) {
	consultant := &recordingConsultant{}
	client := spawnTurn(t, "/explain", consultant)

	asked, ok := consultant.asked["bubbleteaexpert"]
	if !ok {
		t.Fatalf("the Bubble Tea expert was not consulted; asked %v", consultant.asked)
	}
	if !strings.Contains(asked, "Lay out how to carry out this task") || !strings.Contains(asked, "cmd/nerd/chat/view.go") {
		t.Errorf("the lead consultation was asked %q", asked)
	}
	system := client.firstSystem(t)
	if !strings.Contains(system, "## Shard agent consultation") || !strings.Contains(system, "TEA_EXPERT_PLAN") {
		t.Errorf("the model's system prompt lacks the consultation:\n%s", system)
	}
}

// A turn that already runs as the shard agent (a consultation) consults no
// one: consultations never recurse.
func TestConsultationTurnSpawnsNoConsultation(t *testing.T) {
	consultant := &recordingConsultant{}
	spawnTurn(t, "/consult/bubbleteaexpert", consultant)
	if len(consultant.asked) != 0 {
		t.Errorf("a consultation turn consulted %v", consultant.asked)
	}
}

// With no spawner wired the turn still runs and is told which consultation it
// is missing, never silently.
func TestTurnWithoutAConsultantNamesTheMissingConsultation(t *testing.T) {
	client := spawnTurn(t, "/explain", nil)
	system := client.firstSystem(t)
	if !strings.Contains(system, "bubbleteaexpert") || !strings.Contains(system, "did not complete") {
		t.Errorf("the missing consultation is not named:\n%s", system)
	}
}

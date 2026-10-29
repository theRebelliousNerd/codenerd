package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// cappedLLMClient is a slot client that reports an input cap, as a worker
// slot's Anthropic client does for max_input_tokens.
type cappedLLMClient struct {
	*MockLLMClient
	window int
}

func (c *cappedLLMClient) InputWindowTokens() int { return c.window }

func newCappedClient(window int) *cappedLLMClient {
	return &cappedLLMClient{MockLLMClient: &MockLLMClient{}, window: window}
}

// The window is the session budget lowered to the serving client's cap, and
// says which of the two bound it.
func TestInputBudgetFor(t *testing.T) {
	tests := []struct {
		name        string
		tokenBudget int
		slotCap     int
		want        inputBudget
	}{
		{name: "no cap leaves the session budget", tokenBudget: 200000, want: inputBudget{tokens: 200000}},
		{name: "a smaller cap binds", tokenBudget: 200000, slotCap: 100000, want: inputBudget{tokens: 100000, slotCapped: true}},
		{name: "a larger cap does not raise the budget", tokenBudget: 200000, slotCap: 300000, want: inputBudget{tokens: 200000}},
		{name: "an equal cap is the session budget", tokenBudget: 100000, slotCap: 100000, want: inputBudget{tokens: 100000}},
		{name: "an unset budget takes the default before the cap", slotCap: 9000, want: inputBudget{tokens: 9000, slotCapped: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newWorkingLoopExecutor(t, &MockLLMClient{})
			e.config.TokenBudget = tt.tokenBudget
			if got := e.inputBudgetFor(tt.slotCap); got != tt.want {
				t.Errorf("inputBudgetFor(%d) = %+v, want %+v", tt.slotCap, got, tt.want)
			}
		})
	}
}

// A working loop is sized to the client the turn resolved: a request that
// fits the session budget but not the serving slot's max_input_tokens has its
// largest result archived, the holographic render budget shrinks with it, and
// a request that cannot be fit at all is refused naming the slot cap.
func TestWorkingLoop_IsSizedToTheServingSlotsCap(t *testing.T) {
	t.Parallel()
	const sessionBudget, slotCap = 200000, 20000
	body := strings.Repeat("b", 4*30000) // about 30000 tokens: under the session budget, over the cap

	tests := []struct {
		name         string
		client       types.LLMClient
		wantArchived bool
		wantRender   int
	}{
		{name: "uncapped client", client: &MockLLMClient{}, wantRender: sessionBudget * nerdconfig.BytesPerToken},
		{name: "capped worker", client: newCappedClient(slotCap), wantArchived: true, wantRender: slotCap * nerdconfig.BytesPerToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newWorkingLoopExecutor(t, tt.client)
			e.config.TokenBudget = sessionBudget
			if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, "target.go"), []byte("package target\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), tt.client, "fix target.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "target.go"})
			if err != nil {
				t.Fatalf("beginWorkingLoop: %v", err)
			}
			t.Cleanup(closeLoop)
			loop := activeWorkingLoop(ctx)
			if got := e.holographicBudgetBytes(loop); got != tt.wantRender {
				t.Errorf("holographicBudgetBytes = %d, want %d", got, tt.wantRender)
			}

			call := types.ToolCall{ID: "call-1", Name: "read_file", Input: map[string]any{"path": "target.go"}}
			if err := e.recordWorkingResult(ctx, call, body, nil); err != nil {
				t.Fatalf("recordWorkingResult: %v", err)
			}
			history := []types.Message{
				{Role: "user", Text: "fix target.go"},
				{Role: "assistant", ToolCalls: []types.ToolCall{call}},
				{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: body}}},
			}
			messages, err := e.prepareWorkingRequest(ctx, "SYSTEM", history, nil)
			if err != nil {
				t.Fatalf("prepareWorkingRequest: %v", err)
			}
			got := lastResultOf(t, messages, call.ID)
			if archived := strings.HasPrefix(got, archivedResultPrefix); archived != tt.wantArchived {
				t.Errorf("result archived = %v, want %v", archived, tt.wantArchived)
			}

			_, err = e.singleShotRequest(ctx, "SYSTEM", body, nil)
			if !tt.wantArchived {
				if err != nil {
					t.Fatalf("a task under the session budget was refused: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrInputBudgetExceeded) {
				t.Fatalf("singleShotRequest err = %v, want ErrInputBudgetExceeded", err)
			}
			if !strings.Contains(err.Error(), "max_input_tokens") || !strings.Contains(err.Error(), "the budget is 20000") {
				t.Errorf("the refusal does not name the slot cap as the binding limit: %v", err)
			}
		})
	}
}

// The turn's prompt is compiled for the client that will be sent it: a verb
// the worker serves gets the worker's cap, one the planner serves the session
// budget.
func TestBuildCompilationContext_BudgetFollowsTheServingSlot(t *testing.T) {
	worker := newCappedClient(20000)
	planner := &MockLLMClient{}
	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, worker, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.SetPlannerClient(planner)
	e.config.TokenBudget = 200000
	e.reasoningVerbCache.Store("/fix", false)
	e.reasoningVerbCache.Store("/review", true)

	tests := []struct {
		verb string
		want int
	}{
		{verb: "/fix", want: 20000},
		{verb: "/review", want: 200000},
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			cc := e.buildCompilationContext(context.Background(), perception.Intent{Verb: tt.verb})
			if cc.TokenBudget != tt.want {
				t.Errorf("compile budget for %s = %d, want %d", tt.verb, cc.TokenBudget, tt.want)
			}
		})
	}
}

// A subagent spawned onto a capped slot is compiled for the window it will be
// sent, not the session's.
func TestSpawner_CompilesSubagentsForTheServingSlotsCap(t *testing.T) {
	tests := []struct {
		name   string
		client types.LLMClient
		want   int
	}{
		{name: "capped worker", client: newCappedClient(20000), want: 20000},
		{name: "uncapped client", client: &MockLLMClient{}, want: 200000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var budgets []int
			jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				budgets = append(budgets, cc.TokenBudget)
				return &prompt.CompilationResult{Prompt: "compiled"}, nil
			}}
			spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, tt.client, jit, &MockConfigFactory{}, &MockTransducer{},
				SpawnerConfig{MaxActiveSubagents: 2, TokenBudget: 200000})
			if _, err := spawner.Spawn(context.Background(), SpawnRequest{
				Name: "capped", Task: "fix it", Type: SubAgentTypeEphemeral, IntentVerb: "/fix",
			}); err != nil {
				t.Fatalf("Spawn: %v", err)
			}
			if len(budgets) == 0 {
				t.Fatal("the spawn compiled no prompt")
			}
			for _, got := range budgets {
				if got != tt.want {
					t.Errorf("subagent compile budget = %d, want %d", got, tt.want)
				}
			}
		})
	}
}

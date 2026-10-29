package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/experts"
	"codenerd/internal/types"
)

// critiqueConsultant answers the lead consultation with a plan and the
// critique with one finding on a changed line, and keeps what it was asked.
type critiqueConsultant struct {
	mu    sync.Mutex
	asked []string
}

func (c *critiqueConsultant) SpawnConsultation(_ context.Context, agent, task string) (string, error) {
	c.mu.Lock()
	c.asked = append(c.asked, agent+": "+task)
	c.mu.Unlock()
	if strings.Contains(task, "review this change") {
		return "FINDING main.go:4 high: Double must clamp before it doubles, the view model overflows", nil
	}
	return "Plan: keep Double pure.", nil
}

const expertRepairAsk = "The shard agents whose domain this change is in reviewed"

// A /fix whose target imports the Bubble Tea expert's subject owes that
// expert's critique (expert_critique: /fix's mode is advise, execute,
// critique). After the turn writes, the /expert_critique round has the expert
// review the change through the consultation path, and its on-change high
// finding comes back to the model as a repair round, whose edit lands. Before
// this the executor's critique stage did not exist: the expert led, and
// nobody from its domain looked at what was written.
func TestExpertCritiqueRoundHandsTheFindingBack(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("needs the go toolchain")
	}
	consultant := &critiqueConsultant{}
	h := newRepairHarness(t, func(e *Executor) { e.SetExpertConsultant(consultant) })
	if err := h.executor.kernel.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{experts.AgentAtom("BubbleTeaExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("BubbleTeaExpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_exact", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		{Predicate: "dependency_link", Args: []any{"main.go", "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}

	repairPrompt := ""
	round := 0
	writes := func(files map[string]string) *types.LLMToolResponse {
		round++
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		slices.Sort(names)
		resp := &types.LLMToolResponse{Text: "writing"}
		for i, name := range names {
			resp.ToolCalls = append(resp.ToolCalls, h.writeCall(fmt.Sprintf("w%d-%d", round, i), "write_file", filepath.Join(h.ws, name), files[name]))
		}
		return resp
	}
	initial := map[string]string{"main.go": closureDouble, "main_test.go": closureTestDouble}
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithSystemFunc: func(context.Context, string, string) (string, error) { return "NO FINDINGS", nil },
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return writes(initial), nil
			},
		},
		CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if !slices.ContainsFunc(history, func(m types.Message) bool { return len(m.ToolResults) > 0 }) {
				return writes(initial), nil
			}
			if last := history[len(history)-1]; strings.Contains(last.Text, expertRepairAsk) && repairPrompt == "" {
				repairPrompt = last.Text
				return writes(map[string]string{"main.go": strings.Replace(closureDouble, "return x * 2", "return x * 2 // clamped by the caller", 1)}), nil
			}
			return &types.LLMToolResponse{Text: "done"}, nil
		},
	}
	preset := presetIntentForTask("/fix", "fix the Double helper", "main.go")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := h.executor.ProcessWithIntent(ctx, "fix the Double helper", preset)
	if err != nil {
		t.Logf("turn error (not under test): %v", err)
	}

	consultant.mu.Lock()
	asked := slices.Clone(consultant.asked)
	consultant.mu.Unlock()
	if !slices.ContainsFunc(asked, func(a string) bool {
		return strings.HasPrefix(a, "bubbleteaexpert: ") && strings.Contains(a, "review this change")
	}) {
		t.Fatalf("the expert was not asked to review the change; asked %q", asked)
	}
	if !strings.Contains(repairPrompt, "[bubbleteaexpert] main.go:4 [high]") {
		t.Fatalf("the expert's finding did not come back as a repair round; prompt %q", repairPrompt)
	}
	if result == nil || len(result.ExpertFindings) != 1 {
		t.Fatalf("ExpertFindings = %+v, want the one finding", result)
	}
	if data, err := os.ReadFile(filepath.Join(h.ws, "main.go")); err != nil || !strings.Contains(string(data), "clamped by the caller") {
		t.Errorf("the repair round's edit did not land: %q (%v)", data, err)
	}
}

package session

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

func campaignCheckContext() context.Context {
	return tools.WithCampaignCheck(context.Background(), tools.CampaignCheck{
		CampaignID: "camp",
		TaskID:     "task",
		Argv:       []string{"go", "test", "./internal/session/"},
	})
}

func assertFact(t *testing.T, k types.Kernel, predicate string, args ...any) {
	t.Helper()
	if err := k.Assert(types.Fact{Predicate: predicate, Args: args}); err != nil {
		t.Fatalf("assert %s: %v", predicate, err)
	}
}

// A campaign check offers run_check only on the turn that carries it, and
// only when that turn's persona edits. The static envelope never gains it,
// and neither does a concurrent coder turn that was not handed the check.
func TestTurnCatalog_RunCheckFollowsTheDeclaredCheck(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	checked := newTurnAtom()
	plain := newTurnAtom()
	assertFact(t, k, "turn_verb", checked, types.MangleAtom("/fix"))
	assertFact(t, k, "turn_declared_check", checked)
	assertFact(t, k, "turn_verb", plain, types.MangleAtom("/fix"))

	with, err := prompt.DeriveTurnTools(k, "/fix", string(checked))
	if err != nil {
		t.Fatalf("checked turn: %v", err)
	}
	without, err := prompt.DeriveTurnTools(k, "/fix", string(plain))
	if err != nil {
		t.Fatalf("plain turn: %v", err)
	}
	static, err := prompt.DeriveTurnTools(k, "/fix")
	if err != nil {
		t.Fatalf("static /fix: %v", err)
	}
	if !slices.Contains(with, "run_check") {
		t.Fatalf("a /fix turn carrying a declared check was not offered run_check: %v", with)
	}
	if slices.Contains(without, "run_check") {
		t.Fatalf("a concurrent /fix turn with no check was offered run_check: %v", without)
	}
	if slices.Contains(static, "run_check") {
		t.Fatalf("the static /fix envelope gained run_check while another turn holds a check: %v", static)
	}
	if len(with) != len(without)+1 || len(without) != len(static) {
		t.Fatalf("catalog sizes checked=%d plain=%d static=%d, want the check to add exactly run_check", len(with), len(without), len(static))
	}
	emptyTurn, err := prompt.DeriveTurnTools(k, "/fix", "")
	if err != nil {
		t.Fatalf("empty turn: %v", err)
	}
	if !slices.Equal(emptyTurn, static) {
		t.Fatalf("an empty turn atom changed the catalog: %v", emptyTurn)
	}

	review := newTurnAtom()
	assertFact(t, k, "turn_verb", review, types.MangleAtom("/review"))
	assertFact(t, k, "turn_declared_check", review)
	reviewed, err := prompt.DeriveTurnTools(k, "/review", string(review))
	if err != nil {
		t.Fatalf("review turn: %v", err)
	}
	if slices.Contains(reviewed, "run_check") {
		t.Fatalf("a /review turn was offered run_check: %v", reviewed)
	}

	tester := newTurnAtom()
	assertFact(t, k, "turn_verb", tester, types.MangleAtom("/test"))
	assertFact(t, k, "turn_declared_check", tester)
	tested, err := prompt.DeriveTurnTools(k, "/test", string(tester))
	if err != nil {
		t.Fatalf("test turn: %v", err)
	}
	if !slices.Contains(tested, "run_check") {
		t.Fatalf("a /test turn carrying a declared check was not offered run_check: %v", tested)
	}

	if _, err := prompt.DeriveTurnTools(&MockKernel{}, "/fix", "/not_a_turn"); err == nil || !strings.Contains(err.Error(), "not a turn atom") {
		t.Fatalf("invalid turn atom err = %v, want it rejected before a query", err)
	}
}

// Through the turn: the executor asserts the check from the context and the
// model is offered run_check. A plain /fix is not. A precompiled allowlist
// is not re-derived, so a frozen catalog stays frozen.
func TestTurn_ACampaignCheckOffersRunCheck(t *testing.T) {
	registerProductionTools(t)
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	offer := func(t *testing.T, verb string, ctx context.Context, precompiled []string) (compiled, offered []string) {
		t.Helper()
		var mu sync.Mutex
		jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			mu.Lock()
			compiled = slices.Clone(cc.AvailableTools)
			mu.Unlock()
			return &prompt.CompilationResult{Prompt: "system"}, nil
		}}
		llm := &MockLLMClient{CompleteWithToolsFunc: func(_ context.Context, _, _ string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			mu.Lock()
			for _, d := range defs {
				offered = append(offered, d.Name)
			}
			mu.Unlock()
			return &types.LLMToolResponse{Text: "done"}, nil
		}}
		e := NewExecutor(k, &MockVirtualStore{}, llm, jit, &MockConfigFactory{}, &MockTransducer{})
		if precompiled != nil {
			e.EffectiveAgentRuntimeConfig = &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: precompiled}
		}
		preset := &perception.Intent{Verb: verb, Category: "/mutation", Target: "notes.md", Confidence: 1}
		_, _ = e.ProcessWithIntent(ctx, "fix the notes", preset)
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(compiled), slices.Clone(offered)
	}

	for _, tc := range []struct {
		name        string
		verb        string
		ctx         context.Context
		precompiled []string
		want        bool
	}{
		{"campaign /fix", "/fix", campaignCheckContext(), nil, true},
		{"plain /fix", "/fix", context.Background(), nil, false},
		{"campaign /test", "/test", campaignCheckContext(), nil, true},
		{"campaign /review", "/review", campaignCheckContext(), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled, offered := offer(t, tc.verb, tc.ctx, tc.precompiled)
			if len(offered) == 0 || len(compiled) == 0 {
				t.Fatalf("the turn offered nothing (compiled %v, offered %v); the test proves nothing", compiled, offered)
			}
			if slices.Contains(compiled, "run_check") != tc.want || slices.Contains(offered, "run_check") != tc.want {
				t.Fatalf("run_check compiled=%v offered=%v, want %v", slices.Contains(compiled, "run_check"), slices.Contains(offered, "run_check"), tc.want)
			}
		})
	}

	t.Run("a precompiled catalog is not re-derived", func(t *testing.T) {
		compiled, offered := offer(t, "/fix", campaignCheckContext(), []string{"read_file"})
		if slices.Contains(compiled, "run_check") || slices.Contains(offered, "run_check") {
			t.Fatalf("precompiled catalog was re-derived: compiled=%v offered=%v", compiled, offered)
		}
		if !slices.Equal(compiled, []string{"read_file"}) || len(offered) == 0 {
			t.Fatalf("precompiled catalog = compiled %v offered %v, want read_file and nothing else", compiled, offered)
		}
		for _, name := range offered {
			if name != "read_file" {
				t.Fatalf("precompiled catalog offered %q", name)
			}
		}
	})
}

func TestSpawn_OffersRunCheckOnlyToAnEditingPersonaWithACheck(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	spawn := func(t *testing.T, verb string, ctx context.Context) (compiled, allowed []string) {
		t.Helper()
		jit := &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			compiled = slices.Clone(cc.AvailableTools)
			return &prompt.CompilationResult{Prompt: "system"}, nil
		}}
		spawner := NewSpawner(k, &MockVirtualStore{}, &MockLLMClient{}, jit, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
		agent, err := spawner.Spawn(ctx, SpawnRequest{
			Name: "worker", Task: "fix the thing", Type: SubAgentTypeEphemeral, IntentVerb: verb,
		})
		if err != nil {
			t.Fatalf("Spawn %s: %v", verb, err)
		}
		allowed = agent.config.EffectiveAgentRuntimeConfig.AllowedTools
		if len(compiled) != len(allowed) {
			t.Errorf("compiled catalog %d drifted from AllowedTools %d", len(compiled), len(allowed))
		}
		return compiled, allowed
	}

	for _, tc := range []struct {
		name string
		verb string
		ctx  context.Context
		want bool
	}{
		{"/fix with a check", "/fix", campaignCheckContext(), true},
		{"/implement with a check", "/implement", campaignCheckContext(), true},
		{"/fix without a check", "/fix", context.Background(), false},
		{"/review with a check", "/review", campaignCheckContext(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled, allowed := spawn(t, tc.verb, tc.ctx)
			if len(allowed) == 0 {
				t.Fatal("empty catalog proves nothing")
			}
			if slices.Contains(compiled, "run_check") != tc.want || slices.Contains(allowed, "run_check") != tc.want {
				t.Fatalf("run_check compiled=%v allowed=%v, want %v", slices.Contains(compiled, "run_check"), slices.Contains(allowed, "run_check"), tc.want)
			}
		})
	}

	for _, pred := range []string{"turn_declared_check", "turn_verb"} {
		facts, err := k.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
		}
		if len(facts) != 0 {
			t.Fatalf("spawn left %s %v; the temporary turn facts must be retracted", pred, facts)
		}
	}
}

// The /check gate is the last run_check after the turn's last write, decided
// in Go the way /test_run is: a run before a write is not a verdict, and the
// receipt facts stay either way.
func TestCheckGate_IsTheLastRunAfterTheLastWrite(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(&tools.Tool{
		Effect: tools.EffectRead, Name: "probe_run_check", Category: tools.CategoryTest,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			code, _ := args["exit"].(int)
			tools.RecordAcceptanceRun(ctx, tools.AcceptanceRun{Argv: []string{"go", "test"}, ExitCode: code})
			return "ran", nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))

	write := func(id string) types.ToolCall {
		return types.ToolCall{ID: id, Name: "write_file", Input: map[string]any{"path": filepath.Join(ws, "notes.md"), "content": "x\n"}}
	}
	run := func(id string, exit int) types.ToolCall {
		return types.ToolCall{ID: id, Name: "probe_run_check", Input: map[string]any{"exit": exit}}
	}
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "probe_run_check"}}

	for _, tc := range []struct {
		name  string
		calls []types.ToolCall
		want  VerifyOutcome
	}{
		{"a run before the write is no verdict", []types.ToolCall{run("r", 0), write("w")}, VerifySkipped},
		{"the last run after the write passed", []types.ToolCall{write("w"), run("r1", 1), run("r2", 0)}, VerifyPassed},
		{"the last run after the write failed", []types.ToolCall{write("w"), run("r1", 0), run("r2", 1)}, VerifyFailed},
		{"a later write sets the verdict aside", []types.ToolCall{write("w1"), run("r", 0), write("w2")}, VerifySkipped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kernel := &MockKernel{}
			e := NewExecutor(kernel, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
			e.config.WorkspaceRoot = ws
			e.config.EnableSafetyGate = false
			result := &ExecutionResult{}
			for _, call := range tc.calls {
				e.executeToolBatch(context.Background(), []types.ToolCall{call}, cfg, result)
			}
			if result.SuccessfulWriteTools == 0 {
				t.Fatal("no write succeeded; the probe measured nothing")
			}
			if got := result.checkVerdict(); got != tc.want {
				t.Fatalf("check gate = %v, want %v", got, tc.want)
			}
			facts, err := kernel.Query("turn_check_run")
			if err != nil {
				t.Fatalf("query turn_check_run: %v", err)
			}
			var runs int
			for _, call := range tc.calls {
				if call.Name == "probe_run_check" {
					runs++
				}
			}
			if len(facts) != runs {
				t.Fatalf("turn_check_run count = %d, want %d (a later write does not erase the receipt)", len(facts), runs)
			}
		})
	}
}

func TestTurnOutcome_ADeclaredCheckIsEvidence(t *testing.T) {
	namesCheck := func(result *ExecutionResult) bool {
		return slices.Contains(result.MissingEvidence, "/check_not_green")
	}
	mdWrite := func() *ExecutionResult {
		result := mutationResult()
		result.WrittenPaths = []string{"Docs/guide.md"}
		result.Intent.Verb = "/fix"
		return result
	}
	declare := func(t *testing.T, e *Executor) {
		t.Helper()
		if !e.assertTurnFact(types.Fact{Predicate: "turn_declared_check", Args: []any{testTurn}}) {
			t.Fatal("assert turn_declared_check")
		}
	}
	closeTurn := func(e *Executor, result *ExecutionResult, verb string) {
		e.assertTurnEvidence(testTurn, verb, result)
		e.captureTurnOutcome(testTurn, result, nil)
	}

	t.Run("aWriteWithNoRunNamesTheCheck", func(t *testing.T) {
		e := newObligationExec(t)
		result := mdWrite()
		declare(t, e)
		closeTurn(e, result, "/fix")
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Fatal("TurnOutcome = /done for a write whose acceptance check never ran")
		}
		if !namesCheck(result) {
			t.Fatalf("MissingEvidence = %v, want /check_not_green", result.MissingEvidence)
		}
		if sentence := DescribeMissingEvidence(result.MissingEvidence); !strings.Contains(sentence, missingEvidenceSentence("/check_not_green")) {
			t.Fatalf("the model is told %q, want the check named", sentence)
		}
	})

	t.Run("aRedRunAfterTheWriteNamesTheCheck", func(t *testing.T) {
		e := newObligationExec(t)
		result := mdWrite()
		result.CheckSinceLastWrite = &tools.AcceptanceRun{Argv: []string{"go", "test"}, ExitCode: 1}
		declare(t, e)
		closeTurn(e, result, "/fix")
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Fatal("TurnOutcome = /done after a red acceptance check")
		}
		if !namesCheck(result) {
			t.Fatalf("MissingEvidence = %v, want /check_not_green", result.MissingEvidence)
		}
	})

	t.Run("aGreenRunAfterTheWriteVerifies", func(t *testing.T) {
		e := newObligationExec(t)
		result := mdWrite()
		result.CheckSinceLastWrite = &tools.AcceptanceRun{Argv: []string{"go", "test"}, ExitCode: 0}
		declare(t, e)
		closeTurn(e, result, "/fix")
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Fatalf("TurnOutcome = %v (missing %v), want /done after a green check", result.TurnOutcome, result.MissingEvidence)
		}
	})

	t.Run("aRunBeforeTheLastWriteDoesNotCount", func(t *testing.T) {
		e := newObligationExec(t)
		result := mdWrite()
		declare(t, e)
		// The tool loop clears CheckSinceLastWrite on the write that follows
		// a run. The closure sees no run since the last write.
		closeTurn(e, result, "/fix")
		if result.TurnOutcome == types.MangleAtom("/done") || !namesCheck(result) {
			t.Fatalf("TurnOutcome = %v missing %v, want unverified and /check_not_green", result.TurnOutcome, result.MissingEvidence)
		}
	})

	t.Run("aWriteWithNoDeclaredCheckIsUnchanged", func(t *testing.T) {
		e := newObligationExec(t)
		result := mdWrite()
		closeTurn(e, result, "/fix")
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Fatalf("TurnOutcome = %v (missing %v), want /done: a document with no check owes nothing", result.TurnOutcome, result.MissingEvidence)
		}
		if namesCheck(result) {
			t.Fatalf("a turn with no declared check named /check_not_green: %v", result.MissingEvidence)
		}
	})

	t.Run("aReviewerWhoWroteDoesNotOweTheCheck", func(t *testing.T) {
		e := newObligationExec(t)
		result := mdWrite()
		result.Intent.Verb = "/review"
		declare(t, e)
		closeTurn(e, result, "/review")
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Fatalf("TurnOutcome = %v (missing %v), want /done: a reviewer is not offered run_check", result.TurnOutcome, result.MissingEvidence)
		}
	})

	t.Run("theModelCannotAssertTheCheckOrItsReceipt", func(t *testing.T) {
		permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
		for _, update := range []string{
			"turn_declared_check(/turn_x).",
			"turn_check_run(/turn_x, 1, 0).",
			"editing_persona(/reviewer).",
			"turn_catalog(/turn_x, /run_check).",
			"turn_gate(/turn_x, /check, /passing).",
		} {
			if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
				t.Errorf("the model can assert %s; the check and its catalog are the harness's", update)
			}
		}
	})
}

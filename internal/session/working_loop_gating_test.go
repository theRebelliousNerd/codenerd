package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// roundScriptProvider answers every model call from one script: `rounds`
// responses that each request a single tool call, then a final text. It keeps
// every history it was handed, so a test can see exactly what the model saw.
//
// Each round asks for a different path on purpose: an identical call every
// round is a period-1 cycle, which the working policy stops for its own reason
// and would mask the one under test.
type roundScriptProvider struct {
	*MockLLMClient
	toolName  string
	rounds    int
	histories [][]types.Message
	catalogs  [][]string // tool names offered on each call, in order
}

func (p *roundScriptProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.histories = append(p.histories, append([]types.Message(nil), history...))
	names := make([]string, 0, len(definitions))
	for _, def := range definitions {
		names = append(names, def.Name)
	}
	p.catalogs = append(p.catalogs, names)
	if p.rounds > 0 {
		p.rounds--
		n := len(p.histories)
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
			ID: fmt.Sprintf("call-%d", n), Name: p.toolName,
			Input: map[string]any{"path": fmt.Sprintf("probe-%d.txt", n)},
		}}}, nil
	}
	return &types.LLMToolResponse{Text: "done"}, nil
}

// newWorkingLoopExecutor is the production shape of the loop after boot: a
// workspace root, so every turn runs inside a working loop, and no count
// limits anywhere — there are none left to set.
func newWorkingLoopExecutor(t *testing.T, client types.LLMClient) *Executor {
	t.Helper()
	e := &Executor{kernel: &MockKernel{}, virtualStore: &MockVirtualStore{}, llmClient: client, config: DefaultExecutorConfig()}
	e.config.EnableSafetyGate = false
	e.config.VerifyTestsAfterEdits = false
	e.config.CriticReviewAfterEdits = false
	e.config.WorkspaceRoot = t.TempDir()
	e.workingWorld = &MockKernel{}
	return e
}

func toolResultContents(histories [][]types.Message) []string {
	var out []string
	for _, history := range histories {
		for _, m := range history {
			for _, r := range m.ToolResults {
				out = append(out, r.Content)
			}
		}
	}
	return out
}

func anyContains(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}

// modelFacingText is everything a provider was handed across a run: the text
// of every message and the content of every tool result, including the
// orchestrator's appended steering.
func modelFacingText(histories [][]types.Message) []string {
	var out []string
	for _, history := range histories {
		for _, m := range history {
			if strings.TrimSpace(m.Text) != "" {
				out = append(out, m.Text)
			}
			for _, r := range m.ToolResults {
				out = append(out, r.Content)
			}
		}
	}
	return out
}

// budgetVocabulary is the wording the model used to be handed when the loop
// was bounded by counts: the per-round nudge ("Orchestrator budget: N tool
// calls and M rounds remain"), the refused call ("tool call budget exceeded
// for this turn", "<name>: budget exceeded") and the forced-final prose.
// None of it describes a fact about the task, and a model told how many calls
// remain optimizes for the count.
var budgetVocabulary = []string{
	"budget",
	"Budget",
	"calls remain",
	"rounds remain",
	"tool calls left",
	"exploration budget",
	"iterations left",
}

func assertNoBudgetVocabulary(t *testing.T, histories [][]types.Message) {
	t.Helper()
	seen := modelFacingText(histories)
	for _, banned := range budgetVocabulary {
		for _, text := range seen {
			if strings.Contains(text, banned) {
				t.Fatalf("the model was handed budget vocabulary %q: %q", banned, text)
			}
		}
	}
}

// No count of calls or rounds ends a turn. This run makes 60 tool calls over
// 60 rounds, each round novel and successful — comfortably past the 50 calls
// and 8 rounds that were DefaultExecutorConfig's ceilings until 2026-09-18,
// and past the 16-round extension arithmetic that used to sit on top of them.
// The loop must simply keep going until the model stops asking for tools, and
// the model must never be told anything about a budget.
func TestToolLoop_NeverStopsOnCallCount(t *testing.T) {
	const toolName = "working_loop_uncounted_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	const rounds = 60
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: rounds}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}

	resp, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v — %d rounds of progress is not a reason to stop", err, rounds)
	}
	if resp == nil || resp.Text != "done" {
		t.Fatalf("response = %+v, want the scripted final answer after %d rounds", resp, rounds)
	}
	if result.ToolCallsExecuted != rounds {
		t.Fatalf("executed = %d, want %d: the loop was cut short by a count", result.ToolCallsExecuted, rounds)
	}
	if client.rounds != 0 {
		t.Fatalf("%d scripted rounds were never reached: something bounded the loop before the model was done", client.rounds)
	}
	assertNoBudgetVocabulary(t, client.histories)
}

// With no count ceiling the loop ends only when the working policy says so.
// Three consecutive rounds in which every tool failed derive
// working_stop(/tool_failures); the turn then ends as unresolved, not as a
// success and not by running the model out of script, and the message names
// the rule and the facts that satisfied it.
func TestToolLoop_StopsOnDerivedToolFailures(t *testing.T) {
	const toolName = "working_loop_failing_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "", errors.New("probe failed") },
	})
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: 50}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err == nil || !strings.Contains(err.Error(), "working_stop(/tool_failures)") {
		t.Fatalf("err = %v, want the working policy to stop the turn for repeated tool failures, by name", err)
	}
	if !strings.Contains(err.Error(), "working_control(_, 3)") {
		t.Fatalf("err = %v, want the facts that satisfied the rule", err)
	}
	if result.ToolCallsExecuted != 3 {
		t.Fatalf("executed = %d, want 3: the third failed round is the stop, not the script running out", result.ToolCallsExecuted)
	}
	if client.rounds == 0 {
		t.Fatal("the script ran out: the loop was bounded by the mock, not by policy")
	}
}

// sameCallProvider asks for the identical tool call every round: same name,
// same arguments, and (because the tool is deterministic) the same result
// bytes. That is a period-1 trace cycle.
type sameCallProvider struct {
	*MockLLMClient
	toolName  string
	histories [][]types.Message
	requests  int
}

func (p *sameCallProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.histories = append(p.histories, append([]types.Message(nil), history...))
	p.requests++
	if len(defs) == 0 {
		return &types.LLMToolResponse{Text: "done"}, nil
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
		// The ID differs per round because providers require unique IDs; the
		// trace signature is name + arguments + status + returned bytes, so
		// the cycle is visible regardless.
		ID: fmt.Sprintf("same-%d", p.requests), Name: p.toolName,
		Input: map[string]any{"path": "probe.txt"},
	}}}, nil
}

// working_stop(/repeated_cycle) fires from working_control(/yes, _), which the
// loop asserts when the tail of the trace repeats for working_repeat_threshold
// cycles, on a change task that has written nothing. With the policy's
// threshold of 2, the second identical round is the stop — measured in Go
// because only the loop can see the trace, decided in Mangle because only the
// policy says what a repeat means.
func TestToolLoop_StopsOnDerivedRepeatedCycle(t *testing.T) {
	const toolName = "working_loop_cycle_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	client := &sameCallProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err == nil || !strings.Contains(err.Error(), "working_stop(/repeated_cycle)") {
		t.Fatalf("err = %v, want the working policy to stop an identically repeating trace, by name", err)
	}
	if !strings.Contains(err.Error(), "working_repeat_threshold") {
		t.Fatalf("err = %v, want the span that made it a cycle named as policy's", err)
	}
	if result.ToolCallsExecuted != 2 {
		t.Fatalf("executed = %d, want 2: working_repeat_threshold(2) makes the second identical round the cycle", result.ToolCallsExecuted)
	}
	assertNoBudgetVocabulary(t, client.histories)
}

// The same repeat on a read task is the end of its reading, not a failure:
// the policy derives working_finalize(/repeat_after_reading) and the harness
// asks for the conclusion. Observed 2026-09-22 on campaign 7b853890: a
// /research task was stopped here after 21 reads, failed, and was retried from
// nothing.
func TestToolLoop_ARepeatingReadTaskFinalizesWithItsConclusion(t *testing.T) {
	const toolName = "working_loop_read_cycle_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	client := &sameCallProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}

	resp, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("err = %v, want the read task finalized with its conclusion, not stopped", err)
	}
	if resp == nil || resp.Text != "done" {
		t.Fatalf("response = %+v, want the conclusion the forced-final call collected", resp)
	}
	// Two rounds make the repeat; the batch in hand when the policy finalized
	// is executed too, because every provider requires each tool_use to be
	// paired with its result before the conclusion is asked for.
	if result.ToolCallsExecuted != 3 {
		t.Fatalf("executed = %d, want 3: two rounds to see the repeat, then the pending batch", result.ToolCallsExecuted)
	}
}

// A repeat of a call that fails every time read nothing, so there is no
// conclusion to ask for: the read-repeat finalize does not fire, and the
// failures run on to working_stop(/tool_failures), which names them. Until
// 2026-09-23 the finalize fired at the second failed round and the turn ended
// with the model's text and no error (e2e InfiniteToolLoop).
func TestToolLoop_ARepeatingFailedReadStopsOnToolFailures(t *testing.T) {
	const toolName = "working_loop_failing_cycle_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "", errors.New("probe failed") },
	})
	client := &sameCallProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err == nil || !strings.Contains(err.Error(), "working_stop(/tool_failures)") {
		t.Fatalf("err = %v, want working_stop(/tool_failures): a repeat of failed calls gathered nothing to conclude from", err)
	}
	if result.ToolCallsExecuted != 3 {
		t.Fatalf("executed = %d, want 3: the third failed round is the stop", result.ToolCallsExecuted)
	}
}

// scriptedCallsProvider answers each model call with the next scripted tool
// call. Once the script runs out, or no tools are offered (the forced-final
// call), it answers with text. It keeps every history it was handed.
type scriptedCallsProvider struct {
	*MockLLMClient
	calls     []types.ToolCall
	next      int
	histories [][]types.Message
}

func (p *scriptedCallsProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.histories = append(p.histories, append([]types.Message(nil), history...))
	if p.next < len(p.calls) && len(defs) > 0 {
		call := p.calls[p.next]
		p.next++
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{call}}, nil
	}
	return &types.LLMToolResponse{Text: "done"}, nil
}

func readCalls(toolName string, n int) []types.ToolCall {
	calls := make([]types.ToolCall, 0, n)
	for i := 1; i <= n; i++ {
		calls = append(calls, types.ToolCall{
			ID: fmt.Sprintf("read-%d", i), Name: toolName,
			Input: map[string]any{"path": fmt.Sprintf("probe-%d.txt", i)},
		})
	}
	return calls
}

// A change task that only reads is stopped by policy at the stall span, as
// unresolved — and one round short of it is not stopped at all. That pair is
// the whole claim: the boundary is working_stall_rounds in working_set.mg,
// derived over the rounds the loop asserted, not a ceiling in Go.
// Observed 2026-09-11: 300 reads in 25 minutes before a one-line edit the
// brief had named by file and line.
func TestToolLoop_StopsOnDerivedReadOnlyStall(t *testing.T) {
	const toolName = "working_loop_stall_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	// A change task's catalog carries a write tool; under the commit regime it
	// is what remains on offer, and this scripted model never uses it.
	const writerName = "working_loop_stall_writer"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectWrite, Name: writerName, Category: tools.CategoryCode,
		Execute: func(context.Context, map[string]any) (string, error) { return "written", nil },
	})
	// working_stall_rounds(24) in internal/context/working_set.mg.
	const stallRounds = 24

	run := func(t *testing.T, scripted int) (*scriptedCallsProvider, *ExecutionResult, error) {
		t.Helper()
		client := &scriptedCallsProvider{MockLLMClient: &MockLLMClient{}, calls: readCalls(toolName, scripted)}
		e := newWorkingLoopExecutor(t, client)
		result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
		_, _, err := e.runToolLoop(context.Background(), "system", "fix it",
			&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName, writerName}},
			&prompt.CompilationContext{ShardID: "probe"}, result)
		return client, result, err
	}

	t.Run("the stall span derives the stop", func(t *testing.T) {
		client, result, err := run(t, stallRounds+36)
		if err == nil || !strings.Contains(err.Error(), "working_stop(/read_only_stall)") {
			t.Fatalf("err = %v, want the working policy to stop a change task that never wrote, by name", err)
		}
		if !strings.Contains(err.Error(), "working_progress(/write, 24, 0, _, _)") ||
			!strings.Contains(err.Error(), "working_stall_rounds") {
			t.Fatalf("err = %v, want the rule and the facts that satisfied it", err)
		}
		if result.ToolCallsExecuted != stallRounds {
			t.Fatalf("executed = %d, want %d: the stall span is the stop, not the script running out", result.ToolCallsExecuted, stallRounds)
		}
		if client.next >= len(client.calls) {
			t.Fatal("the script ran out: the loop was bounded by the mock, not by policy")
		}
		seen := toolResultContents(client.histories)
		if !anyContains(seen, "[orchestrator] 8 rounds of reading and no file written") {
			t.Fatalf("the model was never told to implement before the stop; tool results seen: %q", seen)
		}
		assertNoBudgetVocabulary(t, client.histories)
	})

	t.Run("one round short of it is not a stop", func(t *testing.T) {
		client, result, err := run(t, stallRounds-1)
		if err != nil {
			t.Fatalf("err = %v, want no stop at %d rounds: the span is %d", err, stallRounds-1, stallRounds)
		}
		if result.ToolCallsExecuted != stallRounds-1 {
			t.Fatalf("executed = %d, want %d", result.ToolCallsExecuted, stallRounds-1)
		}
		if client.next != len(client.calls) {
			t.Fatalf("the script did not run out (%d of %d used): the turn ended for some other reason", client.next, len(client.calls))
		}
		assertNoBudgetVocabulary(t, client.histories)
	})
}

// A read task is steered to conclude after the nudge span; policy never stops
// it for reading alone.
func TestRunToolLoop_ProgressDriven_NudgesAReadTaskToConclude(t *testing.T) {
	const toolName = "working_loop_conclude_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	client := &scriptedCallsProvider{MockLLMClient: &MockLLMClient{}, calls: readCalls(toolName, 9)}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}

	resp, _, err := e.runToolLoop(context.Background(), "system", "explain it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if resp == nil || resp.Text != "done" {
		t.Fatalf("response = %+v, want the scripted final answer", resp)
	}
	if result.ToolCallsExecuted != 9 {
		t.Fatalf("executed = %d, want 9", result.ToolCallsExecuted)
	}
	seen := toolResultContents(client.histories)
	if !anyContains(seen, "[orchestrator] 8 rounds of reading. Conclude") {
		t.Fatalf("no conclude nudge after the nudge span; tool results seen: %q", seen)
	}
	if anyContains(seen, "Orchestrator budget:") {
		t.Fatalf("an open loop reported a count budget it does not have; tool results seen: %q", seen)
	}
}

// A change task that wrote and then only read has its reading closed after a
// nudge span (the commit regime, as for a task that never wrote) and is
// finalized by policy a commit span later: the model's pending batch still
// runs, then the harness asks for the conclusion and takes over verification.
// Before this the loop waited for the model to get round to the test the task
// named, which it never did; and before the regime applied here, a turn that
// made one of three briefed edits and then re-read the same files was
// finalized with the other two unmade (observed 2026-09-11).
func TestRunToolLoop_ProgressDriven_FinalizesAChangeTaskThatWroteThenDrifted(t *testing.T) {
	const readTool = "working_loop_drift_probe"
	const writeTool = "create_file"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: readTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectWrite, Name: writeTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "written", nil },
	})
	calls := append([]types.ToolCall{{ID: "write-1", Name: writeTool, Input: map[string]any{"path": "probe.txt", "content": "x"}}}, readCalls(readTool, 40)...)
	client := &scriptedCallsProvider{MockLLMClient: &MockLLMClient{}, calls: calls}
	e := newWorkingLoopExecutor(t, client)
	// An effectful tool needs the executive gate; without one the executor
	// refuses the write and this would be the read-only stall test again.
	e.virtualStore = &testExecutiveStore{}
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	resp, toolErrs, err := e.runToolLoop(context.Background(), "system", "fix it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q)", err, toolErrs)
	}
	if resp == nil || resp.Text != "done" {
		t.Fatalf("response = %+v, want the forced final answer", resp)
	}
	// One write, eight idle reads to the regime boundary, eight more rounds
	// whose reads are answered with the regime instead of running, and the
	// pending seventeenth read executed inside the forced-final path.
	if result.ToolCallsExecuted != 18 {
		t.Fatalf("executed = %d, want 18: reading closes eight idle rounds after the write and the turn finalizes eight rounds later", result.ToolCallsExecuted)
	}
	if client.next >= len(client.calls) {
		t.Fatal("the script ran out: the loop was bounded by the mock, not by policy")
	}
	seen := toolResultContents(client.histories)
	if !anyContains(seen, "nothing has verified it") {
		t.Fatalf("the model was never told to verify after its write; tool results seen: %q", seen)
	}
	if !anyContains(seen, "Reading is closed for this task") {
		t.Fatalf("reading was never closed after the write; tool results seen: %q", seen)
	}
}

// A change task that has ignored the implement nudge for the commit span is
// put in the commit regime: the read tools leave the catalog offered to the
// model, a read it asks for anyway is answered with the regime instead of
// run, and the stall span still ends the turn. Observed 2026-09-11: four runs
// of one insertion brief re-read the same four facts for 24 rounds each,
// nudge in hand, and never wrote.
func TestRunToolLoop_ProgressDriven_ClosesReadingUnderTheCommitRegime(t *testing.T) {
	const toolName = "working_loop_commit_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	// An external-effect tool is exploration too and leaves the catalog with
	// the read tools; recall_context stays.
	const externalName = "working_loop_commit_external"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectExternal, Name: externalName, Category: tools.CategoryResearch,
		Execute: func(context.Context, map[string]any) (string, error) { return "fetched", nil },
	})
	// Whether the core tools are registered depends on which tests ran
	// before this one; a stub under the real name stands in for
	// recall_context (which the regime keeps by name) only when it is absent.
	if tools.Global().Get("recall_context") == nil {
		registerTestTool(t, &tools.Tool{
			Effect: tools.EffectRead, Name: "recall_context", Category: tools.CategoryGeneral,
			Execute: func(context.Context, map[string]any) (string, error) { return "recalled", nil },
		})
	}
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: 30}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	_, _, err := e.runToolLoop(context.Background(), "system", "change it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName, externalName, "recall_context"}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err == nil || !strings.Contains(err.Error(), "read_only_stall") {
		t.Fatalf("err = %v, want the stall span to end a task that never wrote", err)
	}
	// Calls 1..16 are the open regime; the policy closes reading at 16 rounds,
	// so the 17th request onward offers no read tool.
	for i, names := range client.catalogs {
		offered := slices.Contains(names, toolName)
		if i < 16 && !offered {
			t.Fatalf("request %d: the read tool was withheld before the commit span", i+1)
		}
		if i >= 16 && offered {
			t.Fatalf("request %d: the read tool is still offered under the commit regime (%v)", i+1, names)
		}
		if i >= 16 && slices.Contains(names, externalName) {
			t.Fatalf("request %d: an external tool is still offered under the commit regime (%v)", i+1, names)
		}
		if i >= 16 && !slices.Contains(names, "recall_context") {
			t.Fatalf("request %d: recall_context must stay under the commit regime (%v)", i+1, names)
		}
	}
	if len(client.catalogs) < 18 {
		t.Fatalf("only %d requests; the run must continue under the commit regime", len(client.catalogs))
	}
	results := toolResultContents(client.histories)
	if !anyContains(results, "Reading is closed for this task") {
		t.Fatalf("a read asked for under the commit regime must be answered with the regime; results: %q", results[len(results)-3:])
	}
	if anyContains(results[len(results)-2:], "observed") {
		t.Fatalf("a read tool ran under the commit regime; last results: %q", results[len(results)-2:])
	}
}

package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// stepScriptProvider answers the planning call with a fixed plan and plays
// each step's pass from a script keyed by the step's file: the anchor names
// the current step, the provider looks the file up and answers with that
// step's calls in order, then a final sentence. It records every anchor it
// was given and every catalog it was offered.
type stepScriptProvider struct {
	*MockLLMClient
	plan     string
	script   map[string][]types.ToolCall // per step file, the calls to make
	played   map[string]int
	anchors  []string
	catalogs [][]string
}

func newStepScriptProvider(plan string, script map[string][]types.ToolCall) *stepScriptProvider {
	p := &stepScriptProvider{MockLLMClient: &MockLLMClient{}, plan: plan, script: script, played: map[string]int{}}
	p.MockLLMClient.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) { return p.plan, nil }
	return p
}

// currentStepFile reads the file the executive named for this pass out of
// the anchor, the only place the provider can learn it from.
func currentStepFile(history []types.Message) string {
	for _, m := range history {
		if m.Role != "user" || m.Text == "" {
			continue
		}
		for _, line := range strings.Split(m.Text, "\n") {
			if strings.HasPrefix(line, "Step ") && strings.Contains(line, " of ") && strings.Contains(line, " :: ") {
				after := line[strings.Index(line, ": ")+2:]
				file, _, _ := strings.Cut(after, " :: ")
				return strings.TrimSpace(file)
			}
		}
	}
	return ""
}

func (p *stepScriptProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	p.catalogs = append(p.catalogs, names)
	for _, m := range history {
		if m.Role == "user" && m.Text != "" {
			p.anchors = append(p.anchors, m.Text)
		}
	}
	file := currentStepFile(history)
	calls := p.script[file]
	if i := p.played[file]; i < len(calls) {
		call := calls[i]
		// A call the catalog no longer offers is answered with prose: that is
		// what a provider does when a tool it wanted has left the catalog.
		offered := false
		for _, n := range names {
			if n == call.Name {
				offered = true
				break
			}
		}
		p.played[file] = i + 1
		if offered {
			return &types.LLMToolResponse{ToolCalls: []types.ToolCall{call}}, nil
		}
	}
	return &types.LLMToolResponse{Text: "finished " + file}, nil
}

func newPlannedStepsExecutor(t *testing.T, client types.LLMClient) *Executor {
	t.Helper()
	e := newWorkingLoopExecutor(t, client)
	// An effectful tool needs the executive gate; without one the executor
	// refuses every write.
	e.virtualStore = &testExecutiveStore{}
	return e
}

func registerStepTools(t *testing.T) (readTool, writeTool string) {
	t.Helper()
	readTool, writeTool = "planned_steps_read_probe", "create_file"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: readTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectWrite, Name: writeTool, Category: tools.CategoryGeneral,
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			return "written " + args["path"].(string), nil
		},
	})
	return readTool, writeTool
}

func TestParseWorkSteps(t *testing.T) {
	plan := "Here is the plan:\n" +
		"STEP internal/a.go :: add the field after line 10\n" +
		"- STEP `internal/b.go` :: call it at line 20\n" +
		"2. STEP internal/a.go :: add the field after line 10\n" +
		"STEP :: no file\n" +
		"STEP internal/c.go\n" +
		"STEP internal/prompt/ :: Verify with: go test ./internal/prompt/ -run TestEmbeddedCorpus\n" +
		"STEP ./internal/prompt/... :: run go test on the package\n" +
		"STEP internal/d.go :: Run the tests named in the task\n" +
		"not a step line\n"
	got := parseWorkSteps(plan)
	if len(got) != 2 {
		t.Fatalf("steps = %+v, want the two well-formed distinct edit steps (a verification is not a step)", got)
	}
	if got[0].File != "internal/a.go" || got[0].Change != "add the field after line 10" {
		t.Errorf("step 1 = %+v", got[0])
	}
	if got[1].File != "internal/b.go" || got[1].Change != "call it at line 20" {
		t.Errorf("step 2 = %+v (the bullet and the backticks must be tolerated)", got[1])
	}
	var many strings.Builder
	for i := 0; i < maxPlannedSteps+5; i++ {
		many.WriteString("STEP f" + strings.Repeat("x", i) + ".go :: change\n")
	}
	if n := len(parseWorkSteps(many.String())); n != maxPlannedSteps {
		t.Errorf("unbounded plan parsed to %d steps, want the cap %d", n, maxPlannedSteps)
	}
}

// Two edit sites: the executive runs two passes, each anchored on its own
// step with the earlier step's outcome stated, both files are written, and
// the turn's report lists both as edited.
func TestRunToolLoop_PlannedSteps_RunsEachStepAsItsOwnPass(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it with the greeting\nSTEP b.txt :: create it with the farewell\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	resp, toolErrs, err := e.runToolLoop(context.Background(), "system", "create both files",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q)", err, toolErrs)
	}
	if resp == nil || resp.Text != "finished b.txt" {
		t.Fatalf("response = %+v, want the last step's closing sentence", resp)
	}
	if result.SuccessfulWriteTools != 2 {
		t.Fatalf("writes = %d, want one per step", result.SuccessfulWriteTools)
	}
	if !strings.Contains(result.StepReport, "Planned steps: 2, edited: 2.") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if !anyContains(client.anchors, "this turn is step 1 of 2") || !anyContains(client.anchors, "Step 1 of 2: a.txt :: create it with the greeting") {
		t.Fatalf("the first pass was not anchored on step 1; anchors: %q", client.anchors)
	}
	if !anyContains(client.anchors, "Done: [1] a.txt :: create it with the greeting (edited)") || !anyContains(client.anchors, "Step 2 of 2: b.txt") {
		t.Fatalf("the second pass was not told step 1 had edited; anchors: %q", client.anchors)
	}
	if !anyContains(client.anchors, "create both files") {
		t.Fatalf("the task itself must be in every anchor; anchors: %q", client.anchors)
	}
}

// A step whose first pass only read gets one more pass with reading closed:
// the read tool leaves the catalog, and the edit lands.
func TestRunToolLoop_PlannedSteps_RetriesAStepThatMadeNoEditWithReadingClosed(t *testing.T) {
	readTool, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		// First pass: a read, then prose. Second pass: the read is refused by
		// the catalog, so the script's next call, the write, is what runs.
		"a.txt": {
			{ID: "r-a", Name: readTool, Input: map[string]any{"path": "a.txt"}},
			{ID: "r-a2", Name: readTool, Input: map[string]any{"path": "a.txt"}},
			{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}},
		},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, toolErrs, err := e.runToolLoop(context.Background(), "system", "create both files",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q)", err, toolErrs)
	}
	if result.SuccessfulWriteTools != 2 {
		t.Fatalf("writes = %d, want both steps edited after the retry; report:\n%s", result.SuccessfulWriteTools, result.StepReport)
	}
	if !anyContains(client.anchors, "This step's first pass made no edit. Reading is closed") {
		t.Fatalf("no retry anchor seen; anchors: %q", client.anchors)
	}
	closed := false
	for _, catalog := range client.catalogs {
		if !slices.Contains(catalog, readTool) && slices.Contains(catalog, writeTool) {
			closed = true
		}
	}
	if !closed {
		t.Fatalf("no pass was offered a catalog without the read tool; catalogs: %v", client.catalogs)
	}
}

// A step that makes no edit in either pass is reported and fails the turn,
// after the other steps ran.
func TestRunToolLoop_PlannedSteps_ReportsAStepThatNeverEdited(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "create both files",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if !errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("err = %v, want ErrStepsIncomplete", err)
	}
	if !strings.Contains(err.Error(), "[1] a.txt") {
		t.Fatalf("the error must name the step that made no edit: %v", err)
	}
	if result.SuccessfulWriteTools != 1 {
		t.Fatalf("writes = %d, want the second step's edit despite the first step's failure", result.SuccessfulWriteTools)
	}
	if !strings.Contains(result.StepReport, "[1] a.txt :: create it — no edit") || !strings.Contains(result.StepReport, "[2] b.txt :: create it — edited") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if wrapped := wrapToolLoopError(err); !errors.Is(wrapped, ErrStepsIncomplete) || strings.HasPrefix(wrapped.Error(), "LLM generation failed") {
		t.Fatalf("the turn must surface the incomplete plan as itself, got %v", wrapped)
	}
}

// A planner that splits an import out of the change that needs it produces
// a step the earlier step has already done. Such a step is reported, not
// failed: the guarantee is that every file the plan names was edited.
func TestRunToolLoop_PlannedSteps_AStepOnAFileAlreadyEditedIsNotAFailure(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it with the greeting\nSTEP a.txt :: add the import it needs\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, toolErrs, err := e.runToolLoop(context.Background(), "system", "create both files",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q); report:\n%s", err, toolErrs, result.StepReport)
	}
	if result.SuccessfulWriteTools != 2 {
		t.Fatalf("writes = %d, want one per file", result.SuccessfulWriteTools)
	}
	if !strings.Contains(result.StepReport, "[2] a.txt :: add the import it needs — no edit (file edited in step 1)") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if !anyContains(client.anchors, "This step's first pass made no edit. Reading is closed") {
		t.Fatalf("the covered step must still have been given its second pass; anchors: %q", client.anchors)
	}
}

// One STEP line, or none, is the single pass the loop always ran: the task
// text is the anchor, untouched, and no report is written.
func TestRunToolLoop_PlannedSteps_OneStepIsOnePass(t *testing.T) {
	_, writeTool := registerStepTools(t)
	for _, plan := range []string{"STEP a.txt :: create it\n", "", "no plan here"} {
		client := newStepScriptProvider(plan, nil)
		e := newPlannedStepsExecutor(t, client)
		result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
		_, _, err := e.runToolLoop(context.Background(), "system", "create the file",
			&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
			&prompt.CompilationContext{ShardID: "probe"}, result)
		if err != nil {
			t.Fatalf("plan %q: runToolLoop: %v", plan, err)
		}
		if result.StepReport != "" {
			t.Fatalf("plan %q: a single pass wrote a step report: %q", plan, result.StepReport)
		}
		if len(client.anchors) == 0 || client.anchors[0] != "create the file" {
			t.Fatalf("plan %q: the anchor must be the task itself, got %q", plan, client.anchors)
		}
	}
}

// A turn that is not write-oriented, or has no write tool, is never planned:
// the planning call is not even made.
func TestRunToolLoop_PlannedSteps_OnlyForChangeTasksWithAWriteTool(t *testing.T) {
	readTool, writeTool := registerStepTools(t)
	planned := 0
	newClient := func() *stepScriptProvider {
		c := newStepScriptProvider("STEP a.txt :: x\nSTEP b.txt :: y\n", nil)
		c.MockLLMClient.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) {
			planned++
			return c.plan, nil
		}
		return c
	}
	cases := []struct {
		name  string
		verb  string
		tools []string
	}{
		{"a read task", "/explain", []string{readTool, writeTool}},
		{"a change task with no write tool", "/fix", []string{readTool}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClient()
			e := newPlannedStepsExecutor(t, client)
			result := &ExecutionResult{Intent: perception.Intent{Verb: tc.verb}}
			if _, _, err := e.runToolLoop(context.Background(), "system", "do it",
				&config.EffectiveAgentRuntimeConfig{AllowedTools: tc.tools},
				&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
				t.Fatalf("runToolLoop: %v", err)
			}
			if planned != 0 {
				t.Fatalf("the planning call was made %d time(s)", planned)
			}
		})
	}
}

// The report reaches the user through the response, after the model's own
// closing sentence, so a task run in steps is described by the ledger and
// not by whatever the last pass claimed.
func TestWorkStepReport_ListsEveryStep(t *testing.T) {
	steps := []workStep{
		{File: "a.go", Change: "add the field", Edited: true, Calls: 3, Note: "Added the field.\nmore"},
		{File: "b.go", Change: "call it", Edited: false, Calls: 9, Note: "task unresolved: read_only_stall"},
		{File: "a.go", Change: "add the import", Edited: false, Calls: 2},
	}
	markCoveredSteps(steps)
	got := workStepReport(steps)
	for _, want := range []string{
		"Planned steps: 3, edited: 1.",
		"[1] a.go :: add the field — edited (3 tool call(s)): Added the field.",
		"[2] b.go :: call it — no edit (9 tool call(s)): task unresolved: read_only_stall",
		"[3] a.go :: add the import — no edit (file edited in step 1) (2 tool call(s))",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "more") {
		t.Errorf("only the first line of a note belongs in the report:\n%s", got)
	}
	if missing := unfinishedSteps(steps); len(missing) != 1 || missing[0] != "[2] b.go" {
		t.Errorf("unfinished = %v, want only the file no step edited", missing)
	}
}
